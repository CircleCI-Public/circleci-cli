package policy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestClient_ListPolicies(t *testing.T) {
	t.Run("List Policies - Bad Request", func(t *testing.T) {
		expectedResponse := `{
    "error": "active: query string not a boolean."
}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()
		config := settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(config)
		client.serverUrl = svr.URL

		policies, err := client.ListPolicies("ownerId", "badValue")
		assert.Equal(t, policies, "")
		assert.Error(t, err, "active: query string not a boolean.")
	})

	t.Run("List Policies - Forbidden", func(t *testing.T) {
		expectedResponse := `{
    "error": "Forbidden"
}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()
		config := settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(config)
		client.serverUrl = svr.URL

		policies, err := client.ListPolicies("ownerId", "")
		assert.Equal(t, policies, "")
		assert.Error(t, err, "Forbidden")
	})

	t.Run("List Policies - no policies", func(t *testing.T) {
		expectedResponse := "[]"
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()
		config := settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(config)
		client.serverUrl = svr.URL

		policies, err := client.ListPolicies("ownerId", "")
		assert.Equal(t, policies, expectedResponse)
		assert.NilError(t, err)
	})

	t.Run("List Policies - some policies", func(t *testing.T) {
		expectedResponse := `[
	{
		"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
		"name": "policy_1",
		"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
		"context": "config",
		"active": false,
		"created_at": "2022-05-31T14:15:10.86097Z",
		"modified_at": null
	},
	{
		"id": "a917a0ab-ceb6-482d-9a4e-f2f6b8bdfdcd",
		"name": "policy_2",
		"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
		"context": "config",
		"active": true,
		"created_at": "2022-05-31T14:15:23.582383Z",
		"modified_at": "2022-05-31T14:15:46.72321Z"
	}
]`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()
		config := settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(config)
		client.serverUrl = svr.URL

		policies, err := client.ListPolicies("ownerId", "")
		assert.Equal(t, policies, expectedResponse)
		assert.NilError(t, err)
	})
}
