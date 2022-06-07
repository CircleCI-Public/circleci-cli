package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
// policy resonse as an interface{}.
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

// NewClient returns a new policy client that will use the provided settings.Config to automatically inject appropriate
// Circle-Token authentication and other relevant CLI headers.
func NewClient(baseURL string, config *settings.Config) *Client {
	transport := config.HTTPClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	config.HTTPClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
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
