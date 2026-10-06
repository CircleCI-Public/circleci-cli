// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/runnerconfig"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testRunnerOrgID = "f22b6566-597d-46d5-ba74-99ef5bb3d85c"
const testOtherRunnerOrgID = "a1a1a1a1-1111-4111-8111-a1a1a1a1a1a1"

func fakeRC(id, resourceClass, desc string) fakes.ResourceClass {
	return fakes.ResourceClass{ID: id, ResourceClass: resourceClass, Description: desc, OrgID: testRunnerOrgID}
}

func fakeToken(id, rc, nickname string) fakes.RunnerToken {
	return fakes.RunnerToken{
		ID:            id,
		ResourceClass: rc,
		Nickname:      nickname,
		CreatedAt:     "2026-01-01T00:00:00Z",
	}
}

func fakeAgent(id, rc, name, version string, busy bool) fakes.RunnerAgent {
	return fakes.RunnerAgent{
		ID:             id,
		ResourceClass:  rc,
		Name:           name,
		Version:        version,
		IsBusy:         busy,
		FirstConnected: "2026-01-01T00:00:00Z",
		LastConnected:  "2026-04-18T12:00:00Z",
	}
}

func setupRunnerFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddOrg(testRunnerOrgID, "gh/my-org", "My Org", "github")

	fake.AddResourceClass(fakeRC("11111111-1111-4111-8111-111111111111", "my-org/linux-runner", "Linux amd64 runner"))
	fake.AddResourceClass(fakeRC("22222222-2222-4222-8222-222222222222", "my-org/arm-runner", "ARM runner"))

	fake.AddRunnerToken("my-org/linux-runner", fakeToken("10000000-0000-4000-8000-000000000001", "my-org/linux-runner", "prod-server-1"))
	fake.AddRunnerToken("my-org/linux-runner", fakeToken("10000000-0000-4000-8000-000000000002", "my-org/linux-runner", "prod-server-2"))

	fake.AddRunnerAgent(fakeAgent("aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa", "my-org/linux-runner", "runner-1", "1.0.0", false))
	fake.AddRunnerAgent(fakeAgent("bbbbbbbb-2222-4222-8222-bbbbbbbbbbbb", "my-org/arm-runner", "runner-2", "1.0.0", false))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- resource-class list ---

