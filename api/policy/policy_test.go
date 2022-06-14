package policy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestClientListPolicies(t *testing.T) {
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
			_, err := w.Write([]byte("[]"))
			assert.NilError(t, err)
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
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
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
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
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
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
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
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policies, err := client.ListPolicies("ownerId", nil)
		assert.DeepEqual(t, policies, expectedResponseValue)
		assert.NilError(t, err)
	})
}

func TestClientGetPolicy(t *testing.T) {
	t.Run("expected request", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "GET")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/policy/policyID")

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.GetPolicy("ownerId", "policyID")
		assert.NilError(t, err)
	})

	t.Run("Get Policy - Bad Request", func(t *testing.T) {
		expectedResponse := `{"error": "PolicyID: must be a valid UUID."}`

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policy, err := client.GetPolicy("ownerId", "policyID")
		assert.Equal(t, policy, nil)
		assert.Error(t, err, "unexpected status-code: 400 - PolicyID: must be a valid UUID.")
	})

	t.Run("Get Policy - Forbidden", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policy, err := client.GetPolicy("ownerId", "policyID")
		assert.Equal(t, policy, nil)
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
	})

	t.Run("Get Policy - Not Found", func(t *testing.T) {
		expectedResponse := `{"error": "policy not found"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policy, err := client.GetPolicy("ownerId", "a917a0ab-ceb6-482d-9a4e-f2f6b8bdfdca")
		assert.Equal(t, policy, nil)
		assert.Error(t, err, "unexpected status-code: 404 - policy not found")
	})

	t.Run("Get Policy - successfully gets a policy", func(t *testing.T) {
		expectedResponse := `{
   			 "document_version": 1,
   			 "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
   			 "name": "policy_1",
   			 "owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
   			 "context": "config",
   			 "content": "package test",
   			 "active": false,
   			 "created_at": "2022-05-31T14:15:10.86097Z",
   			 "modified_at": null
		}`

		var expectedResponseValue interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedResponse), &expectedResponseValue))

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		policy, err := client.GetPolicy("462d67f8-b232-4da4-a7de-0c86dd667d3f", "60b7e1a5-c1d7-4422-b813-7a12d353d7c6")
		assert.DeepEqual(t, policy, expectedResponseValue)
		assert.NilError(t, err)
	})
}

func TestClientCreatePolicy(t *testing.T) {
	t.Run("expected request", func(t *testing.T) {
		req := CreationRequest{
			Name:    "test-name",
			Context: "config",
			Content: "test-content",
		}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/policy")

			var actual CreationRequest
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&actual))
			assert.DeepEqual(t, actual, req)

			w.WriteHeader(http.StatusCreated)
			_, err := w.Write([]byte("{}"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		client := NewClient(svr.URL, config)

		_, err := client.CreatePolicy("ownerId", req)
		assert.NilError(t, err)
	})

	t.Run("unexpected status code", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.CreatePolicy("ownerId", CreationRequest{})
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
	})
}

func TestClientDeletePolicy(t *testing.T) {
	t.Run("expected request", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "DELETE")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/policy/policyID")

			w.WriteHeader(http.StatusNoContent)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		err := client.DeletePolicy("ownerId", "policyID")
		assert.NilError(t, err)
	})

	t.Run("Delete Policy - Bad Request", func(t *testing.T) {
		expectedResponse := `{"error": "PolicyID: must be a valid UUID."}`

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		err := client.DeletePolicy("ownerId", "policyID")
		assert.Error(t, err, "unexpected status-code: 400 - PolicyID: must be a valid UUID.")
	})

	t.Run("Delete Policy - Forbidden", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		err := client.DeletePolicy("ownerId", "policyID")
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
	})

	t.Run("Delete Policy - Not Found", func(t *testing.T) {
		expectedResponse := `{"error": "policy not found"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		err := client.DeletePolicy("ownerId", "a917a0ab-ceb6-482d-9a4e-f2f6b8bdfdca")
		assert.Error(t, err, "unexpected status-code: 404 - policy not found")
	})

	t.Run("Delete Policy - successfully deletes a policy", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		err := client.DeletePolicy("462d67f8-b232-4da4-a7de-0c86dd667d3f", "60b7e1a5-c1d7-4422-b813-7a12d353d7c6")
		assert.NilError(t, err)
	})
}

