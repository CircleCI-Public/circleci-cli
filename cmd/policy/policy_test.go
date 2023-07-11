package policy

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/config"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

//go:embed testdata
var testdata embed.FS

func testdataContent(t *testing.T, filePath string) string {
	data, err := testdata.ReadFile(path.Join(".", "testdata", filePath))
	assert.NoError(t, err)
	return string(data)
}

func TestPushPolicyWithPrompt(t *testing.T) {
	var requestCount int

	expectedURLs := []string{
		"/api/v1/owner/test-org/context/config/policy-bundle?dry=true",
		"/api/v1/owner/test-org/context/config/policy-bundle",
	}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, expectedURLs[requestCount], r.URL.String())
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]interface{}{
			"policies": map[string]interface{}{
				filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
				filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
			},
		}, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("{}"))
		requestCount++
	}))
	defer svr.Close()

	config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient, IsTelemetryDisabled: true}
	cmd := NewCommand(config, nil)

	buffer := makeSafeBuffer()

	pr, pw := io.Pipe()

	cmd.SetOut(buffer)
	cmd.SetErr(buffer)
	cmd.SetIn(pr)

	cmd.SetArgs([]string{
		"push", "./testdata/test0",
		"--owner-id", "test-org",
		"--policy-base-url", svr.URL,
	})

	done := make(chan struct{})
	go func() {
		assert.NoError(t, cmd.Execute())
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	expectedMessage := "The following changes are going to be made: {}\n\nDo you wish to continue? (y/N) "
	assert.Equal(t, expectedMessage, buffer.String())

	_, err := pw.Write([]byte("y\n"))
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "\nPolicy Bundle Pushed Successfully\n\ndiff: {}\n", buffer.String()[len(expectedMessage):])

	<-done
}

func TestPushPolicyBundleNoPrompt(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedErr    string
		ExpectedStdErr string
		ExpectedStdOut string
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
			ExpectedErr: "failed to walk policy directory path: ",
		},
		{
			Name:        "fails if policy path points to a file instead of directory",
			Args:        []string{"push", "./testdata/test0/policy.rego", "--owner-id", "test-org"},
			ExpectedErr: "failed to walk policy directory path: policy path is not a directory",
		},
		{
			Name: "no policy files in given policy directory path",
			Args: []string{"push", "./testdata/test0/no-valid-policy-files", "--owner-id", "test-org", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-org/context/custom/policy-bundle", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{
					"policies": map[string]interface{}{},
				}, body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{}"))
			},
			ExpectedStdOut: "{}\n",
			ExpectedStdErr: "Policy Bundle Pushed Successfully\n\ndiff: ",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"push", "./testdata/test0", "--owner-id", "test-org", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-org/context/custom/policy-bundle", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{
					"policies": map[string]interface{}{
						filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
						filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
					},
				}, body)

				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{}"))
			},
			ExpectedStdOut: "{}\n",
			ExpectedStdErr: "Policy Bundle Pushed Successfully\n\ndiff: ",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, stderr := makeCMD("", "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL, "--no-prompt"))

			err := cmd.Execute()
			if tc.ExpectedErr != "" {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedStdOut, stdout.String())
			assert.Equal(t, tc.ExpectedStdErr, stderr.String())
		})
	}
}