func TestRunnerResourceClassList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunnerResourceClassList_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_Namespace(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddResourceClass(fakeRC("44444444-4444-4444-8444-444444444444", "other-org/other-runner", "Another namespace"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_NamespaceJSON(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddResourceClass(fakeRC("44444444-4444-4444-8444-444444444444", "other-org/other-runner", "Another namespace"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	resourceClasses := make([]string, 0, len(out))
	for _, rc := range out {
		resourceClass, _ := rc["resource_class"].(string)
		resourceClasses = append(resourceClasses, resourceClass)
	}
	assert.Check(t, cmp.DeepEqual(resourceClasses, []string{"my-org/linux-runner", "my-org/arm-runner"}))

	query := fake.LastRequest().URL.Query()
	assert.Check(t, cmp.Equal(query.Get("filter[namespace]"), "my-org"))
	assert.Check(t, cmp.Equal(query.Get("filter[org_id]"), ""))
}

func TestRunnerResourceClassList_NamespaceNoMatch(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "empty-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunnerResourceClassList_ResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--resource-class", "my-org/arm-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(fake.LastRequest().URL.Query().Get("filter[resource_class]"), "my-org/arm-runner"))
}

func TestRunnerResourceClassList_FiltersMutuallyExclusive(t *testing.T) {
	_, env := setupRunnerFake(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"NamespaceOrg", []string{"--namespace", "my-org", "--org", "gh/my-org"}},
		{"NamespaceResourceClass", []string{"--namespace", "my-org", "--resource-class", "my-org/arm-runner"}},
		{"OrgResourceClass", []string{"--org", "gh/my-org", "--resource-class", "my-org/arm-runner"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    append([]string{"runner", "resource-class", "list"}, tc.args...),
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 1))
			assert.Check(t, golden.String(result.Stderr, "TestRunnerResourceClassList_FiltersMutuallyExclusive_"+tc.name+".stderr.txt"))
		})
	}
}

func TestRunnerResourceClassList_NamespaceIgnoresAgents(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddResourceClass(fakeRC("33333333-3333-4333-8333-333333333333", "my-org/idle-runner", "No runners attached"))
	// Two agents on one class must not double it up in the listing.
	fake.AddRunnerAgent(fakeAgent("cccccccc-3333-4333-8333-cccccccccccc", "my-org/linux-runner", "runner-3", "1.0.0", false))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)

	resourceClasses := make([]string, 0, len(out))
	for _, rc := range out {
		resourceClass, _ := rc["resource_class"].(string)
		resourceClasses = append(resourceClasses, resourceClass)
		assert.Check(t, rc["id"] != "", "resource class %q came back without an id", resourceClass)
	}
	assert.Check(t, cmp.DeepEqual(resourceClasses, []string{
		"my-org/linux-runner", "my-org/arm-runner", "my-org/idle-runner",
	}))

	assert.Check(t, cmp.Equal(fake.LastRequest().URL.Path, "/api/v3/runner/resource-classes"))
}

func TestRunnerResourceClassList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["resource_class"], "my-org/linux-runner"))
	assert.Check(t, cmp.Equal(out[0]["description"], "Linux amd64 runner"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerResourceClassList_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

// A filter[org_id] naming an org the caller is not a member of answers 404, and
// the CLI cannot tell that apart from an org that does not exist, this is intentional.
func TestRunnerResourceClassList_OrgNotAccessible(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.HideRunnerOrg(testRunnerOrgID)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_Org(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddOrg(testRunnerOrgID, "gh/my-org", "My Org", "github")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_OrgID(t *testing.T) {
	// A bare UUID is used directly; no org lookup is performed.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- resource-class create ---

func TestRunnerResourceClassCreate(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/resource-classes"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"description":"New runner","resource_class":"my-org/new-runner"},"references":{"org":{"id":"f22b6566-597d-46d5-ba74-99ef5bb3d85c"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassCreate_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassCreate_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner", "--json",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["resource_class"], "my-org/new-runner"))
	assert.Check(t, cmp.Equal(out["description"], "New runner"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	// The Runner Terms notice is a compliance requirement, not command output,
	// so --json must not suppress it.
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassCreate_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner", "--json",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerResourceClassCreate_GenerateToken(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// The token is data, so it belongs on stdout only.
	leaked := strings.Contains(result.Stderr, "fake-runner-token-value")
	assert.Check(t, !leaked, "token value leaked into stderr: %s", result.Stderr)

	// Three calls: resolving --org's slug to a UUID, the resource class create,
	// then the token create. LastRequest() would only see the last of these.
	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 3))

		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/resource-classes"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"description":"","resource_class":"my-org/new-runner"},"references":{"org":{"id":"f22b6566-597d-46d5-ba74-99ef5bb3d85c"}}}}`),
		}, ignoreCommonHeaders))

		assert.Check(t, cmp.DeepEqual(reqs[2], httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/tokens"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"nickname":"default"},"references":{"resource_class":{"id":"f212fd52-c7f7-57dc-b25e-5f29aada41f1"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassCreate_GenerateToken_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A token wrapped across lines by the markdown renderer would not be copyable.
func TestRunnerResourceClassCreate_GenerateToken_LongTokenNotWrapped(t *testing.T) {
	longToken := strings.Repeat("a1b2c3d4", 15) // 120 chars, wider than the 80-column test pty

	fake, env := setupRunnerFake(t)
	fake.SetRunnerTokenCreateResponse(http.StatusCreated, map[string]any{
		"data": map[string]any{
			"id": "10000000-0000-4000-8000-000000000009",
			"attributes": map[string]any{
				"nickname":   "default",
				"created_at": "2026-01-01T00:00:00Z",
				"token":      longToken,
			},
		},
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stdout, longToken))
}

func TestRunnerResourceClassCreate_GenerateToken_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token", "--json",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"id":             "f212fd52-c7f7-57dc-b25e-5f29aada41f1",
		"resource_class": "my-org/new-runner",
		"description":    "",
		"token_id":       "3c49fcdb-6513-5438-8b97-d69d46e0d0d2",
		"token":          "fake-runner-token-value",
	}))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

// Without the flag no token exists, so both token fields are absent rather than empty.
func TestRunnerResourceClassCreate_GenerateToken_NoTokenFields(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--json",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	_, hasToken := out["token"]
	assert.Check(t, !hasToken, "expected no token field: %s", result.Stdout)
	_, hasTokenID := out["token_id"]
	assert.Check(t, !hasTokenID, "expected no token_id field: %s", result.Stdout)
}

