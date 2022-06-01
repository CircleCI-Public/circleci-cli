package policy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestClient_ListPolicies(t *testing.T) {
	t.Run("expected request", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "GET")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/policy")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.ListPolicies("ownerId", nil)
		assert.NilError(t, err)
	})

	t.Run("List Policies - Bad Request", func(t *testing.T) {
		expectedResponse := `{"error": "active: query string not a boolean."}`

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policies, err := client.ListPolicies("ownerId", nil)
		assert.Equal(t, policies, nil)
		assert.Error(t, err, "unexpected status-code: 400 - active: query string not a boolean.")
	})

	t.Run("List Policies - Forbidden", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policies, err := client.ListPolicies("ownerId", nil)
		assert.Equal(t, policies, nil)
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
	})

	t.Run("List Policies - no policies", func(t *testing.T) {
		expectedResponse := "[]"

		var expectedResponseValue interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedResponse), &expectedResponseValue))

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policies, err := client.ListPolicies("ownerId", nil)
		assert.DeepEqual(t, policies, expectedResponseValue)
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

		var expectedResponseValue interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedResponse), &expectedResponseValue))

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policies, err := client.ListPolicies("ownerId", nil)
		assert.DeepEqual(t, policies, expectedResponseValue)
		assert.NilError(t, err)
	})
}
