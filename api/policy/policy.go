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

// Client communicates with the CircleCI policy-service to ask questions
// about policies. It satisfies policy.ClientInterface.
type Client struct {
	serverUrl string
	client    *http.Client
}

type httpError struct {
	Error   string                 `json:"error"`
	Context map[string]interface{} `json:"context,omitempty"`
}

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
type CreationRequest struct {
	Name    string `json:"name"`
	Context string `json:"context"`
	Content string `json:"content"`
}

type CreationResponse struct {
	DocumentVersion int        `json:"document_version"`
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Context         string     `json:"context"`
	Content         string     `json:"content"`
	CreatedAt       *time.Time `json:"created_at"`
}

func (c Client) CreatePolicy(ownerID string, policy CreationRequest) (*CreationResponse, error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to encode policy payload: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/owner/%s/policy", c.serverUrl, ownerID), bytes.NewReader(data))
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
			return nil, fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected statuscode: %d - %s", resp.StatusCode, response.Error)
	}

	var response CreationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// NewClient returns a new client satisfying the api.PolicyInterface interface via the REST API.
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

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
