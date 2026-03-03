package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/api/context"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func newContextServer(t testing.TB, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

const (
	ctxToken   = "testtoken"
	ctxName    = "foo-context"
	ctxOrgID   = "bb604b45-b6b0-4b81-ad80-796f15eddf87"
	ctxVCSType = "bitbucket"
	ctxOrgName = "test-org"
)

var ctxOrgSlug = fmt.Sprintf("%s/%s", ctxVCSType, ctxOrgName)

func openAPIHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v2/openapi.json" || r.Method != http.MethodGet {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"paths":{"/context":{}}}`))
}

func gqlGetOrgHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/graphql-unstable" || r.Method != http.MethodPost {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{
	"data": {
		"organization": {
			"id": "%s",
			"name": "%s",
			"vcsType": "%s"
		}
	}
}`, ctxOrgID, ctxOrgName, ctxVCSType)
}

func TestContextInvalidToken(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "list", "github", "foo",
			"--skip-update-check",
			"--token", "",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, testhelpers.ShouldFail(),
		"stdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: please set a token with 'circleci setup'"),
		"stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "https://circleci.com/account/api"),
		"stderr: %s", result.Stderr)
}

func TestContextCreatePermissionError(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"permission issue"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "create", "--org-id", ctxOrgID, "context-name",
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit, got %d", result.ExitCode)
	assert.Equal(t, strings.TrimSpace(result.Stderr), "Error: permission issue")
}

func TestContextListEmpty(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			gqlGetOrgHandlerFunc(w, r)
		case 3:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items":[]}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "list", ctxVCSType, ctxOrgName,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	expected := `+--------------+--------+------+------------+
