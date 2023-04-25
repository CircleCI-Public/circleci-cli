package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

// Policy-service endpoints documentation : https://github.com/circleci/policy-service/blob/main/cmd/server/openapi.yaml

// Client communicates with the CircleCI policy-service to ask questions
// about policies. It satisfies policy.ClientInterface.
type Client struct {
	serverUrl string
	client    *http.Client
}

// httpError represents error response json payload as sent by the policy-server: internal/error.go
type httpError struct {
	Error   string                 `json:"error"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// Creation types taken from policy-service: internal/policy/api.go

// CreatePolicyBundleRequest defines the fields for the Create-Policy-Bundle endpoint as defined in Policy Service
type CreatePolicyBundleRequest struct {
	Policies map[string]string `json:"policies"`
	DryRun   bool              `json:"-"`
}

// CreatePolicyBundle calls the Create Policy Bundle API in the Policy-Service.
// It creates a policy bundle for the specified owner+context and returns the http status code as response
func (c Client) CreatePolicyBundle(ownerID string, context string, request CreatePolicyBundleRequest) (interface{}, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode policy payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/owner/%s/context/%s/policy-bundle", c.serverUrl, ownerID, context), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	if request.DryRun {
		q := req.URL.Query()
		q.Set("dry", "true")
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from policy-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var response httpError
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, response.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// FetchPolicyBundle calls the GET policy-bundle API in the policy-service
// If policyName is empty, the full policy bundle would be fetched for given ownerID+context
// If a policyName is provided, only that matching policy would be fetched for given ownerID+context+policyName
func (c Client) FetchPolicyBundle(ownerID, context, policyName string) (interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/owner/%s/context/%s/policy-bundle/%s", c.serverUrl, ownerID, context, policyName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

type DecisionQueryRequest struct {
	Status    string
	After     *time.Time
	Before    *time.Time
	Branch    string
	ProjectID string
	Offset    int
}

// GetDecisionLogs calls the GET decision query API of policy-service. The endpoint accepts multiple filter values as
// path query parameters (start-time, end-time, branch-name, project-id and offset).
func (c Client) GetDecisionLogs(ownerID string, context string, request DecisionQueryRequest) ([]interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/owner/%s/context/%s/decision", c.serverUrl, ownerID, context), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	query := make(url.Values)
	if request.Status != "" {
		query.Set("status", fmt.Sprint(request.Status))
	}
	if request.After != nil {
		query.Set("after", request.After.Format(time.RFC3339))
	}
	if request.Before != nil {
		query.Set("before", request.Before.Format(time.RFC3339))
	}
	if request.Branch != "" {
		query.Set("branch", fmt.Sprint(request.Branch))
	}
	if request.ProjectID != "" {
		query.Set("project_id", fmt.Sprint(request.ProjectID))
	}
	if request.Offset > 0 {
		query.Set("offset", fmt.Sprint(request.Offset))
	}

	req.URL.RawQuery = query.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// GetDecisionLog calls the GET decision query API of policy-service for a DecisionID.
// It also accepts a policyBundle bool param; If set to true will return only the policy bundle corresponding to that decision log.
func (c Client) GetDecisionLog(ownerID string, context string, decisionID string, policyBundle bool) (interface{}, error) {
	path := fmt.Sprintf("%s/api/v1/owner/%s/context/%s/decision/%s", c.serverUrl, ownerID, context, decisionID)
	if policyBundle {
		path += "/policy-bundle"
	}
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// DecisionRequest represents a request to Policy-Service to evaluate a given input against an organization's policies.
// The context determines which policies to apply.
type DecisionRequest struct {
	Input    string                 `json:"input"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// GetSettings calls the GET decision-settings API of policy-service.
func (c Client) GetSettings(ownerID string, context string) (interface{}, error) {
	path := fmt.Sprintf("%s/api/v1/owner/%s/context/%s/decision/settings", c.serverUrl, ownerID, context)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// DecisionSettings represents a request to Policy-Service to configure decision settings.
type DecisionSettings struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// SetSettings calls the PATCH decision-settings API of policy-service.
func (c Client) SetSettings(ownerID string, context string, request DecisionSettings) (interface{}, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	path := fmt.Sprintf("%s/api/v1/owner/%s/context/%s/decision/settings", c.serverUrl, ownerID, context)
	req, err := http.NewRequest("PATCH", path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// MakeDecision sends a requests to Policy-Service public decision endpoint and returns the decision response
func (c Client) MakeDecision(ownerID string, context string, req DecisionRequest) (*cpa.Decision, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/owner/%s/context/%s/decision", c.serverUrl, ownerID, context)

	request, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	request.Header.Set("Content-Length", strconv.Itoa(len(payload)))

	resp, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body cpa.Decision
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &body, nil
}

// NewClient returns a new policy client that will use the provided settings.Config to automatically inject appropriate
// Circle-Token authentication and other relevant CLI headers.
func NewClient(baseURL string, config *settings.Config) *Client {
	transport := config.HTTPClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Throttling the client so that it cannot make more than 10 concurrent requests at time
	sem := make(chan struct{}, 10)

	config.HTTPClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		// Acquiring semaphore to respect throttling
		sem <- struct{}{}

		// releasing the semaphore after a second ensuring client doesn't make more than cap(sem)/second
		time.AfterFunc(time.Second, func() { <-sem })

		if config.Token != "" {
			r.Header.Add("circle-token", config.Token)
		}
		r.Header.Add("Accept", "application/json")
		r.Header.Add("Content-Type", "application/json")
		r.Header.Add("User-Agent", version.UserAgent())
		if commandStr := header.GetCommandStr(); commandStr != "" {
			r.Header.Add("Circleci-Cli-Command", commandStr)
		}
		return transport.RoundTrip(r)
	})

	return &Client{
		serverUrl: strings.TrimSuffix(baseURL, "/"),
		client:    config.HTTPClient,
	}
}

// transportFunc is utility type for declaring a http.RoundTripper as a function literal
type transportFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements the http.RoundTripper interface
func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