func TestClientUpdatePolicy(t *testing.T) {
	t.Run("expected request", func(t *testing.T) {
		isActive := true
		name := "test-name"
		context := "config"
		content := "test-content"
		req := UpdateRequest{
			Name:    &name,
			Context: &context,
			Content: &content,
			Active:  &isActive,
		}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "PATCH")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerID/policy/policyID")

			var actual UpdateRequest
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&actual))
			assert.DeepEqual(t, actual, req)

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{}"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		client := NewClient(svr.URL, config)

		_, err := client.UpdatePolicy("ownerID", "policyID", req)
		assert.NilError(t, err)
	})

	t.Run("nil active", func(t *testing.T) {
		name := "test-name"
		context := "config"
		content := "test-content"
		req := UpdateRequest{
			Name:    &name,
			Context: &context,
			Content: &content,
			Active:  nil,
		}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "PATCH")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerID/policy/policyID")

			var actual UpdateRequest
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&actual))
			assert.DeepEqual(t, actual, req)

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{}"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		client := NewClient(svr.URL, config)

		_, err := client.UpdatePolicy("ownerID", "policyID", req)
		assert.NilError(t, err)
	})

	t.Run("unexpected status code", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.UpdatePolicy("ownerId", "policyId", UpdateRequest{})
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
	})

	t.Run("no changes", func(t *testing.T) {
		req := UpdateRequest{}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "PATCH")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerID/policy/policyID")

			var actual UpdateRequest
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&actual))
			assert.DeepEqual(t, actual, req)

			expectedResponse := `{"error": "at least one of name, context, content, or active cannot be blank"}`
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		client := NewClient(svr.URL, config)

		_, err := client.UpdatePolicy("ownerID", "policyID", req)
		assert.Error(t, err, "unexpected status-code: 400 - at least one of name, context, content, or active cannot be blank")
	})

	t.Run("one change", func(t *testing.T) {
		name := "test-name"
		req := UpdateRequest{
			Name: &name,
		}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "PATCH")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerID/policy/policyID")

			var actual UpdateRequest
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&actual))
			assert.DeepEqual(t, actual, req)

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{}"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		client := NewClient(svr.URL, config)

		_, err := client.UpdatePolicy("ownerID", "policyID", req)
		assert.NilError(t, err)
	})
}