func TestDiffPolicyBundle(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedErr    string
		ExpectedStdErr string
		ExpectedStdOut string
	}{
		{
			Name:        "requires policy bundle directory path ",
			Args:        []string{"diff", "--owner-id", "ownerID"},
			ExpectedErr: "accepts 1 arg(s), received 0",
		},
		{
			Name:        "requires owner-id",
			Args:        []string{"diff", "./testdata/test0/policy.rego"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "fails for policy bundle directory path not found",
			Args:        []string{"diff", "./testdata/directory_not_present", "--owner-id", "test-org"},
			ExpectedErr: "failed to walk policy directory path: ",
		},
		{
			Name:        "fails if policy path points to a file instead of directory",
			Args:        []string{"diff", "./testdata/test0/policy.rego", "--owner-id", "test-org"},
			ExpectedErr: "failed to walk policy directory path: policy path is not a directory",
		},
		{
			Name: "no policy files in given policy directory path",
			Args: []string{"diff", "./testdata/test0/no-valid-policy-files", "--owner-id", "test-org", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-org/context/custom/policy-bundle?dry=true", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{
					"policies": map[string]interface{}{},
				}, body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{}"))
			},
			ExpectedStdOut: "{}\n",
		},
		{
			Name: "sends appropriate desired request",
			Args: []string{"diff", "./testdata/test0", "--owner-id", "test-org", "--context", "custom"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-org/context/custom/policy-bundle?dry=true", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{
					"policies": map[string]interface{}{
						filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
						filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
					},
				}, body)

				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{}"))
			},
			ExpectedStdOut: "{}\n",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, stderr := makeCMD("", "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr != "" {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedStdOut, stdout.String())
			assert.Equal(t, tc.ExpectedStdErr, stderr.String())
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
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/ownerID/context/someContext/policy-bundle/policyName", r.URL.String())
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NoError(t, err)
			},
		},
		{
			Name: "successfully fetches single policy",
			Args: []string{"fetch", "my_policy", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/policy-bundle/my_policy", r.URL.String())
				_, err := w.Write([]byte(`{
						"content": "package org\n\npolicy_name[\"my_policy\"] { true }",
						"created_at": "2022-08-10T10:47:01.859756-04:00",
	 					"created_by": "737fc204-4048-49fd-9aee-96c97698ed28",
	 					"name": "my_policy"
					}`))
				assert.NoError(t, err)
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
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/policy-bundle/", r.URL.String())
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

				assert.NoError(t, err)
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

			cmd, stdout, _ := makeCMD("", "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}

			assert.JSONEq(t, stdout.String(), tc.ExpectedOutput)
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
			Name:        "gives error when a filter is provided when decisionID is also provided",
			Args:        []string{"logs", "decisionID", "--owner-id", "ownerID", "--branch", "main"},
			ExpectedErr: `filters are not accepted when decision_id is provided`,
		},
		{
			Name:        "gives error when --policy-bundle flag is used but decisionID is not provided",
			Args:        []string{"logs", "--owner-id", "ownerID", "--policy-bundle"},
			ExpectedErr: `decision_id is required when --policy-bundle flag is used`,
		},
		{
			Name: "no filter is set",
			Args: []string{"logs", "--owner-id", "ownerID"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision", r.URL.String())
				_, err := w.Write([]byte("[]"))
				assert.NoError(t, err)
			},
			ExpectedOutput: "[]",
		},
		{
			Name: "all filters are set",
			Args: []string{
				"logs", "--owner-id", "ownerID", "--status", "PASS", "--after", "2022/03/14", "--before", "2022/03/15",
				"--branch", "branchValue", "--project-id", "projectIDValue",
			},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision?after=2022-03-14T00%3A00%3A00Z&before=2022-03-15T00%3A00%3A00Z&branch=branchValue&project_id=projectIDValue&status=PASS", r.URL.String())
				_, err := w.Write([]byte("[]"))
				assert.NoError(t, err)
			},
			ExpectedOutput: "[]",
		},
		{
			Name:        "gets error response",
			Args:        []string{"logs", "--owner-id", "ownerID"},
			ExpectedErr: "failed to get policy decision logs: unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision", r.URL.String())
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NoError(t, err)
			},
		},
		{
			Name: "successfully gets decision logs",
			Args: []string{"logs", "--owner-id", "ownerID"},
			ServerHandler: func() http.HandlerFunc {
				var count int
				return func(w http.ResponseWriter, r *http.Request) {
					defer func() { count++ }()

					assert.Equal(t, "GET", r.Method)

					if count == 0 {
						assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision", r.URL.String())
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
						assert.NoError(t, err)
					} else if count == 1 {
						assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision?offset=1", r.URL.String())
						_, err := w.Write([]byte("[]"))
						assert.NoError(t, err)
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
		{
			Name: "successfully gets a decision log for given decision ID",
			Args: []string{"logs", "--owner-id", "ownerID", "decisionID"},
			ServerHandler: func() http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision/decisionID", r.URL.String())
					_, err := w.Write([]byte("{}"))
					assert.NoError(t, err)
				}
			}(),
			ExpectedOutput: "{}",
		},
		{
			Name: "successfully gets policy-bundle for given decision ID",
			Args: []string{"logs", "--owner-id", "ownerID", "decisionID", "--policy-bundle"},
			ServerHandler: func() http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/api/v1/owner/ownerID/context/config/decision/decisionID/policy-bundle", r.URL.String())
					_, err := w.Write([]byte("{}"))
					assert.NoError(t, err)
				}
			}(),
			ExpectedOutput: "{}",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, _ := makeCMD("", "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}
			assert.JSONEq(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestMakeDecisionCommand(t *testing.T) {
	testcases := []struct {
		Name                  string
		Args                  []string
		ServerHandler         http.HandlerFunc
		CompilerServerHandler http.HandlerFunc
		ExpectedOutput        string
		ExpectedErr           string
	}{
		{
			Name:        "requires flags",
			Args:        []string{"decide"},
			ExpectedErr: `required flag(s) "input" not set`,
		},
		{
			Name: "sends expected request, config compilation is disabled",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "sends expected request, config compilation is enabled (source config has _compiled_ top level key)",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test4/config.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				var payload map[string]interface{}
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.Equal(t, map[string]interface{}{
					"input": `_compiled_:
    test: config
test: config
`,
				}, payload)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			CompilerServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var req config.CompileConfigRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				// dummy compilation here (remove the _compiled_ key in compiled config, as compiled config can't have that at top-level key).
				var yamlResp map[string]any
				err = yaml.Unmarshal([]byte(req.ConfigYaml), &yamlResp)
				require.NoError(t, err)
				delete(yamlResp, "_compiled_")
				compiledConfig, err := yaml.Marshal(yamlResp)
				require.NoError(t, err)

				response := config.ConfigResponse{Valid: true, SourceYaml: req.ConfigYaml, OutputYaml: string(compiledConfig)}

				jsonResponse, err := json.Marshal(response)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write(jsonResponse)
				require.NoError(t, err)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "sends expected request, config compilation is enabled",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				var payload map[string]interface{}
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.Equal(t, map[string]interface{}{
					"input": `_compiled_:
    test: config
test: config
`,
				}, payload)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			CompilerServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var req config.CompileConfigRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				response := config.ConfigResponse{Valid: true, SourceYaml: req.ConfigYaml, OutputYaml: req.ConfigYaml}

				jsonResponse, err := json.Marshal(response)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write(jsonResponse)
				require.NoError(t, err)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "passes when decision status = HARD_FAIL AND --strict is OFF",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"HARD_FAIL"}`)
			},
			ExpectedOutput: `{"status":"HARD_FAIL"}`,
		},
		{
			Name: "fails when decision status = HARD_FAIL AND --strict is ON",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--strict", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"HARD_FAIL"}`)
			},
			ExpectedErr: "policy decision status: HARD_FAIL",
		},
		{
			Name: "passes when decision status = ERROR AND --strict is OFF",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"ERROR", "reason": "some reason"}`)
			},
			ExpectedOutput: `{"status":"ERROR", "reason": "some reason"}`,
		},
		{
			Name: "fails when decision status = ERROR AND --strict is ON",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--strict", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"ERROR", "reason": "some reason"}`)
			},
			ExpectedErr: "policy decision status: ERROR",
		},
		{
			Name: "sends expected request with context",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--context", "custom", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/custom/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "sends expected request with meta",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--context", "custom", "--meta", `{"project_id": "test-project-id","vcs": {"branch": "main"}}`, "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/custom/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n", "metadata": {"project_id": "test-project-id", "vcs":{"branch": "main"}}}`)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "sends expected request with metafile",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--context", "custom", "--metafile", "./testdata/test1/meta.yml", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/custom/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.JSONEq(t, string(payload), `{"input": "test: config\n", "metadata": {"project_id": "test-project-id", "vcs":{"branch": "main"}}}`)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: `{"status":"PASS"}`,
		},
		{
			Name: "fails on unexpected status code",
			Args: []string{"decide", "--input", "./testdata/test1/test.yml", "--owner-id", "test-owner", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
				_, _ = io.WriteString(w, `{"error":"oopsie!"}`)
			},

			ExpectedErr: "failed to make decision: unexpected status-code: 500 - oopsie!",
		},
		{
			Name:        "fails if neither local-policy nor owner-id is provided",
			Args:        []string{"decide", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ExpectedErr: "either [policy_file_or_dir_path] or --owner-id is required",
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml", "--no-compile"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"decide", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to walk root: ",
		},
		{
			Name:        "fails if both meta and metafile are provided",
			Args:        []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test1/test.yml", "--meta", "{}", "--metafile", "somefile", "--no-compile"},
			ExpectedErr: "failed to read metadata: use either --meta or --metafile flag, but not both",
		},
		{
			Name:        "fails if config compilation is enabled, but owner-id isn't provided",
			Args:        []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "--owner-id is required for compiling config (use --no-compile to evaluate policy against source config only)",
		},
		{
			Name:           "successfully performs decision for policy FILE provided locally",
			Args:           []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test0/config.yml", "--no-compile"},
			ExpectedOutput: `{"status": "PASS", "enabled_rules": ["branch_is_main"]}`,
		},
		{
			Name: "successfully performs decision for policy FILE provided locally, when config compilation is enabled",
			Args: []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test0/config.yml", "--owner-id", "test-owner"},
			CompilerServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var req config.CompileConfigRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				response := config.ConfigResponse{Valid: true, SourceYaml: req.ConfigYaml, OutputYaml: req.ConfigYaml}

				jsonResponse, err := json.Marshal(response)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write(jsonResponse)
				require.NoError(t, err)
			},
			ExpectedOutput: `{"status": "PASS", "enabled_rules": ["branch_is_main"]}`,
		},
		{
			Name: "successfully performs decision for policy FILE provided locally, passes when decision = HARD_FAIL and strict = OFF",
			Args: []string{"decide", "./testdata/test2/hard_fail_policy.rego", "--input", "./testdata/test0/config.yml", "--no-compile"},
			ExpectedOutput: `{
	 "status": "HARD_FAIL",
	 "enabled_rules": [
	   "always_hard_fails"
	 ],
	 "hard_failures": [
	   {
	     "rule": "always_hard_fails",
	     "reason": "0 is not equals 1"
	   }
	 ]
	}

