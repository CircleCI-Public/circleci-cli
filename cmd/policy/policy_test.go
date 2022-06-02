package policy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestListPolicies(t *testing.T) {
	makeCMD := func() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		cmd := NewCommand(config, nil)

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)

		return cmd, stdout, stderr
	}

	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires org-id",
			Args:        []string{"list"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "invalid active filter value",
			Args:        []string{"list", "--owner-id", "ownerID", "--active=badValue"},
			ExpectedErr: `invalid argument "badValue" for "--active" flag: strconv.ParseBool: parsing "badValue": invalid syntax`,
		},
		{
			Name: "should set active to true",
			Args: []string{"list", "--owner-id", "ownerID", "--active"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy?active=true")
				w.Write([]byte("[]"))
			},
			ExpectedOutput: "[]\n",
		},
		{
			Name: "should set active to false",
			Args: []string{"list", "--owner-id", "ownerID", "--active=false"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy?active=false")
				w.Write([]byte("[]"))
			},
			ExpectedOutput: "[]\n",
		},
		{
			Name:        "gets error response",
			Args:        []string{"list", "--owner-id", "ownerID"},
			ExpectedErr: "failed to list policies: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy")
				w.WriteHeader(http.StatusForbidden)
				io.WriteString(w, `{"error": "Forbidden"}`)
			},
		},
		{
			Name: "successfully gets a policy",
			Args: []string{"list", "--owner-id", "ownerID"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy")
				w.Write([]byte(`[
			{
				"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
				"name": "policy_1",
				"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
				"context": "config",
				"active": false,
				"created_at": "2022-05-31T14:15:10.86097Z",
				"modified_at": null
			}
		]`))
			},
			ExpectedOutput: `[
  {
    "active": false,
    "context": "config",
    "created_at": "2022-05-31T14:15:10.86097Z",
    "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
    "modified_at": null,
    "name": "policy_1",
    "owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f"
  }
]
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, _ := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}
			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestCreatePolicy(t *testing.T) {
	makeCMD := func() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		cmd := NewCommand(config, nil)

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)

		return cmd, stdout, stderr
	}

	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id and name and policy",
			Args:        []string{"create"},
			ExpectedErr: "required flag(s) \"name\", \"owner-id\", \"policy\" not set",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"create", "--owner-id", "test-org", "--name", "test-policy", "--policy", "./testdata/test.rego"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/policy")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
					"context": "config",
					"name":    "test-policy",
				})

				w.WriteHeader(http.StatusCreated)
				io.WriteString(w, "{}")
			},
			ExpectedOutput: "{}\n",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, _ := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}

			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestGetPolicy(t *testing.T) {
	makeCMD := func() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
		cmd := NewCommand(config, nil)

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)

		return cmd, stdout, stderr
	}

	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires policy-id",
			Args:        []string{"get", "--owner-id", "ownerID"},
			ExpectedErr: "accepts 1 arg(s), received 0",
		},
		{
			Name:        "requires org-id",
			Args:        []string{"get", "policyID"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "gets error response",
			Args:        []string{"get", "policyID", "--owner-id", "ownerID"},
			ExpectedErr: "failed to get policy: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy/policyID")
				w.WriteHeader(http.StatusForbidden)
				io.WriteString(w, `{"error": "Forbidden"}`)
			},
		},
		{
			Name: "successfully gets a policy",
			Args: []string{"get", "60b7e1a5-c1d7-4422-b813-7a12d353d7c6", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/policy/60b7e1a5-c1d7-4422-b813-7a12d353d7c6")
				w.Write([]byte(`{
					"document_version": 1,
					"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
					"name": "policy_1",
					"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
					"context": "config",
					"content": "package test",
					"active": false,
					"created_at": "2022-05-31T14:15:10.86097Z",
					"modified_at": null
				}`))
			},
			ExpectedOutput: `{
  "active": false,
  "content": "package test",
  "context": "config",
  "created_at": "2022-05-31T14:15:10.86097Z",
  "document_version": 1,
  "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
  "modified_at": null,
  "name": "policy_1",
  "owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f"
}
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, _ := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}

			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}
