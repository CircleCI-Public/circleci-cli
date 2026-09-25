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

func fakeRC(id, slug, desc string) fakes.ResourceClass {
	return fakes.ResourceClass{ID: id, Slug: slug, Description: desc, OrgID: testRunnerOrgID}
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

	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-1", "my-org/linux-runner", "prod-server-1"))
	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-2", "my-org/linux-runner", "prod-server-2"))

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

// --namespace has no V3 equivalent (the API only supports filter[org_id] and
// filter[slug], the latter being a full namespace/name) so the flag is now a
// clear, immediate error pointing at --org, rather than silently guessing.
func TestRunnerResourceClassList_Namespace(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
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

	slugs := make([]string, 0, len(out))
	for _, rc := range out {
		slug, _ := rc["resource_class"].(string)
		slugs = append(slugs, slug)
		assert.Check(t, rc["id"] != "", "resource class %q came back without an id", slug)
	}
	assert.Check(t, cmp.DeepEqual(slugs, []string{
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

// A filter[org_id] naming an org the token cannot view answers 403, which maps
// to the same runnerNotEnabledErr as a 404 would.
func TestRunnerResourceClassList_OrgForbidden(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.ForbidRunnerOrg(testRunnerOrgID)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--org", testRunnerOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
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
			URL:    url.URL{Path: "/api/v3/runner/token"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"nickname":"default","resource_class":"my-org/new-runner"}`),
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
		"id":             "tok-id-9",
		"resource_class": "my-org/new-runner",
		"nickname":       "default",
		"created_at":     "2026-01-01T00:00:00Z",
		"token":          longToken,
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
		"id":             "rc-my-org/new-runner",
		"resource_class": "my-org/new-runner",
		"description":    "",
		"token_id":       "tok-my-org/new-runner",
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
		"id":             "tok-id-9",
		"resource_class": "my-org/new-runner",
		"nickname":       "default",
		"created_at":     "2026-01-01T00:00:00Z",
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
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[slug]"), "my-org/linux-runner"))

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
		// First: resolve resource class by slug to get UUID.
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[slug]"), "my-org/linux-runner"))

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
	fake.AddRunnerToken("my-org/idle-runner", fakeToken("tok-id-9", "my-org/idle-runner", "idle-token"))

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
	assert.Check(t, cmp.Contains(ids, "tok-id-9"))
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

// The V3 tokens endpoint is scoped by filter[resource_class], the namespace/name. The fake
// rejects a request without it, but this pins the exact query so that renaming
// the filter (it was once filter[resource_class_id]) or sending the UUID instead
// of the namespace/name fails here rather than in production.
func TestRunnerTokenList_FiltersByResourceClass(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.AddRunnerToken("my-org/arm-runner", fakeToken("tok-id-arm", "my-org/arm-runner", "arm-server"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	t.Run("check tokens request", func(t *testing.T) {
		var tokenReqs []httprecorder.Request
		for _, req := range fake.AllRequests() {
			if req.URL.Path == "/api/v3/runner/tokens" {
				tokenReqs = append(tokenReqs, req)
			}
		}
		assert.Assert(t, cmp.Len(tokenReqs, 1))

		assert.Check(t, cmp.Equal(tokenReqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.DeepEqual(tokenReqs[0].URL.Query(), url.Values{
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
		assert.Check(t, cmp.DeepEqual(ids, []string{"tok-id-1", "tok-id-2"}))
	})
}

// --resource-class also accepts a resource class's UUID. The tokens endpoint
// only filters by namespace/name, so the CLI resolves the ID first.
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
		assert.Assert(t, cmp.Len(reqs, 2))

		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes/11111111-1111-4111-8111-111111111111"))

		assert.Check(t, cmp.Equal(reqs[1].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[1].URL.Path, "/api/v3/runner/tokens"))
		assert.Check(t, cmp.DeepEqual(reqs[1].URL.Query(), url.Values{
			"filter[resource_class]": {"my-org/linux-runner"},
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
		{"id": "tok-id-1", "resource_class": "my-org/linux-runner", "nickname": "prod-server-1", "created_at": "2026-01-01T00:00:00Z"},
		{"id": "tok-id-2", "resource_class": "my-org/linux-runner", "nickname": "prod-server-2", "created_at": "2026-01-01T00:00:00Z"},
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
// org, and every one of those requests must carry that resource class's namespace/name.
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
		filters = append(filters, q.Get("filter[resource_class]"))
	}
	slices.Sort(filters)
	assert.Check(t, cmp.DeepEqual(filters, []string{"my-org/arm-runner", "my-org/linux-runner"}))
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
	assert.Check(t, cmp.Equal(out[0]["id"], "tok-id-1"))
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

		// First: resolve resource class by slug to get UUID.
		assert.Check(t, cmp.Equal(reqs[0].Method, http.MethodGet))
		assert.Check(t, cmp.Equal(reqs[0].URL.Path, "/api/v3/runner/resource-classes"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[slug]"), "my-org/linux-runner"))

		// Second: create token via V3 using the resource class UUID.
		assert.Check(t, cmp.DeepEqual(reqs[1], httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/tokens"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"nickname":"my-server","references":{"resource_class":{"id":"11111111-1111-4111-8111-111111111111"}}}`),
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
		Args:    []string{"runner", "token", "delete", "--force", "tok-id-1"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v3/runner/tokens/tok-id-1"},
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
		Args:    []string{"runner", "token", "delete", "--force", "--json", "tok-id-1"},
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
		Args:    []string{"runner", "token", "delete", "--force", "--json", "tok-id-nope"},
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
		Args:    []string{"runner", "token", "delete", "--force", "tok-id-1"},
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
		Args:    []string{"runner", "token", "delete", "tok-id-1"},
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
		Args:    []string{"runner", "token", "delete", "--force", "nonexistent-token-id"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
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

func TestRunnerConfig_ProductProvisioner(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--product", "provisioner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
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
