package info

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

// InfoClient An interface with all the Info Functions.
type InfoClient interface {
	GetInfo() (*[]Organization, error)
}

// errorResponse used to handle error messages from the API.
type errorResponse struct {
	Message *string `json:"message"`
}

// InfoRESTClient A restful implementation of the InfoClient
type InfoRESTClient struct {
	token  string
	server string
	client *http.Client
}

// organization json org info
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetInfo
func (c *InfoRESTClient) GetInfo() (*[]Organization, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse("me/collaborations")
	if err != nil {
		return nil, err
	}
	req, err := c.newHTTPRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct new request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err

		}
		return nil, errors.New(*dest.Message)
	}

	orgs := make([]Organization, 0)
	if err := json.Unmarshal(bodyBytes, &orgs); err != nil {
		return nil, err
	}

	return &orgs, nil
}

// newHTTPRequest Creates a new standard HTTP request object used to communicate with the API
func (c *InfoRESTClient) newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("circle-token", c.token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Add("Circleci-Cli-Command", commandStr)
	}
	return req, nil
}

// Creates a new client to talk with the rest info endpoints.
func NewInfoClient(config settings.Config) (InfoClient, error) {
	serverURL, err := config.ServerURL()
	if err != nil {
		return nil, err
	}

	client := &InfoRESTClient{
		token:  config.Token,
		server: serverURL.String(),
		client: config.HTTPClient,
	}

	return client, nil
}
