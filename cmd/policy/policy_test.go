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

func TestPushPolicyBundle(t *testing.T) {
	testcases := []struct {
		Name          string
		Args          []string
		ServerHandler http.HandlerFunc
		ExpectedErr   string
	}{
		{
			Name:        "requires policy bundle directory path ",
			Args:        []string{"push", "--owner-id", "ownerID"},
			ExpectedErr: "accepts 1 arg(s), received 0",
		},
		{
			Name:        "requires owner-id",
			Args:        []string{"push", "./testdata/test0/policy.rego"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "fails for policy bundle directory path not found",
			Args:        []string{"push", "./testdata/directory_not_present", "--owner-id", "test-org"},
			ExpectedErr: "failed to walk policy directory path: lstat ./testdata/directory_not_present: ",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"push", "./testdata/test0", "--owner-id", "test-org", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/context/custom/policy-bundle")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"policies": map[string]interface{}{
						"meta-policy.rego": `package org

policy_name["meta_policy_test"]
enable_rule["enabled"] { data.meta.branch == "main" }
enable_rule["disabled"] { data.meta.project_id != "test-project-id" }
`,
						"policy.rego": `package org

policy_name["test"]
enable_rule["branch_is_main"]
branch_is_main = "branch must be main!" { input.branch != "main" }
`,
					},
				})

				w.WriteHeader(http.StatusCreated)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, _, _ := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}
		})
	}
}

func TestFetchPolicyBundle(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id",
			Args:        []string{"fetch", "policyID"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "gets error response",
			Args:        []string{"fetch", "policyName", "--owner-id", "ownerID", "--context", "someContext"},
			ExpectedErr: "failed to fetch policy bundle: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/someContext/policy-bundle/policyName")
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully fetches single policy",
			Args: []string{"fetch", "my_policy", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/policy-bundle/my_policy")
				_, err := w.Write([]byte(`{
					"content": "package org\n\npolicy_name[\"my_policy\"] { true }",
					"created_at": "2022-08-10T10:47:01.859756-04:00",
  					"created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
  					"name": "my_policy"
				}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "content": "package org\n\npolicy_name[\"my_policy\"] { true }",
  "created_at": "2022-08-10T10:47:01.859756-04:00",
  "created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
  "name": "my_policy"
}
`,
		},
		{
			Name: "successfully fetches policy bundle",
			Args: []string{"fetch", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/policy-bundle/")
				_, err := w.Write([]byte(`{
  "a": {
    "content": "package org\n\npolicy_name[\"a\"] { true }",
    "created_at": "2022-08-10T10:47:01.859756-04:00",
    "created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
    "name": "a"
  },
  "b": {
    "content": "package org\n\npolicy_name[\"b\"] { true }",
    "created_at": "2022-08-10T10:47:01.859756-04:00",
    "created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
    "name": "b"
  }
}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "a": {
    "content": "package org\n\npolicy_name[\"a\"] { true }",
    "created_at": "2022-08-10T10:47:01.859756-04:00",
    "created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
    "name": "a"
  },
  "b": {
    "content": "package org\n\npolicy_name[\"b\"] { true }",
    "created_at": "2022-08-10T10:47:01.859756-04:00",
    "created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
    "name": "b"
  }
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
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision")
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
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision?after=2022-03-14T00%3A00%3A00Z&before=2022-03-15T00%3A00%3A00Z&branch=branchValue&project_id=projectIDValue&status=PASS")
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
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision")
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
						assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision")
						_, err := w.Write([]byte(`
							[
							  {
								"created_at": "2022-08-11T09:20:40.674594-04:00",
								"decision": {
								  "enabled_rules": [
									"branch_is_main"
								  ],
								  "status": "PASS"
								},
								"metadata": {},
								"policies": [
								  "8c69adc542bcfd6e65f5d5a2b6a4e3764480db2253cd075d0954e64a1f827a9c695c916d5a49302991df781447b3951410824dce8a8282d11ed56302272cf6fb",
								  "3124131001ec20b4b524260ababa6411190a1bc9c5ac3219ccc2d21109fc5faf4bb9f7bbe38f3f798d9c232d68564390e0ca560877711f3f2ff7f89e10eef685"
								],
								"time_taken_ms": 4
							  }
							]`),
						)
						assert.NilError(t, err)
					} else if count == 1 {
						assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision?offset=1")
						_, err := w.Write([]byte("[]"))
						assert.NilError(t, err)
					} else {
						t.Fatal("did not expect more than two requests but received a third")
					}
				}
			}(),
			ExpectedOutput: `[
  {
    "created_at": "2022-08-11T09:20:40.674594-04:00",
    "decision": {
      "enabled_rules": [
        "branch_is_main"
      ],
      "status": "PASS"
    },
    "metadata": {},
    "policies": [
      "8c69adc542bcfd6e65f5d5a2b6a4e3764480db2253cd075d0954e64a1f827a9c695c916d5a49302991df781447b3951410824dce8a8282d11ed56302272cf6fb",
      "3124131001ec20b4b524260ababa6411190a1bc9c5ac3219ccc2d21109fc5faf4bb9f7bbe38f3f798d9c232d68564390e0ca560877711f3f2ff7f89e10eef685"
    ],
    "time_taken_ms": 4
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
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/config/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"PASS\"\n}\n",
		},
		{
			Name: "sends expected request with context",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/custom/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"PASS\"\n}\n",
		},
		{
			Name: "sends expected request with metadata",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--context", "custom", "--metafile", "./testdata/test1/meta.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/custom/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
					"metadata": map[string]interface{}{
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
			Args: []string{"decide", "--input", "./testdata/test1/test.yml", "--owner-id", "test-owner"},
			ServerHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
				_, _ = io.WriteString(w, `{"error":"oopsie!"}`)
			},

			ExpectedErr: "failed to make decision: unexpected status-code: 500 - oopsie!",
		},
		{
			Name:        "fails if neither local-policy nor owner-id is provided",
			Args:        []string{"decide", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "--owner-id or --policy is required",
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"decide", "--policy", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"decide", "--policy", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to get path info: ",
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
				"decide", "--metafile", "./testdata/test1/meta.yml", "--policy", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--input",
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

func TestRawOPAEvaluationCommand(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires flags",
			Args:        []string{"eval"},
			ExpectedErr: `required flag(s) "input", "policy" not set`,
		},
		{
			Name:        "fails if local-policy is not provided",
			Args:        []string{"eval", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: `required flag(s) "policy" not set`,
		},
		{
			Name:        "fails if input is not provided",
			Args:        []string{"eval", "--policy", "./testdata/test0/policy.rego"},
			ExpectedErr: `required flag(s) "input" not set`,
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"eval", "--policy", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"eval", "--policy", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to get path info: ",
		},
		{
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input and metadata",
			Args: []string{
				"eval", "--metafile", "./testdata/test1/meta.yml", "--policy", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--input",
				"./testdata/test0/config.yml",
			},
			ExpectedOutput: `{
  "meta": {
    "branch": "main",
    "project_id": "test-project-id"
  },
  "org": {
    "enable_rule": [
      "enabled"
    ],
    "policy_name": [
      "meta_policy_test"
    ]
  }
}
`,
		},
		{
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input, metadata and query",
			Args: []string{
				"eval", "--metafile", "./testdata/test1/meta.yml", "--policy", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--input",
				"./testdata/test0/config.yml", "--query", "data.org.enable_rule",
			},
			ExpectedOutput: `[
  "enabled"
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
