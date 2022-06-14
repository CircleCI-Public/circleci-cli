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

// ListPolicies calls the view policy-service list policy API. If the active filter is nil, all policies are returned. If
// activeFilter is not nil it will only return active or inactive policies based on the value of *activeFilter.
func (c Client) ListPolicies(ownerID string, activeFilter *bool) (interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/owner/%s/policy", c.serverUrl, ownerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	query := make(url.Values)
	if activeFilter != nil {
		query.Set("active", fmt.Sprint(*activeFilter))
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
			return nil, fmt.Errorf("unexected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// Creation types taken from policy-service: internal/policy/api.go

// CreationRequest represents the json payload to create a Policy in the Policy-Service
type CreationRequest struct {
	Name    string `json:"name"`
	Context string `json:"context"`
	Content string `json:"content"`
}

// CreatePolicy call the Create Policy API in the Policy-Service. It creates a policy for the specified owner and returns the created
// policy response as an interface{}.
func (c Client) CreatePolicy(ownerID string, policy CreationRequest) (interface{}, error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to encode policy payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/owner/%s/policy", c.serverUrl, ownerID), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from policy-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var response httpError
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, response.Error)
	}

	var response interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

type UpdateRequest struct {
	Name    *string `json:"name,omitempty"`
	Context *string `json:"context,omitempty"`
	Content *string `json:"content,omitempty"`
	Active  *bool   `json:"active,omitempty"`
}

// UpdatePolicy calls the UPDATE policy API in the policy-service. It updates a policy in the policy-service matching the given owner-id and policy-id.
func (c Client) UpdatePolicy(ownerID string, policyID string, policy UpdateRequest) (interface{}, error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to encode policy payload: %w", err)
	}

	req, err := http.NewRequest(
		"PATCH",
		fmt.Sprintf("%s/api/v1/owner/%s/policy/%s", c.serverUrl, ownerID, policyID),
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from policy-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var response httpError
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("unexpected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, response.Error)
	}

	var response interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetPolicy calls the GET policy API in the policy-service.It fetches the policy from policy-service matching the given owner-id and policy-id.
// It returns an error if the call fails or the policy could not be found.
func (c Client) GetPolicy(ownerID string, policyID string) (interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/owner/%s/policy/%s", c.serverUrl, ownerID, policyID), nil)
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
			return nil, fmt.Errorf("unexected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

// DeletePolicy calls the DELETE Policy API in the policy-service.
// It attempts to delete the policy matching the given policy-id and belonging to the given ownerID.
// It returns an error if the call fails or the policy could not be deleted.
func (c Client) DeletePolicy(ownerID string, policyID string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/owner/%s/policy/%s", c.serverUrl, ownerID, policyID), nil)
	if err != nil {
		return fmt.Errorf("failed to construct request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		var payload httpError
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return fmt.Errorf("unexected status-code: %d", resp.StatusCode)
		}
		return fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	return nil
}

type DecisionQueryRequest struct {
	After     *time.Time
	Before    *time.Time
	Branch    string
	ProjectID string
	Offset    int
}

// GetDecisionLogs calls the GET decision query API of policy-service. The endpoint accepts multiple filter values as
// path query parameters (start-time, end-time, branch-name, project-id and offset).
func (c Client) GetDecisionLogs(ownerID string, request DecisionQueryRequest) ([]interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/owner/%s/decision", c.serverUrl, ownerID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	query := make(url.Values)
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
			return nil, fmt.Errorf("unexected status-code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status-code: %d - %s", resp.StatusCode, payload.Error)
	}

	var body []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return body, nil
}

type DecisionRequest struct {
	Input   string `json:"input"`
	Context string `json:"context"`
}

func (c Client) MakeDecision(ownerID string, req DecisionRequest) (interface{}, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/owner/%s/decision", c.serverUrl, ownerID)

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

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return body, nil
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

		r.Header.Add("circle-token", config.Token)
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