func TestRunnerResourceClassCreate_GenerateToken_TokenFails(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetRunnerTokenCreateResponse(http.StatusInternalServerError, nil)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	// A half-done create must not look like a success, so stdout stays empty.
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A 401 on the token call must still exit 3, not be flattened to the generic 4.
func TestRunnerResourceClassCreate_GenerateToken_TokenUnauthorized(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetRunnerTokenCreateResponse(http.StatusUnauthorized, nil)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// Exiting 0 here would leave the user owning a credential they can never see.
func TestRunnerResourceClassCreate_GenerateToken_TokenValueMissing(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetRunnerTokenCreateResponse(http.StatusCreated, map[string]any{
		"data": map[string]any{
			"id": "10000000-0000-4000-8000-000000000009",
			"attributes": map[string]any{
				"nickname":   "default",
				"created_at": "2026-01-01T00:00:00Z",
			},
		},
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/new-runner", "--generate-token",
			"--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// The resource class already exists under a different org (via setupRunnerFake's
// seeded classes owned by testRunnerOrgID); creating under a different org for
// the same namespace should 403.
func TestRunnerResourceClassCreate_OrgMismatch(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddOrg(testOtherRunnerOrgID, "gh/other-org", "Other Org", "github")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "create", "my-org/mismatched-runner",
			"--org", "gh/other-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- resource-class delete ---

func TestRunnerResourceClassDelete_NoForce(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 6))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A token created between the lookup and the forced delete makes runner-admin answer
// 409. Its V3 error body must not leak into the message as raw JSON.
func TestRunnerResourceClassDelete_TokensInUse(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetResourceClassDeleteResponse(http.StatusConflict, map[string]any{
		"error": map[string]any{
			"id":    "trace-409",
			"title": "Resource class my-org/linux-runner still has tokens in use.",
		},
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/linux-runner", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Contains(result.Stderr, "error id: trace-409"))
	assert.Check(t, !strings.Contains(result.Stderr, `{"error"`), "raw error body leaked: %s", result.Stderr)
}

func TestRunnerResourceClassDelete_Force(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/linux-runner", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[resource_class]"), "my-org/linux-runner"))

		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodDelete,
			URL: url.URL{
				Path:     "/api/v3/runner/resource-classes/11111111-1111-4111-8111-111111111111",
				RawQuery: "force=true",
			},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassDelete_Force_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "delete", "my-org/linux-runner",
			"--force", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunnerResourceClassDelete_NotFound_JSON is the reason --json is worth having
// on a command that returns nothing on success: the runner API answers a
// permission denial with 404 as well, so ExitNotFound alone cannot say which
// happened. The structured code can.
func TestRunnerResourceClassDelete_NotFound_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "resource-class", "delete", "my-org/nope",
			"--force", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassDelete_Force_RemovesTokens(t *testing.T) {
	_, env := setupRunnerFake(t)

	del := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/linux-runner", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Assert(t, cmp.Equal(del.ExitCode, 0))

	list := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(list.ExitCode, clierrors.ExitNotFound))
}

func TestRunnerResourceClassDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/nonexistent", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- resource-class update ---

func TestRunnerResourceClassUpdate(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "my-org/linux-runner", "--description", "Updated description"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		reqs := fake.AllRequests()
		// First: resolve the resource class to its UUID.
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[resource_class]"), "my-org/linux-runner"))

		// Second: update via POST /resource-classes/{id}/update.
		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/resource-classes/11111111-1111-4111-8111-111111111111/update"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"description":"Updated description"}`),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassUpdate_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "my-org/linux-runner", "--description", "New desc", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["resource_class"], "my-org/linux-runner"))
	assert.Check(t, cmp.Equal(out["description"], "New desc"))
	assert.Check(t, out["id"] != "", "expected non-empty id")

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "my-org/nonexistent", "--description", "New desc"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_MissingArg(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "--description", "New desc"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_NoDescription(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// my-org/linux-runner already has tokens (see setupRunnerFake). Confirming
// interactively, without --force, must still delete them: the prompt already
// warns that tokens and runner connections will be removed, so the delete
// request always carries force=true once confirmed.
func TestRunnerResourceClassDelete_HasTokens(t *testing.T) {
	fake, env := setupRunnerFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("confirms deletion despite existing tokens", func(t *testing.T) {
		_, err := console.ExpectString("[y/N]")
		assert.NilError(t, err)
		_, err = console.Send("y")
		assert.NilError(t, err)
		_, err = console.ExpectString("Deleted resource class my-org/linux-runner")
		assert.NilError(t, err)
	}))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodDelete))
		assert.Check(t, cmp.Equal(reqs[1].URL.RawQuery, "force=true"))
	})
}

// --- token list ---

// Without --resource-class every resource class in the org is enumerated.
func TestRunnerTokenList_EnumeratesEveryResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddResourceClass(fakeRC("33333333-3333-4333-8333-333333333333", "my-org/idle-runner", "No runners attached"))
	fake.AddRunnerToken("my-org/idle-runner", fakeToken("10000000-0000-4000-8000-000000000009", "my-org/idle-runner", "idle-token"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--org", testRunnerOrgID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)

	ids := make([]string, 0, len(out))
	for _, tok := range out {
		id, _ := tok["id"].(string)
		ids = append(ids, id)
	}
	assert.Check(t, cmp.Contains(ids, "10000000-0000-4000-8000-000000000009"))
	assert.Check(t, cmp.Len(ids, 3))
}

func TestRunnerTokenList_NoOrgDetermined(t *testing.T) {
	// Outside any git checkout with no --org and no --resource-class, the CLI
	// must fail with a structured bad-arguments error.
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenList_Org(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddOrg(testRunnerOrgID, "gh/my-org", "My Org", "github")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A namespace/name goes straight to the tokens endpoint as filter[resource_class], with no
// resource class lookup first. The fake rejects a request without a filter, but this pins
// the exact query and the request count so that renaming the filter or reintroducing
// the lookup fails here rather than in production.
func TestRunnerTokenList_FiltersByResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddRunnerToken("my-org/arm-runner", fakeToken("10000000-0000-4000-8000-00000000000a", "my-org/arm-runner", "arm-server"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	t.Run("check the only request is the tokens request", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 1))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/tokens"))
		assert.Check(t, cmp.DeepEqual(reqs[0].URL.Query(), url.Values{
			"filter[resource_class]": {"my-org/linux-runner"},
		}))
	})

	t.Run("check only the filtered resource class is listed", func(t *testing.T) {
		var out []map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))

		ids := make([]string, 0, len(out))
		for _, tok := range out {
			id, _ := tok["id"].(string)
			ids = append(ids, id)
		}
		assert.Check(t, cmp.DeepEqual(ids, []string{"10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002"}))
	})
}

// --resource-class also accepts a resource class's UUID, which goes straight to the
// tokens endpoint as filter[resource_class_id], with no resource class lookup first.
func TestRunnerTokenList_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "11111111-1111-4111-8111-111111111111"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 1))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/tokens"))
		assert.Check(t, cmp.DeepEqual(reqs[0].URL.Query(), url.Values{
			"filter[resource_class_id]": {"11111111-1111-4111-8111-111111111111"},
		}))
	})
}