func TestClientGetDecisionLogs(t *testing.T) {
	t.Run("expected request without any filters", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "GET")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/decision")
			assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerId/decision")

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{})
		assert.NilError(t, err)
	})

	t.Run("expected request without only one filter", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "GET")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/decision")
			assert.Equal(t, r.URL.RawQuery, "project_id=projectIDValue")

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{ProjectID: "projectIDValue"})
		assert.NilError(t, err)
	})

	t.Run("expected request with all filters", func(t *testing.T) {
		testTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")
			assert.Equal(t, r.Header.Get("accept"), "application/json")
			assert.Equal(t, r.Header.Get("content-type"), "application/json")
			assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
			assert.Equal(t, r.Header.Get("circle-token"), "testtoken")

			assert.Equal(t, r.Method, "GET")
			assert.Equal(t, r.URL.Path, "/api/v1/owner/ownerId/decision")
			assert.Equal(
				t,
				r.URL.RawQuery,
				"after=2000-01-01T00%3A00%3A00Z&before=2000-01-01T00%3A00%3A00Z&branch=branchValue&offset=42&project_id=projectIDValue",
			)

			assert.Equal(t, r.URL.Query().Get("before"), testTime.Format(time.RFC3339))
			assert.Equal(t, r.URL.Query().Get("after"), testTime.Format(time.RFC3339))

			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		_, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{
			After:     &testTime,
			Before:    &testTime,
			Branch:    "branchValue",
			ProjectID: "projectIDValue",
			Offset:    42,
		})
		assert.NilError(t, err)
	})

	t.Run("Get Decision Logs - Bad Request", func(t *testing.T) {
		expectedResponse := `{"error": "Offset: must be an integer number."}`

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		logs, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{})
		assert.Error(t, err, "unexpected status-code: 400 - Offset: must be an integer number.")
		assert.Equal(t, len(logs), 0)
	})

	t.Run("Get Decision Logs - Forbidden", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		logs, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{})
		assert.Error(t, err, "unexpected status-code: 403 - Forbidden")
		assert.Equal(t, len(logs), 0)
	})

	t.Run("Get Decision Logs - no decision logs", func(t *testing.T) {
		expectedResponse := "[]"

		var expectedResponseValue []interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedResponse), &expectedResponseValue))

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		logs, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{})
		assert.DeepEqual(t, logs, expectedResponseValue)
		assert.NilError(t, err)
	})

	t.Run("Get Decision Logs - some logs", func(t *testing.T) {
		expectedResponse := `[
    {
        "metadata": {},
        "created_at": "2022-06-08T16:56:22.179906Z",
        "policies": [
            {
                "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
                "version": 2
            }
        ],
        "decision": {
            "status": "PASS"
        }
    },
    {
        "metadata": {},
        "created_at": "2022-06-08T17:06:14.591951Z",
        "policies": [
            {
                "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
                "version": 2
            }
        ],
        "decision": {
            "status": "PASS"
        }
    }
]`

		var expectedResponseValue []interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedResponse), &expectedResponseValue))

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(expectedResponse))
			assert.NilError(t, err)
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		client := NewClient(svr.URL, config)

		logs, err := client.GetDecisionLogs("ownerId", DecisionQueryRequest{})
		assert.DeepEqual(t, logs, expectedResponseValue)
		assert.NilError(t, err)
	})
}

func TestMakeDecision(t *testing.T) {
	testcases := []struct {
		Name             string
		OwnerID          string
		Request          DecisionRequest
		Handler          http.HandlerFunc
		ExpectedError    error
		ExpectedDecision interface{}
	}{
		{
			Name:    "sends expexted request",
			OwnerID: "test-owner",
			Request: DecisionRequest{
				Input:   "test-input",
				Context: "test-context",
			},
			Handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/decision")
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.Header.Get("Circle-Token"), "test-token")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"context": "test-context",
					"input":   "test-input",
				})

				json.NewEncoder(w).Encode(map[string]string{"status": "PASS"})
			},
			ExpectedDecision: map[string]interface{}{"status": "PASS"},
		},
		{
			Name:    "unexpected statuscode",
			OwnerID: "test-owner",
			Request: DecisionRequest{},
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
				io.WriteString(w, `{"error":"that was a bad request!"}`)
			},
			ExpectedError: errors.New("unexpected status-code: 400 - that was a bad request!"),
		},

		{
			Name:    "unexpected statuscode no body",
			OwnerID: "test-owner",
			Request: DecisionRequest{},
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
			ExpectedError: errors.New("unexpected status-code: 204"),
		},
		{
			Name:    "bad decoding",
			OwnerID: "test-owner",
			Request: DecisionRequest{},
			Handler: func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, "not a json response")
			},
			ExpectedError: errors.New("failed to decode response body: invalid character 'o' in literal null (expecting 'u')"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			svr := httptest.NewServer(tc.Handler)
			defer svr.Close()

			client := NewClient(svr.URL, &settings.Config{Token: "test-token", HTTPClient: http.DefaultClient})

			decision, err := client.MakeDecision(tc.OwnerID, tc.Request)
			if tc.ExpectedError == nil {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedError.Error())
				return
			}

			assert.DeepEqual(t, decision, tc.ExpectedDecision)
		})
	}
}
