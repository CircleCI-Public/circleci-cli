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
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

//go:embed testdata
var testdata embed.FS

func testdataContent(t *testing.T, filePath string) string {
	data, err := testdata.ReadFile(path.Join(".", "testdata", filePath))
	assert.NilError(t, err)
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
		assert.Equal(t, r.Method, "POST")
		assert.Equal(t, r.URL.String(), expectedURLs[requestCount])
		assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.DeepEqual(t, body, map[string]interface{}{
			"policies": map[string]interface{}{
				filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
				filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
			},
		})
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("{}"))
		requestCount++
	}))
	defer svr.Close()

	config := &settings.Config{Token: "testtoken", HTTPClient: http.DefaultClient}
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
		assert.NilError(t, cmd.Execute())
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	expectedMessage := "The following changes are going to be made: {}\n\nDo you wish to continue? (y/N) "
	assert.Equal(t, buffer.String(), expectedMessage)

	_, err := pw.Write([]byte("y\n"))
	assert.NilError(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, buffer.String()[len(expectedMessage):], "\nPolicy Bundle Pushed Successfully\n\ndiff: {}\n")

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
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/context/custom/policy-bundle")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"policies": map[string]interface{}{},
				})
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
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/context/custom/policy-bundle")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"policies": map[string]interface{}{
						filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
						filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
					},
				})

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

			cmd, stdout, stderr := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL, "--no-prompt"))

			err := cmd.Execute()
			if tc.ExpectedErr != "" {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, stdout.String(), tc.ExpectedStdOut)
			assert.Equal(t, stderr.String(), tc.ExpectedStdErr)
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
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/context/custom/policy-bundle?dry=true")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"policies": map[string]interface{}{},
				})
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
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/test-org/context/custom/policy-bundle?dry=true")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{
					"policies": map[string]interface{}{
						filepath.Join("testdata", "test0", "policy.rego"):                                      testdataContent(t, "test0/policy.rego"),
						filepath.Join("testdata", "test0", "subdir", "meta-policy-subdir", "meta-policy.rego"): testdataContent(t, "test0/subdir/meta-policy-subdir/meta-policy.rego"),
					},
				})

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

			cmd, stdout, stderr := makeCMD()

			cmd.SetArgs(append(tc.Args, "--policy-base-url", svr.URL))

			err := cmd.Execute()
			if tc.ExpectedErr != "" {
				assert.ErrorContains(t, err, tc.ExpectedErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, stdout.String(), tc.ExpectedStdOut)
			assert.Equal(t, stderr.String(), tc.ExpectedStdErr)
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
		{
			Name: "successfully gets a decision log for given decision ID",
			Args: []string{"logs", "--owner-id", "ownerID", "decisionID"},
			ServerHandler: func() http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision/decisionID")
					_, err := w.Write([]byte("{}"))
					assert.NilError(t, err)
				}
			}(),
			ExpectedOutput: "{}\n",
		},
		{
			Name: "successfully gets policy-bundle for given decision ID",
			Args: []string{"logs", "--owner-id", "ownerID", "decisionID", "--policy-bundle"},
			ServerHandler: func() http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "GET")
					assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/config/decision/decisionID/policy-bundle")
					_, err := w.Write([]byte("{}"))
					assert.NilError(t, err)
				}
			}(),
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
			Name: "passes when decision status = HARD_FAIL AND --strict is OFF",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/config/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"HARD_FAIL"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"HARD_FAIL\"\n}\n",
		},
		{
			Name: "fails when decision status = HARD_FAIL AND --strict is ON",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--strict"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/config/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"HARD_FAIL"}`)
			},
			ExpectedErr: "policy decision status: HARD_FAIL",
		},
		{
			Name: "passes when decision status = ERROR AND --strict is OFF",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/config/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"ERROR", "reason": "some reason"}`)
			},
			ExpectedOutput: "{\n  \"status\": \"ERROR\",\n  \"reason\": \"some reason\"\n}\n",
		},
		{
			Name: "fails when decision status = ERROR AND --strict is ON",
			Args: []string{"decide", "--owner-id", "test-owner", "--input", "./testdata/test1/test.yml", "--strict"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, "/api/v1/owner/test-owner/context/config/decision")

				var payload map[string]interface{}
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&payload))

				assert.DeepEqual(t, payload, map[string]interface{}{
					"input": "test: config\n",
				})

				_, _ = io.WriteString(w, `{"status":"ERROR", "reason": "some reason"}`)
			},
			ExpectedErr: "policy decision status: ERROR",
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
			ExpectedErr: "either [policy_file_or_dir_path] or --owner-id is required",
		},
		{
			Name:        "fails if both local-policy and owner-id are provided",
			Args:        []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test1/test.yml", "--owner-id", "test-owner"},
			ExpectedErr: "either [policy_file_or_dir_path] or --owner-id is required",
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"decide", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to walk root: ",
		},
		{
			Name: "successfully performs decision for policy FILE provided locally",
			Args: []string{"decide", "./testdata/test0/policy.rego", "--input", "./testdata/test0/config.yml"},
			ExpectedOutput: `{
  "status": "PASS",
  "enabled_rules": [
    "branch_is_main"
  ]
}
`,
		},
		{
			Name: "successfully performs decision for policy FILE provided locally, passes when decision = HARD_FAIL and strict = OFF",
			Args: []string{"decide", "./testdata/test2/hard_fail_policy.rego", "--input", "./testdata/test0/config.yml"},
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
			Args:        []string{"decide", "./testdata/test2/hard_fail_policy.rego", "--input", "./testdata/test0/config.yml", "--strict"},
			ExpectedErr: "policy decision status: HARD_FAIL",
		},
		{
			Name: "successfully performs decision for policy FILE provided locally, passes when decision = ERROR and strict = OFF",
			Args: []string{"decide", "./testdata/test3/runtime_error_policy.rego", "--input", "./testdata/test0/config.yml"},
			ExpectedOutput: `{
  "status": "ERROR",
  "reason": "./testdata/test3/runtime_error_policy.rego:8: eval_conflict_error: complete rules must not produce multiple outputs"
}
`,
		},
		{
			Name:        "successfully performs decision for policy FILE provided locally, fails when decision = ERROR and strict = ON",
			Args:        []string{"decide", "./testdata/test3/runtime_error_policy.rego", "--input", "./testdata/test0/config.yml", "--strict"},
			ExpectedErr: "policy decision status: ERROR",
		},
		{
			Name: "successfully performs decision with metadata for policy FILE provided locally",
			Args: []string{
				"decide", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--metafile",
				"./testdata/test1/meta.yml", "--input", "./testdata/test0/config.yml",
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
			Name:        "fails if local-policy is not provided",
			Args:        []string{"eval", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: `accepts 1 arg(s), received 0`,
		},
		{
			Name:        "fails if input is not provided",
			Args:        []string{"eval", "./testdata/test0/policy.rego"},
			ExpectedErr: `required flag(s) "input" not set`,
		},
		{
			Name:        "fails for input file not found",
			Args:        []string{"eval", "./testdata/test0/policy.rego", "--input", "./testdata/no_such_file.yml"},
			ExpectedErr: "failed to read input file: open ./testdata/no_such_file.yml: ",
		},
		{
			Name:        "fails for policy FILE/DIRECTORY not found",
			Args:        []string{"eval", "./testdata/no_such_file.rego", "--input", "./testdata/test1/test.yml"},
			ExpectedErr: "failed to make decision: failed to load policy files: failed to walk root: ",
		},
		{
			Name: "successfully performs raw opa evaluation for policy FILE provided locally, input and metadata",
			Args: []string{
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--metafile",
				"./testdata/test1/meta.yml", "--input", "./testdata/test0/config.yml",
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
				"eval", "./testdata/test0/subdir/meta-policy-subdir/meta-policy.rego", "--metafile",
				"./testdata/test1/meta.yml", "--input", "./testdata/test0/config.yml", "--query", "data.org.enable_rule",
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
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/ownerID/context/someContext/decision/settings")
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte(`{"error": "Forbidden"}`))
				assert.NilError(t, err)
			},
		},
		{
			Name: "successfully fetches settings",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "enabled": true
}
`,
		},
		{
			Name: "successfully sets settings (--enabled)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "PATCH")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{"enabled": true})
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "enabled": true
}
`,
		},
		{
			Name: "successfully sets settings (--enabled=true)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled=true"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "PATCH")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{"enabled": true})
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": true}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "enabled": true
}
`,
		},
		{
			Name: "successfully sets settings (--enabled=false)",
			Args: []string{"settings", "--owner-id", "462d67f8-b232-4da4-a7de-0c86dd667d3f", "--enabled=false"},
			ServerHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				assert.Equal(t, r.Method, "PATCH")
				assert.Equal(t, r.URL.String(), "/api/v1/owner/462d67f8-b232-4da4-a7de-0c86dd667d3f/context/config/decision/settings")
				assert.NilError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.DeepEqual(t, body, map[string]interface{}{"enabled": false})
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"enabled": false}`))
				assert.NilError(t, err)
			},
			ExpectedOutput: `{
  "enabled": false
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

const jsonDeprecationMessage = "Flag --json has been deprecated, use --format=json to print json test results\n"

func TestTestRunner(t *testing.T) {
	cases := []struct {
		Name     string
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
				assert.Check(t, strings.Contains(s, "testdata/test_policies"))
				assert.Check(t, strings.Contains(s, "2/2 tests passed"))
				assert.Check(t, !strings.Contains(s, "test_feature"), "should not have verbose output")
			},
		},
		{
			Name:    "verbose",
			Verbose: true,
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.Contains(s, "test_feature"))
				assert.Check(t, strings.Contains(s, "test_main"))
				assert.Check(t, strings.Contains(s, "2/2 tests passed"))
			},
		},
		{
			Name:    "verbose with run",
			Verbose: true,
			Run:     "test_main",
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.Contains(s, "test_main"))
				assert.Check(t, !strings.Contains(s, "test_feature"))
				assert.Check(t, strings.Contains(s, "1/1 tests passed"))
			},
		},
		{
			Name:  "debug",
			Debug: true,
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.Contains(s, "---- Debug Test Context ----"))
			},
		},
		{
			Name: "json",
			Json: true,
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.HasPrefix(s, jsonDeprecationMessage))
				assert.Check(t, s[len(jsonDeprecationMessage)] == '[')
				assert.Check(t, s[len(s)-2] == ']')
			},
		},
		{
			Name:   "format:json",
			Format: "json",
			Expected: func(t *testing.T, s string) {
				assert.Check(t, s[0] == '[')
				assert.Check(t, s[len(s)-2] == ']')
			},
		},
		{
			Name:   "format:junit",
			Format: "junit",
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.Contains(s, "<?xml"))
			},
		},
		{
			Name:   "format:junit and json flag",
			Format: "junit",
			Json:   true,
			Expected: func(t *testing.T, s string) {
				assert.Check(t, strings.HasPrefix(s, jsonDeprecationMessage))
				assert.Check(t, strings.Contains(s, "<?xml"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			cmd, stdout, _ := makeCMD()

			args := []string{"test", "./testdata/test_policies"}
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

			assert.NilError(t, cmd.Execute(), stdout.String())
			tc.Expected(t, stdout.String())
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