func TestRunnerTokenList_ByID_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "11111111-1111-4111-8111-111111111111", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, []map[string]any{
		{"id": "10000000-0000-4000-8000-000000000001", "resource_class": "my-org/linux-runner", "resource_class_id": "11111111-1111-4111-8111-111111111111", "nickname": "prod-server-1", "created_at": "2026-01-01T00:00:00Z"},
		{"id": "10000000-0000-4000-8000-000000000002", "resource_class": "my-org/linux-runner", "resource_class_id": "11111111-1111-4111-8111-111111111111", "nickname": "prod-server-2", "created_at": "2026-01-01T00:00:00Z"},
	}))
}

// With no tokens, the message names the resource class by namespace/name, not by the ID
// the user passed.
func TestRunnerTokenList_ByID_Empty(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "22222222-2222-4222-8222-222222222222"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenList_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "99999999-9999-4999-8999-999999999999"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// Without --resource-class the CLI lists tokens once per resource class in the
// org, and every one of those requests must carry that resource class's ID.
func TestRunnerTokenList_EnumerationFiltersEachResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--org", testRunnerOrgID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	var filters []string
	for _, req := range fake.AllRequests() {
		if req.URL.Path != "/api/v3/runner/tokens" {
			continue
		}
		q := req.URL.Query()
		assert.Check(t, cmp.Len(q, 1), "unexpected query on tokens request: %s", req.URL.RawQuery)
		filters = append(filters, q.Get("filter[resource_class_id]"))
	}
	slices.Sort(filters)
	assert.Check(t, cmp.DeepEqual(filters, []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	}))
}

