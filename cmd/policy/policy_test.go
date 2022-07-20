package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestListPolicies(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id",
			Args:        []string{"list"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "gets error response",
			Args:        []string{"list", "--owner-id", "ownerID"},
			ExpectedErr: "failed to list policies: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy")
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name:        "gets bad json response",
			Args:        []string{"list", "--owner-id", "ownerID"},
			ExpectedErr: "failed to list policies: failed to decode response body: invalid character '}' looking for beginning of value",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy")
				_, err := w.Write([]byte(`{"bad json": }`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully gets a policy",
			Args: []string{"list", "--owner-id", "ownerID"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy")
				_, err := w.Write([]byte(`[
			{
				"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
				"name": "policy_1",
				"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
				"context": "config",
				"created_at": "2022-05-31T14:15:10.86097Z",
				"modified_at": null
			}
		]`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `[
  {
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
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id and policy",
			Args:        []string{"create"},
			ExpectedErr: "required flag(s) \"owner-id\", \"policy\" not set",
		},
		{
			Name:        "fails for policy file not found",
			Args:        []string{"create", "--owner-id", "test-org", "--policy", "./testdata/file_not_present.rego"},
			ExpectedErr: "failed to read policy file: open ./testdata/file_not_present.rego: ",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"create", "--owner-id", "test-org", "--policy", "./testdata/test.rego"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/policy")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
					"context": "config",
				})

				w.WriteHeader(http.StatusCreated)
				_, err := w.Write([]byte("{}"))
				assert.NilError(t, err)
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
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestGetPolicy(t *testing.T) {
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
			Name:        "requires owner-id",
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
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully gets a policy",
			Args: []string{"get", "60b7e1a5-c1d7-4422-b813-7a12d353d7c6", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/policy/60b7e1a5-c1d7-4422-b813-7a12d353d7c6")
				_, err := w.Write([]byte(`{
					"document_version": 1,
					"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
					"name": "policy_1",
					"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
					"context": "config",
					"content": "package test",
					"created_at": "2022-05-31T14:15:10.86097Z",
					"modified_at": null
				}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
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

func TestDeletePolicy(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires policy-id",
			Args:        []string{"delete", "--owner-id", "ownerID"},
			ExpectedErr: "accepts 1 arg(s), received 0",
		},
		{
			Name:        "requires owner-id",
			Args:        []string{"delete", "policyID"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "gets error response",
			Args:        []string{"delete", "policyID", "--owner-id", "ownerID"},
			ExpectedErr: "failed to delete policy: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/policy/policyID")
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully deletes a policy",
			Args: []string{"delete", "60b7e1a5-c1d7-4422-b813-7a12d353d7c6", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/policy/60b7e1a5-c1d7-4422-b813-7a12d353d7c6")
				w.WriteHeader(http.StatusNoContent)
			},
			ExpectedOutput: "Deleted Successfully\n",
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

func TestUpdatePolicy(t *testing.T) {
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
			Name:        "requires owner-id flag",
			Args:        []string{"update", "testID"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "requires policy id",
			Args:        []string{"update", "--owner-id", "test-org"},
			ExpectedErr: "accepts 1 arg(s), received 0",
		},
		{
			Name:        "fails if policy file not found",
			Args:        []string{"update", "test-policy-id", "--owner-id", "test-org", "--policy", "./testdata/file_not_present.rego"},
			ExpectedErr: "failed to read policy file: open ./testdata/file_not_present.rego: ",
		},
		{
			Name: "gets error response",
			Args: []string{"update", "test-policy-id", "--owner-id", "test-org", "--policy", "./testdata/test.rego"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-org/policy/test-policy-id")
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
				})

				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
			ExpectedErr: "failed to update policy: unexpected status-code: 403 - Forbidden",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"update", "test-policy-id", "--owner-id", "test-org", "--policy", "./testdata/test.rego"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-org/policy/test-policy-id")
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
				})

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("{}"))
				assert.NilError(t, err)
			},
			ExpectedOutput: "{}\n",
		},
		{
			Name: "explicitly set config",
			Args: []string{"update", "test-policy-id", "--owner-id", "test-org", "--policy", "./testdata/test.rego", "--context", "config"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-org/policy/test-policy-id")
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
					"context": "config",
				})

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("{}"))
				assert.NilError(t, err)
			},
			ExpectedOutput: "{}\n",
		},
		{
			Name: "sends appropriate desired request with only policy path",
			Args: []string{"update", "test-policy-id", "--owner-id", "test-org", "--policy", "./testdata/test.rego"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-org/policy/test-policy-id")
				assert.DeepEqual(t, body, map[string]interface{}{
					"content": "package test",
				})

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("{}"))
				assert.NilError(t, err)
			},
			ExpectedOutput: "{}\n",
		},
		{
			Name:        "check at least one field is changed",
			Args:        []string{"update", "test-policy-id", "--owner-id", "test-org"},
			ExpectedErr: "one of policy or context must be set",
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
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestGetDecisionLogs(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id",
			Args:        []string{"logs"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "invalid --after filter value",
			Args:        []string{"logs", "--owner-id", "ownerID", "--after", "1/2/2022"},
			ExpectedErr: `error in parsing --after value: This date has ambiguous mm/dd vs dd/mm type format`,
		},
		{
			Name:        "invalid --before filter value",
			Args:        []string{"logs", "--owner-id", "ownerID", "--before", "1/2/2022"},
			ExpectedErr: `error in parsing --before value: This date has ambiguous mm/dd vs dd/mm type format`,
		},
		{
			Name: "no filter is set",
			Args: []string{"logs", "--owner-id", "ownerID"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/decision")
				_, err := w.Write([]byte("[]"))
				assert.NilError(t, err)
			},
			ExpectedOutput: "[]\n",
		},
		{
			Name: "all filters are set",
			Args: []string{
				"logs", "--owner-id", "ownerID", "--status", "PASS", "--after", "2022/03/14", "--before", "2022/03/15",
				"--branch", "branchValue", "--project-id", "projectIDValue",
			},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/decision?after=2022-03-14T00%3A00%3A00Z&before=2022-03-15T00%3A00%3A00Z&branch=branchValue&project_id=projectIDValue&status=PASS")
				_, err := w.Write([]byte("[]"))
				assert.NilError(t, err)
			},
			ExpectedOutput: "[]\n",
		},
		{
			Name:        "gets error response",
			Args:        []string{"logs", "--owner-id", "ownerID"},
			ExpectedErr: "failed to get policy decision logs: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/decision")
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully gets decision logs",
			Args: []string{"logs", "--owner-id", "ownerID"},
			ServerHandler: func() http.HandlerFunc {
				var count int
				return func(w http.ResponseWriter, r *http.Request) {
					defer func() { count++ }()

					assert.Equal(t, r.Method, "GET")

					if count == 0 {
						assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/decision")
						_, err := w.Write([]byte(`
							[
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
								}
							]`),
						)
						assert.NilError(t, err)
					} else if count == 1 {
						assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/decision?offset=1")
						_, err := w.Write([]byte("[]"))
						assert.NilError(t, err)
					} else {
						t.Fatal("did not expect more than two requests but received a third")
					}
				}
			}(),
			ExpectedOutput: `[
  {
    "created_at": "2022-06-08T16:56:22.179906Z",
    "decision": {
      "status": "PASS"
    },
    "metadata": {},
    "policies": [
      {
        "id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
        "version": 2
      }
    ]
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
			fmt.Println(stdout.String())
			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestMakeDecisionCommand(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires flags",
			Args:        []string{"decide"},
			ExpectedErr: `required flag(s) "input" not set`,
		},
		{
			Name: "sends expected request",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"context": "config",
					"input":   "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"PASS\"\n}\n",
		},
		{
			Name: "sends expected request with context",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test.yml", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"context": "custom",
					"input":   "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"PASS\"\n}\n",
		},
		{
			Name: "sends expected request with metadata",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test.yml", "--context", "custom", "--metafile", "./testdata/meta.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"context": "custom",
					"input":   "test: config\n",
					"metadata": map[string]any{
						"project_id": "test-project-id",
						"branch":     "main",
					},
				})

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"PASS\"\n}\n",
		},
		{
			Name: "fails on unexpected status code",
			Args: []string{"decide", "--input", "./testdata/test.yml", "--owner-id", "test-owner"},
			ServerHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
				_, _ = io.WriteString(w, `{"error":"oopsie!"}`)
			},

			ExpectedErr: "failed to make decision: unexpected status-code: 500 - oopsie!",
		},
		{
			Name:        "fails if neither local-policy nor owner-id is provided",
			Args:        []string{"decide", "--input", "./testdata/test.yml"},
			ExpectedErr: "--owner-id or --policy is required",
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"decide", "--policy", "./testdata/policy.rego", "--input", "./testdata/no_such_file.yml"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"decide", "--policy", "./testdata/no_such_file.rego", "--input", "./testdata/test.yml"},
			ExpectedErr: "failed to make decision: failed to get path info: ",
		},
		{
			Name: "successfully performs decision for policy FILE provided locally",
			Args: []string{
				"decide", "--policy", "./testdata/test0/policy.rego", "--input",
				"./testdata/test0/config.yml",
			},
			ExpectedOutput: `{
  "status": "PASS",
  "enabled_rules": [
    "branch_is_main"
  ]
}
`,
		},
		{
			Name: "successfully performs decision with metadata for policy FILE provided locally",
			Args: []string{
				"decide", "--metafile", "./testdata/meta.yml", "--policy", "./testdata/test0/meta-policy.rego", "--input",
				"./testdata/test0/config.yml",
			},
			ExpectedOutput: `{
  "status": "PASS",
  "enabled_rules": [
    "enabled"
  ]
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
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}
			assert.Equal(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func makeCMD() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
	cmd := NewCommand(config, nil)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}