`,
		},
		{
			Name:        "successfully performs decision for policy FILE provided locally, fails when decision = HARD_FAIL and strict = ON",
			Args:        []string{"decide", "./testdata/test2/hard_fail_policy.rego", "--input", "./testdata/test0/config.yml", "--strict", "--no-compile"},
			ExpectedErr: "policy decision status: HARD_FAIL",
		},
		{
			Name: "successfully performs decision for policy FILE provided locally, passes when decision = ERROR and strict = OFF",
			Args: []string{"decide", "./testdata/test3/runtime_error_policy.rego", "--input", "./testdata/test0/config.yml", "--no-compile"},
			ExpectedOutput: `{
	 "status": "ERROR",
	 "reason": "./testdata/test3/runtime_error_policy.rego:8: eval_conflict_error: complete rules must not produce multiple outputs"
	}

`,
		},
		{
			Name:        "successfully performs decision for policy FILE provided locally, fails when decision = ERROR and strict = ON",
			Args:        []string{"decide", "./testdata/test3/runtime_error_policy.rego", "--input", "./testdata/test0/config.yml", "--strict", "--no-compile"},
			ExpectedErr: "policy decision status: ERROR",
		},
		{
			Name: "successfully performs decision with meta for policy FILE provided locally",
			Args: []string{
				"decide", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--meta",
				`{"project_id": "test-project-id","vcs": {"branch": "main"}}`, "--input", "./testdata/test0/config.yml", "--no-compile",
			},
			ExpectedOutput: `{"status": "PASS", "enabled_rules": ["enabled"]}`,
		},
		{
			Name: "successfully performs decision with metafile for policy FILE provided locally",
			Args: []string{
				"decide", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--metafile",
				"./testdata/test1/meta.yml", "--input", "./testdata/test0/config.yml", "--no-compile",
			},
			ExpectedOutput: `{"status": "PASS", "enabled_rules": ["enabled"]}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			if tc.CompilerServerHandler == nil {
				tc.CompilerServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			compilerServer := httptest.NewServer(tc.CompilerServerHandler)
			defer compilerServer.Close()

			cmd, stdout, _ := makeCMD(compilerServer.URL, "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}
			assert.JSONEq(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

func TestRawOPAEvaluationCommand(t *testing.T) {
	testcases := []struct {
		Name                  string
		Args                  []string
		ServerHandler         http.HandlerFunc
		CompilerServerHandler http.HandlerFunc
		ExpectedOutput        string
		ExpectedErr           string
	}{
		{
			Name:        "fails if local-policy is not provided",
			Args:        []string{"eval", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ExpectedErr: `accepts 1 arg(s), received 0`,
		},
		{
			Name:        "fails if input is not provided",
			Args:        []string{"eval", "./testdata/test0/policy.rego", "--no-compile"},
			ExpectedErr: `required flag(s) "input" not set`,
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"eval", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml", "--no-compile"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"eval", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to walk root: ",
		},
		{
			Name:        "fails if both meta and metafile are provided",
			Args:        []string{"eval", "./testdata/test0/policy.rego", "--input", "./testdata/test1/test.yml", "--meta", "{}", "--metafile", "somefile", "--no-compile"},
			ExpectedErr: "failed to read metadata: use either --meta or --metafile flag, but not both",
		},
		{
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input and meta",
			Args: []string{
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego",
				"--meta", `{"project_id": "test-project-id","vcs": {"branch": "main"}}`,
				"--input", "./testdata/test0/config.yml", "--no-compile",
			},
			ExpectedOutput: `{
	 "meta": {
	   "vcs": {
			"branch": "main"
		},
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
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input and metafile",
			Args: []string{
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego",
				"--metafile", "./testdata/test1/meta.yml",
				"--input", "./testdata/test0/config.yml", "--no-compile",
			},
			ExpectedOutput: `{
	 "meta": {
	   "vcs": {
			"branch": "main"
		},
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
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input, meta and query",
			Args: []string{
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego",
				"--meta", `{"project_id": "test-project-id","vcs": {"branch": "main"}}`,
				"--input", "./testdata/test0/config.yml",
				"--query", "data.org.enable_rule", "--no-compile",
			},
			ExpectedOutput: `["enabled"]`,
		},
		{
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input, metafile and query",
			Args: []string{
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego",
				"--metafile", "./testdata/test1/meta.yml",
				"--input", "./testdata/test0/config.yml",
				"--query", "data.org.enable_rule", "--no-compile",
			},
			ExpectedOutput: `["enabled"]`,
		},
		{
			Name: "sends expected request, config compilation is disabled",
			Args: []string{"eval", "./testdata/test0/policy.rego", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--no-compile"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				assert.JSONEq(t, string(payload), `{"input": "test: config\n"}`)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			ExpectedOutput: `{"meta": null, "org": {"enable_rule": ["branch_is_main"], "policy_name": ["test"]}}`,
		},
		{
			Name: "sends expected request, config compilation is enabled",
			Args: []string{"eval", "./testdata/test0/policy.rego", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/owner/test-owner/context/config/decision", r.URL.Path)

				var payload map[string]interface{}
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.Equal(t, map[string]interface{}{
					"input": `_compiled_:
    test: config
test: config
`,
				}, payload)

				_, _ = io.WriteString(w, `{"status":"PASS"}`)
			},
			CompilerServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var req config.CompileConfigRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				response := config.ConfigResponse{Valid: true, SourceYaml: req.ConfigYaml, OutputYaml: req.ConfigYaml}

				jsonResponse, err := json.Marshal(response)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write(jsonResponse)
				require.NoError(t, err)
			},
			ExpectedOutput: `{"meta": null, "org": {"enable_rule": ["branch_is_main"], "policy_name": ["test"]}}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			if tc.CompilerServerHandler == nil {
				tc.CompilerServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			compilerServer := httptest.NewServer(tc.CompilerServerHandler)
			defer compilerServer.Close()

			cmd, stdout, _ := makeCMD(compilerServer.URL, "testtoken")

			args := append(tc.Args, "--policy-base-url", svr.URL)

			cmd.SetArgs(args)

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			var actual, expected any
			assert.NoError(t, json.Unmarshal(stdout.Bytes(), &actual))
			assert.NoError(t, json.Unmarshal([]byte(tc.ExpectedOutput), &expected))

			assert.Equal(t, expected, actual)
		})
	}
}

func TestGetSetSettings(t *testing.T) {
	testcases := []struct {
		Name           string
		Args           []string
		ServerHandler  http.HandlerFunc
		ExpectedOutput string
		ExpectedErr    string
	}{
		{
			Name:        "requires owner-id",
			Args:        []string{"settings"},
			ExpectedErr: "required flag(s) \"owner-id\" not set",
		},
		{
			Name:        "gets error response",
			Args:        []string{"settings", "--owner-id", "ownerID", "--context", "someContext"},
			ExpectedErr: "failed to run settings : unexpected status-code: 403 - Forbidden",
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/ownerID/context/someContext/decision/settings", r.URL.String())
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NoError(t, err)
			},
		},
		{
			Name: "successfully fetches settings",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings", r.URL.String())
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NoError(t, err)
			},
			ExpectedOutput: `{"enabled": true}`,
		},
		{
			Name: "successfully sets settings (--enabled)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "PATCH", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{"enabled": true}, body)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NoError(t, err)
			},
			ExpectedOutput: `{"enabled": true}`,
		},
		{
			Name: "successfully sets settings (--enabled=true)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled=true"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "PATCH", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{"enabled": true}, body)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NoError(t, err)
			},
			ExpectedOutput: `{"enabled": true}`,
		},
		{
			Name: "successfully sets settings (--enabled=false)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled=false"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, "PATCH", r.Method)
				assert.Equal(t, "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings", r.URL.String())
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]interface{}{"enabled": false}, body)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": false}`))
				assert.NoError(t, err)
			},
			ExpectedOutput: `{"enabled": false}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ServerHandler == nil {
				tc.ServerHandler = func(w http.ResponseWriter, r *http.Request) {}
			}

			svr := httptest.NewServer(tc.ServerHandler)
			defer svr.Close()

			cmd, stdout, _ := makeCMD("", "testtoken")

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err, tc.ExpectedErr)
				return
			}

			assert.JSONEq(t, stdout.String(), tc.ExpectedOutput)
		})
	}
}

const jsonDeprecationMessage = "Flag --json has been deprecated, use --format=json to print json test results\n"

func TestTestRunner(t *testing.T) {
	cases := []struct {
		Name     string
		Path     string
		Verbose  bool
		Debug    bool
		Run      string
		Json     bool
		Format   string
		Expected func(*testing.T, string)
	}{
		{
			Name: "default options",
			Expected: func(t *testing.T, s string) {
				assert.Contains(t, s, "testdata/test_policies")
				assert.Contains(t, s, "2/2 tests passed")
				assert.NotContains(t, s, "test_feature", "should not have verbose output")
			},
		},
		{
			Name:    "verbose",
			Verbose: true,
			Expected: func(t *testing.T, s string) {
				assert.Contains(t, s, "test_feature")
				assert.Contains(t, s, "test_main")
				assert.Contains(t, s, "2/2 tests passed")
			},
		},
		{
			Name:    "verbose with run",
			Verbose: true,
			Run:     "test_main",
			Expected: func(t *testing.T, s string) {
				assert.Contains(t, s, "test_main")
				assert.NotContains(t, s, "test_feature")
				assert.Contains(t, s, "1/1 tests passed")
			},
		},
		{
			Name:  "debug",
			Debug: true,
			Expected: func(t *testing.T, s string) {
				assert.Contains(t, s, "---- Debug Test Context ----")
			},
		},
		{
			Name: "json",
			Json: true,
			Expected: func(t *testing.T, s string) {
				assert.True(t, strings.HasPrefix(s, jsonDeprecationMessage))
				assert.True(t, s[len(jsonDeprecationMessage)] == '[')
				assert.True(t, s[len(s)-2] == ']')
			},
		},
		{
			Name:   "format:json",
			Format: "json",
			Expected: func(t *testing.T, s string) {
				assert.True(t, s[0] == '[')
				assert.True(t, s[len(s)-2] == ']')
			},
		},
		{
			Name:   "format:junit",
			Format: "junit",
			Expected: func(t *testing.T, s string) {
				assert.Contains(t, s, "<?xml")
			},
		},
		{
			Name:   "format:junit and json flag",
			Format: "junit",
			Json:   true,
			Expected: func(t *testing.T, s string) {
				assert.True(t, strings.HasPrefix(s, jsonDeprecationMessage))
				assert.Contains(t, s, "<?xml")
			},
		},
		{
			Name:    "compile",
			Path:    "./testdata/compile_policies",
			Verbose: false,
			Debug:   false,
			Run:     "",
			Json:    false,
			Format:  "json",
			Expected: func(t *testing.T, s string) {
				require.Contains(t, s, `"Passed": true`)
				require.Contains(t, s, `"Name": "test_compile_policy"`)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			cmd, stdout, _ := makeCMD("", "")

			path := tc.Path
			if path == "" {
				path = "./testdata/test_policies"
			}

			args := []string{"test", path}
			if tc.Verbose {
				args = append(args, "-v")
			}
			if tc.Debug {
				args = append(args, "--debug")
			}
			if tc.Run != "" {
				args = append(args, "--run", tc.Run)
			}
			if tc.Json {
				args = append(args, "--json")
			}
			if tc.Format != "" {
				args = append(args, "--format", tc.Format)
			}

			cmd.SetArgs(args)

			assert.NoError(t, cmd.Execute(), stdout.String())
			tc.Expected(t, stdout.String())
		})
	}
}

func makeCMD(circleHost string, token string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Host:                circleHost,
		Token:               token,
		RestEndpoint:        "/api/v2",
		HTTPClient:          http.DefaultClient,
		IsTelemetryDisabled: true,
	}

	cmd := NewCommand(config, nil)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}

type SafeBuffer struct {
	*bytes.Buffer
	mu *sync.Mutex
}

func (buf SafeBuffer) Write(data []byte) (int, error) {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return buf.Buffer.Write(data)
}

func (buf SafeBuffer) String() string {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return buf.Buffer.String()
}

func makeSafeBuffer() SafeBuffer {
	return SafeBuffer{
		Buffer: &bytes.Buffer{},
		mu:     &sync.Mutex{},
	}
}