func TestRunnerTokenList_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], "10000000-0000-4000-8000-000000000001"))
	assert.Check(t, cmp.Equal(out[0]["nickname"], "prod-server-1"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerTokenList_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerTokenList_JQ(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json", "--jq", ".[0].nickname"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), "prod-server-1"))
}

func TestRunnerTokenList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddResourceClass(fakeRC("11111111-1111-4111-8111-111111111111", "my-org/linux-runner", "Linux amd64 runner"))
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- token create ---

func TestRunnerTokenCreate(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", "my-org/linux-runner", "--nickname", "my-server"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))

		// First: resolve the resource class to its UUID.
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[resource_class]"), "my-org/linux-runner"))

		// Second: create token via V3 using the resource class UUID.
		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/tokens"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"nickname":"my-server"},"references":{"resource_class":{"id":"11111111-1111-4111-8111-111111111111"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerTokenCreate_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", "my-org/linux-runner", "--nickname", "my-server"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenCreate_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["resource_class"], "my-org/linux-runner"))
	assert.Check(t, cmp.Equal(out["token"], "fake-runner-token-value"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerTokenCreate_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

// --- token delete ---

func TestRunnerTokenDelete(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "10000000-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v3/runner/tokens/10000000-0000-4000-8000-000000000001"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerTokenDelete_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "--json", "10000000-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenDelete_NotFound_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "--json", "10000000-0000-4000-8000-0000000000ff"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenDelete_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "10000000-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenDelete_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "10000000-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 6))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "10000000-0000-4000-8000-0000000000ff"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// runner-admin answers a non-UUID token id with a 400, so the CLI rejects it as a bad
// argument before prompting or calling the API.
func TestRunnerTokenDelete_InvalidID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete", "--force", "not-a-token-id"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		assert.Check(t, cmp.Len(fake.AllRequests(), 0))
	})
}

// --- instance list ---

func TestRunnerInstanceList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_FiltersMutuallyExclusive(t *testing.T) {
	_, env := setupRunnerFake(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"NamespaceOrg", []string{"--namespace", "my-org", "--org", "gh/my-org"}},
		{"NamespaceResourceClass", []string{"--namespace", "my-org", "--resource-class", "my-org/arm-runner"}},
		{"OrgResourceClass", []string{"--org", "gh/my-org", "--resource-class", "my-org/arm-runner"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    append([]string{"runner", "instance", "list"}, tc.args...),
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 1))
			assert.Check(t, golden.String(result.Stderr, "TestRunnerInstanceList_FiltersMutuallyExclusive_"+tc.name+".stderr.txt"))
		})
	}
}

