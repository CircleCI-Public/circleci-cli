package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestGetGitHubRepositories(t *testing.T) {
	// Mock response data - API returns array directly
	mockRepositories := []Repository{
		{
			ID:            123456,
			Name:          "example-repo",
			FullName:      "myorg/example-repo",
			Private:       false,
			HTMLURL:       "https://github.com/myorg/example-repo",
			CloneURL:      "https://github.com/myorg/example-repo.git",
			SSHURL:        "git@github.com:myorg/example-repo.git",
			Description:   "An example repository",
			Language:      "Go",
			CreatedAt:     "2023-01-01T00:00:00Z",
			UpdatedAt:     "2023-12-01T00:00:00Z",
			PushedAt:      "2023-12-01T12:00:00Z",
			DefaultBranch: "main",
		},
		{
			ID:            789012,
			Name:          "another-repo",
			FullName:      "myorg/another-repo",
			Private:       true,
			HTMLURL:       "https://github.com/myorg/another-repo",
			CloneURL:      "https://github.com/myorg/another-repo.git",
			SSHURL:        "git@github.com:myorg/another-repo.git",
			Description:   "Another example repository",
			Language:      "JavaScript",
			CreatedAt:     "2023-02-01T00:00:00Z",
			UpdatedAt:     "2023-11-15T00:00:00Z",
			PushedAt:      "2023-11-15T15:30:00Z",
			DefaultBranch: "master",
		},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request details
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/private/soc/github-app/organization/test-org-id/repositories", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("Circle-Token"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, version.UserAgent(), r.Header.Get("User-Agent"))

		// Send mock response - array directly
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockRepositories)
	}))
	defer server.Close()

	// Create client with test server URL
	config := settings.Config{
		Token:      "test-token",
		HTTPClient: http.DefaultClient,
	}

	client := &repositoryRestClient{
		token:      config.Token,
		baseURL:    server.URL,
		httpClient: config.HTTPClient,
	}

	// Test the API call
	result, err := client.GetGitHubRepositories("test-org-id")

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
	assert.Len(t, result.Repositories, 2)

	// Check first repository
	repo1 := result.Repositories[0]
	assert.Equal(t, 123456, repo1.ID)
	assert.Equal(t, "example-repo", repo1.Name)
	assert.Equal(t, "myorg/example-repo", repo1.FullName)
	assert.False(t, repo1.Private)
	assert.Equal(t, "Go", repo1.Language)
	assert.Equal(t, "main", repo1.DefaultBranch)

	// Check second repository
	repo2 := result.Repositories[1]
	assert.Equal(t, 789012, repo2.ID)
	assert.Equal(t, "another-repo", repo2.Name)
	assert.Equal(t, "myorg/another-repo", repo2.FullName)
	assert.True(t, repo2.Private)
	assert.Equal(t, "JavaScript", repo2.Language)
	assert.Equal(t, "master", repo2.DefaultBranch)
}

func TestGetGitHubRepositories_ErrorResponse(t *testing.T) {
	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Organization not found",
		})
	}))
	defer server.Close()

	// Create client with test server URL
	config := settings.Config{
		Token:      "test-token",
		HTTPClient: http.DefaultClient,
	}

	client := &repositoryRestClient{
		token:      config.Token,
		baseURL:    server.URL,
		httpClient: config.HTTPClient,
	}

	// Test the API call
	result, err := client.GetGitHubRepositories("nonexistent-org")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "Organization not found")
}

func TestGetGitHubRepositories_NetworkError(t *testing.T) {
	// Create client with invalid URL to simulate network error
	config := settings.Config{
		Token:      "test-token",
		HTTPClient: http.DefaultClient,
	}

	client := &repositoryRestClient{
		token:      config.Token,
		baseURL:    "http://invalid-url-that-does-not-exist.local",
		httpClient: config.HTTPClient,
	}

	// Test the API call
	result, err := client.GetGitHubRepositories("test-org-id")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestNewRepositoryRestClient(t *testing.T) {
	config := settings.Config{
		Token:      "test-token",
		HTTPClient: http.DefaultClient,
	}

	client, err := NewRepositoryRestClient(config)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test-token", client.token)
	assert.Equal(t, "https://bff.circleci.com", client.baseURL)
	assert.Equal(t, http.DefaultClient, client.httpClient)
}