| ORGANIZATION | ORG ID | NAME | CREATED AT |
+--------------+--------+------+------------+
+--------------+--------+------+------------+
`
	assert.Equal(t, result.Stdout, expected)
}

func TestContextListWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
	}
	body, err := json.Marshal(struct{ Items []context.Context }{contexts})
	assert.NilError(t, err)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			gqlGetOrgHandlerFunc(w, r)
		case 3:
			assert.Equal(t, r.URL.Query().Get("owner-slug"), ctxOrgSlug)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "list", ctxVCSType, ctxOrgName,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	lines := strings.Split(result.Stdout, "\n")
	assert.Assert(t, strings.Contains(lines[1], "ORGANIZATION"), "header line: %s", lines[1])
	assert.Assert(t, strings.Contains(lines[1], "NAME"), "header line: %s", lines[1])
	assert.Equal(t, len(lines), 7)
}

func TestContextListWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
	}
	body, err := json.Marshal(struct{ Items []context.Context }{contexts})
	assert.NilError(t, err)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			gqlGetOrgHandlerFunc(w, r)
		case 3:
			assert.Equal(t, r.URL.Query().Get("owner-id"), ctxOrgID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "list", "--org-id", ctxOrgID,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	lines := strings.Split(result.Stdout, "\n")
	assert.Assert(t, strings.Contains(lines[1], "ID"), "header line: %s", lines[1])
	assert.Assert(t, strings.Contains(lines[1], "NAME"), "header line: %s", lines[1])
	assert.Equal(t, len(lines), 7)
}

func TestContextShowWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	envVars := []context.EnvironmentVariable{
		{Variable: "var-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{Variable: "any-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	envBody, _ := json.Marshal(struct{ Items []context.EnvironmentVariable }{envVars})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-slug"), ctxOrgSlug)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable", contexts[1].ID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(envBody)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "show", ctxVCSType, ctxOrgName, "context-name",
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	lines := strings.Split(result.Stdout, "\n")
	assert.Equal(t, lines[0], "Context: context-name")
	assert.Assert(t, strings.Contains(lines[2], "ENVIRONMENT VARIABLE"), "header: %s", lines[2])
	assert.Equal(t, len(lines), 8)
}

func TestContextShowWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	envVars := []context.EnvironmentVariable{
		{Variable: "var-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{Variable: "any-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	envBody, _ := json.Marshal(struct{ Items []context.EnvironmentVariable }{envVars})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-id"), ctxOrgID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable", contexts[1].ID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(envBody)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "show", "context-name", "--org-id", ctxOrgID,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	lines := strings.Split(result.Stdout, "\n")
	assert.Equal(t, lines[0], "Context: context-name")
	assert.Assert(t, strings.Contains(lines[2], "ENVIRONMENT VARIABLE"), "header: %s", lines[2])
	assert.Equal(t, len(lines), 8)
}

func TestContextStoreSecretWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	envVar := context.EnvironmentVariable{
		Variable:  "env var name",
		ContextID: uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	varBody, _ := json.Marshal(envVar)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-slug"), ctxOrgSlug)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.Method, http.MethodPut)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, envVar.Variable))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(varBody)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "store-secret", ctxVCSType, ctxOrgName, contexts[1].Name, envVar.Variable,
			"--skip-update-check",
			"--integration-testing",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestContextStoreSecretWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	envVar := context.EnvironmentVariable{
		Variable:  "env var name",
		ContextID: uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	varBody, _ := json.Marshal(envVar)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-id"), ctxOrgID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.Method, http.MethodPut)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, envVar.Variable))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(varBody)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "store-secret", "--org-id", ctxOrgID, contexts[1].Name, envVar.Variable,
			"--skip-update-check",
			"--integration-testing",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestContextRemoveSecretWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	varName := "env var name"

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-slug"), ctxOrgSlug)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.Method, http.MethodDelete)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, varName))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Deleted env var"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "remove-secret", ctxVCSType, ctxOrgName, contexts[1].Name, varName,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "Removed secret env var name from context context-name.\n")
}

func TestContextRemoveSecretWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})
	varName := "env var name"

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.URL.Query().Get("owner-id"), ctxOrgID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 3:
			assert.Equal(t, r.Method, http.MethodDelete)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, varName))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Deleted env var"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "remove-secret", "--org-id", ctxOrgID, contexts[1].Name, varName,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "Removed secret env var name from context context-name.\n")
}

func TestContextDeleteWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			gqlGetOrgHandlerFunc(w, r)
		case 3:
			assert.Equal(t, r.URL.Query().Get("owner-slug"), ctxOrgSlug)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 4:
			assert.Equal(t, r.Method, http.MethodDelete)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s", contexts[1].ID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Deleted context"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "delete", "-f", ctxVCSType, ctxOrgName, contexts[1].Name,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "Deleted context context-name.\n")
}

func TestContextDeleteWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	contexts := []context.Context{
		{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
		{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
	}
	ctxBody, _ := json.Marshal(struct{ Items []context.Context }{contexts})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			gqlGetOrgHandlerFunc(w, r)
		case 3:
			assert.Equal(t, r.URL.Query().Get("owner-id"), ctxOrgID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxBody)
		case 4:
			assert.Equal(t, r.Method, http.MethodDelete)
			assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/context/%s", contexts[1].ID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Deleted context"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "delete", "-f", "--org-id", ctxOrgID, contexts[1].Name,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "Deleted context context-name.\n")
}

func TestContextCreateWithOrgID(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	ctxResp, _ := json.Marshal(&context.Context{
		ID:        uuid.NewString(),
		CreatedAt: time.Now(),
		Name:      ctxName,
	})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.Method, http.MethodPost)
			assert.Equal(t, r.URL.Path, "/api/v2/context")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxResp)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "create", "--org-id", ctxOrgID, ctxName,
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
			"--integration-testing",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestContextCreateWithVCSAndOrgName(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	ctxResp, _ := json.Marshal(&context.Context{
		ID:        uuid.NewString(),
		CreatedAt: time.Now(),
		Name:      ctxName,
	})

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			assert.Equal(t, r.Method, http.MethodPost)
			assert.Equal(t, r.URL.Path, "/api/v2/context")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ctxResp)
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "create",
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
			"--integration-testing",
			ctxVCSType,
			ctxOrgName,
			ctxName,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestContextCreateHandlesErrors(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	reqIdx := 0
	server := newContextServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqIdx++
		switch reqIdx {
		case 1:
			openAPIHandlerFunc(w, r)
		case 2:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"ignored error"}`))
		}
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{"context", "create",
			"--skip-update-check",
			"--token", ctxToken,
			"--host", server.URL,
			"--integration-testing",
			ctxVCSType,
			ctxOrgName,
			ctxName,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: ignored error"),
		"stderr: %s", result.Stderr)
}