func TestRunnerInstanceList_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_ResourceClass(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_ResourceClass_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerInstanceList_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunnerInstanceList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// The empty message names a scope the user asked for and stays bare for an inferred org, so each
// of the three scopes takes a different branch.
func TestRunnerInstanceList_Empty_ResourceClass(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_Empty_Org(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_Org(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddOrg(testRunnerOrgID, "gh/my-org", "My Org", "github")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--org", "gh/my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_OrgID(t *testing.T) {
	// A bare UUID is used directly; no org lookup is performed.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// The agents API serves 20 per page, so more than one page has to be followed to the end.
func TestRunnerInstanceList_Paginated(t *testing.T) {
	fake, env := setupRunnerFake(t)
	for i := 0; i < 60; i++ {
		fake.AddRunnerAgent(fakeAgent(
			fmt.Sprintf("dddddddd-0000-4000-8000-%012d", i),
			"my-org/linux-runner", fmt.Sprintf("paged-%02d", i), "1.0.0", false))
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--org", testRunnerOrgID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	// The two fixture agents plus the 60 seeded here.
	assert.Check(t, cmp.Len(out, 62))
}

// Cobra reports a flag-group violation as a flag error, which exits 1 here like any other.
func TestRunnerInstanceList_ConflictingScopes(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--namespace", "my-org", "--resource-class", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInstanceList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- runner config ---

// runnerConfigArgs is the common invocation. An explicit --name keeps output
// deterministic; the hostname default is covered by TestRunnerConfig.
func runnerConfigArgs(extra ...string) []string {
	return append([]string{"runner", "config", "my-org/linux-runner", "--name", "prod-server-1"}, extra...)
}

func TestRunnerConfig(t *testing.T) {
	// A bare invocation must not prompt for --product: RunCLI is
	// non-interactive, so it keeps the documented machine default and names the
	// runner after the host.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	host, err := os.Hostname()
	assert.NilError(t, err)

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	goldenTemplate(t, result.Stdout, t.Name()+".yaml.tmpl", map[string]string{
		"Name": runnerconfig.SanitizeName(host),
	})
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_Name(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs(),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_WorkingDirectory(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs("--working-directory", "/srv/circleci/workdir"),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_ProductContainer(t *testing.T) {
	// Container runner reads no agent config file, so the output is Helm values
	// for the container-agent chart.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--product", "container"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_ProductMachineOrchestrator(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--product", "machineOrchestrator"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunnerConfig_ProductProvisionerAlias guards backward compatibility:
// --product provisioner, the name before the rename to machine runner
// orchestrator, must keep working and produce identical output.
func TestRunnerConfig_ProductProvisionerAlias(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--product", "provisioner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, "TestRunnerConfig_ProductMachineOrchestrator.yaml"))
	assert.Check(t, golden.String(result.Stderr, "TestRunnerConfig_ProductMachineOrchestrator.stderr.txt"))
}

func TestRunnerConfig_ProductInvalid(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--product", "kubernetes"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_MachineFlagOnHelmProduct(t *testing.T) {
	// --name has no home in a Helm values file, so it is rejected rather than
	// silently dropped from the output.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs("--product", "container"),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_InvalidName(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--name", "bad/name"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_InvalidResourceClass(t *testing.T) {
	// --token skips the API call, so a malformed resource class would otherwise
	// reach the generated file and silently never claim a task.
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "linux-runner", "--token", "my-existing-token-value", "--name", "prod-server-1"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_Nickname(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs("--nickname", "prod-server-1"),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_ExistingToken(t *testing.T) {
	// --token skips the API call entirely; no fake server needed.
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs("--token", "my-existing-token-value"),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerConfig_OutputFile(t *testing.T) {
	_, env := setupRunnerFake(t)
	dir := t.TempDir()
	outPath := dir + "/nested/circleci-runner-config.yaml"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    runnerConfigArgs("--output", outPath),
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	contents, err := os.ReadFile(outPath)
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(contents), t.Name()+".yaml"))

	// The file holds a runner token, so it must not be group or world readable.
	// Windows does not honour Unix permission bits, so the check is skipped there.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(outPath)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Mode().Perm(), os.FileMode(0o600)))
	}
}

func TestRunnerConfig_Interactive_DefaultsToMachine(t *testing.T) {
	// On a terminal the product is confirmed rather than assumed. Machine is
	// preselected, so a bare Enter keeps the documented default.
	env := testenv.New(t)
	env.Token = testToken

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--token", "my-existing-token-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("prompts for the product", func(t *testing.T) {
		_, err := console.ExpectString("Runner product")
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("emits machine runner config", func(t *testing.T) {
		_, err := console.ExpectString("Machine runner 3 agent configuration")
		assert.NilError(t, err)
		_, err = console.ExpectString("auth_token: my-existing-token-value")
		assert.NilError(t, err)
	}))
}

func TestRunnerConfig_Interactive_SelectsContainer(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--token", "my-existing-token-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("selects container", func(t *testing.T) {
		_, err := console.ExpectString("Runner product")
		assert.NilError(t, err)
		// Down once: machine -> container.
		_, err = console.Send("\x1b[B\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("emits container runner Helm values", func(t *testing.T) {
		_, err := console.ExpectString("Helm values for container runner")
		assert.NilError(t, err)
		_, err = console.ExpectString("resourceClasses")
		assert.NilError(t, err)
	}))
}

func TestRunnerConfig_NoArgs(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- open ---
//
// runner open never calls the CircleCI API, so these tests don't spin up a
// fake server. They reuse the browser-shadowing helpers from
// login_agent_test.go (installFakeBrowserOpener, withPathPrefix) to assert on
// what URL the CLI hands the browser opener without ever opening one.

func TestRunnerOpen_DefaultOrgFromGitRemote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so it cannot be shadowed on PATH")
	}

	dir := t.TempDir()
	initGitRepoWithRemote(t, dir, "https://github.com/my-org/my-repo.git")

	openedPath := filepath.Join(t.TempDir(), "opened.txt")
	binDir := installFakeBrowserOpener(t, openedPath)

	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open"},
		Env:     withPathPrefix(env.Environ(), binDir),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	opened, err := os.ReadFile(openedPath)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(strings.TrimSpace(string(opened)), "https://app.circleci.com/runners/gh/my-org/inventory"))
}

func TestRunnerOpen_OrgFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so it cannot be shadowed on PATH")
	}

	openedPath := filepath.Join(t.TempDir(), "opened.txt")
	binDir := installFakeBrowserOpener(t, openedPath)

	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open", "--org", "gh/other-org"},
		Env:     withPathPrefix(env.Environ(), binDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	opened, err := os.ReadFile(openedPath)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(strings.TrimSpace(string(opened)), "https://app.circleci.com/runners/gh/other-org/inventory"))
}

func TestRunnerOpen_CustomHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so it cannot be shadowed on PATH")
	}

	openedPath := filepath.Join(t.TempDir(), "opened.txt")
	binDir := installFakeBrowserOpener(t, openedPath)

	env := testenv.New(t)
	env.CircleCIURL = "https://circleci.example.com"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open", "--org", "gh/myorg"},
		Env:     withPathPrefix(env.Environ(), binDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	opened, err := os.ReadFile(openedPath)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(strings.TrimSpace(string(opened)), "https://app.circleci.example.com/runners/gh/myorg/inventory"))
}

// TestRunnerOpen_NoBrowserFound covers the fallback for a headless machine:
// with no browser opener anywhere on PATH, the CLI must print the URL
// instead of failing.
func TestRunnerOpen_NoBrowserFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so PATH has no effect on whether one is found")
	}

	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open", "--org", "gh/myorg"},
		Env:     withPathReplaced(env.Environ(), t.TempDir()),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunnerOpen_NoGitRemoteNoOrg covers running outside any git checkout
// with no --org override: the CLI can't infer an organization and must fail
// with a structured, bad-arguments error rather than a bare Go error.
func TestRunnerOpen_NoGitRemoteNoOrg(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunnerOpen_InvalidOrgSlug covers ONP-3562's reported bug: an --org
// value with no <vcs>/<org> separator used to bubble up a bare Go error
// ("invalid org slug: ...") with exit code 1 instead of a structured
// bad-arguments error.
func TestRunnerOpen_InvalidOrgSlug(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "open", "--org", "myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// withPathReplaced returns environ with PATH replaced (not prefixed) by dir,
// so no real browser opener on the host machine can be found.
func withPathReplaced(environ []string, dir string) []string {
	out := make([]string, 0, len(environ))
	found := false
	for _, e := range environ {
		if strings.HasPrefix(e, "PATH=") {
			out = append(out, "PATH="+dir)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

// --- resource class by ID ---
//
// Every command that takes a resource class also accepts its UUID. The agents
// and tokens endpoints only filter by namespace/name, so the CLI resolves an ID
// with GET /runner/resource-classes/{id} first.

const (
	testLinuxRCID   = "11111111-1111-4111-8111-111111111111" // my-org/linux-runner in setupRunnerFake
	testUnknownRCID = "99999999-9999-4999-8999-999999999999"
)

func TestRunnerInstanceList_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", testLinuxRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))

		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/agents"))
		assert.Check(t, cmp.Equal(reqs[1].URL.Query().Get("filter[resource_class]"), "my-org/linux-runner"))
	})
}

func TestRunnerInstanceList_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", testUnknownRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A value that is neither namespace/name nor a UUID is rejected before any request.
func TestRunnerInstanceList_MalformedResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "instance", "list", "--resource-class", "linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0))
}

func TestRunnerTokenList_MalformedResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0))
}

func TestRunnerTokenCreate_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", testLinuxRCID, "--nickname", "my-server", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))

		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodPost))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/tokens"))
		assert.Check(t, cmp.DeepEqual(reqs[1].Body,
			new(`{"data":{"attributes":{"nickname":"my-server"},"references":{"resource_class":{"id":"`+testLinuxRCID+`"}}}}`)))
	})

	t.Run("check output names the resource class", func(t *testing.T) {
		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.Equal(out["resource_class"], "my-org/linux-runner"))
	})
}

