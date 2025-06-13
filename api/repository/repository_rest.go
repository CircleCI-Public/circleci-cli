package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

type repositoryRestClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

var _ RepositoryClient = &repositoryRestClient{}

func NewRepositoryRestClient(config settings.Config) (*repositoryRestClient, error) {
	return &repositoryRestClient{
		token:      config.Token,
		baseURL:    "https://bff.circleci.com",
		httpClient: config.HTTPClient,
	}, nil
}

func (c *repositoryRestClient) GetGitHubRepositories(orgID string) (*GetRepositoriesResponse, error) {
	path := fmt.Sprintf("/private/soc/github-app/organization/%s/repositories", orgID)

	req, err := c.newHTTPRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &errorResp); err != nil {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}
		message := errorResp.Message
		if message == "" {
			message = errorResp.Error
		}
		if message == "" {
			message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API request failed: %s", message)
	}

	// The API returns an array of repositories directly
	var repositories []Repository
	if err := json.Unmarshal(bodyBytes, &repositories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// We want to return a struct with the total count of repositories
	result := &GetRepositoriesResponse{
		Repositories: repositories,
		TotalCount:   len(repositories),
	}

	return result, nil
}

func (c *repositoryRestClient) newHTTPRequest(method, path string, body io.Reader) (*http.Request, error) {
	fullURL := c.baseURL + path

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Circle-Token", c.token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())

	if commandStr := header.GetCommandStr(); commandStr != "" {
		req.Header.Set("Circleci-Cli-Command", commandStr)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}