func TestRunnerTokenCreate_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", testUnknownRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// The generated config names the resource class by namespace/name, so an ID
// is resolved even though --token means no token is created.
func TestRunnerConfig_ByID_ExistingToken(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"runner", "config", testLinuxRCID,
			"--product", "container", "--token", "my-existing-token-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 1))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))
	})
}

func TestRunnerConfig_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", testLinuxRCID, "--name", "prod-server-1"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))
		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodPost))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/tokens"))
	})
}

func TestRunnerConfig_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", testUnknownRCID, "--name", "prod-server-1"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--resource-class", testLinuxRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))

		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[1].URL.Query().Get("filter[resource_class]"), "my-org/linux-runner"))
	})
}

func TestRunnerResourceClassList_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--resource-class", testUnknownRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_ByID_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", testLinuxRCID, "--description", "New desc", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"id":             testLinuxRCID,
		"resource_class": "my-org/linux-runner",
		"description":    "New desc",
	}))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_ByID(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", testLinuxRCID, "--description", "Updated"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))
		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodPost))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID+"/update"))
	})
}

func TestRunnerResourceClassUpdate_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", testUnknownRCID, "--description", "Updated"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassUpdate_MalformedResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "update", "linux-runner", "--description", "Updated"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0))
}

// Given an ID, the JSON output still reports the resource class by namespace/name.
func TestRunnerResourceClassDelete_ByID_JSON(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", testLinuxRCID, "--force", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))
		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodDelete))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))
	})
}

// Without --force a non-interactive run refuses, and the message names the
// resource class by namespace/name rather than the ID it was given.
func TestRunnerResourceClassDelete_ByID_NoForce(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", testLinuxRCID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitCancelled))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassDelete_ByID_NotFound(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", testUnknownRCID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassDelete_ByID_Force(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", testLinuxRCID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check requests", func(t *testing.T) {
		reqs := fake.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/"+testLinuxRCID))

		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodDelete,
			URL: url.URL{
				Path:     "/api/v3/runner/resource-classes/" + testLinuxRCID,
				RawQuery: "force=true",
			},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassDelete_ByID_NotFound_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", testUnknownRCID, "--force", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassDelete_MalformedResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete", "linux-runner", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0))
}

// --- permissions ---

// An org member who can view runners but lacks the admin role gets runner-admin's
// 403 on every write, which must read as "admin required", not "not found".
func TestRunner_NotAdmin(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "resource-class update", args: []string{"runner", "resource-class", "update", "my-org/linux-runner", "--description", "New desc"}},
		{name: "resource-class delete", args: []string{"runner", "resource-class", "delete", "my-org/linux-runner", "--force"}},
		{name: "token create", args: []string{"runner", "token", "create", "my-org/linux-runner"}},
		{name: "token delete", args: []string{"runner", "token", "delete", "10000000-0000-4000-8000-000000000001", "--force"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake, env := setupRunnerFake(t)
			fake.SetRunnerViewOnly()

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    tc.args,
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
			assert.Check(t, cmp.Contains(result.Stderr, "requires the admin role"))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}
}

// Creating in an org as a member without the admin role names both causes the
// server's 403
func TestRunnerResourceClassCreate_NotAdmin(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetRunnerViewOnly()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, cmp.Contains(result.Stderr, "You do not have the admin role"))
	// Naming the namespace as a possible cause would confirm it is in use by another org.
	mentionsNamespace := strings.Contains(result.Stderr, "namespace")
	assert.Check(t, !mentionsNamespace, "error discloses namespace ownership: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A caller outside the org gets a 404 on create, since runner-admin will not
// reveal whether the org exists.
func TestRunnerResourceClassCreate_OrgNotAccessible(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.HideRunnerOrg(testRunnerOrgID)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, cmp.Contains(result.Stderr, "your token is not a member of it"))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerTokenList_OrgNotAccessible(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.HideRunnerOrg(testRunnerOrgID)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A 403 that is not the admin-role denial, such as the token limit, keeps the
// server's own explanation rather than being reported as a missing role.
func TestRunnerTokenCreate_LimitReached(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetRunnerTokenCreateResponse(http.StatusForbidden, map[string]any{
		"error": map[string]any{"title": "Resource class token limit reached."},
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, cmp.Contains(result.Stderr, "Resource class token limit reached."))
	assert.Check(t, !strings.Contains(result.Stderr, "admin role"), "a limit 403 must not be reported as a missing role")
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
