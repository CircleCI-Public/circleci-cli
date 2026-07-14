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
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testRunnerOrgID = "f22b6566-597d-46d5-ba74-99ef5bb3d85c"

func fakeRC(id, slug, desc string) map[string]any {
	return map[string]any{
		"id":             id,
		"resource_class": slug,
		"description":    desc,
	}
}

func fakeToken(id, rc, nickname string) map[string]any {
	return map[string]any{
		"id":             id,
		"resource_class": rc,
		"nickname":       nickname,
		"created_at":     "2026-01-01T00:00:00Z",
	}
}

func fakeInstance(rc, hostname, name, version string) map[string]any {
	return map[string]any{
		"resource_class":     rc,
		"hostname":           hostname,
		"name":               name,
		"version":            version,
		"ip":                 "10.0.0.1",
		"first_connected_at": "2026-01-01T00:00:00Z",
		"last_connected_at":  "2026-04-18T12:00:00Z",
		"last_used_at":       "2026-04-18T11:00:00Z",
	}
}

func setupRunnerFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddResourceClass(fakeRC("rc-id-1", "my-org/linux-runner", "Linux amd64 runner"))
	fake.AddResourceClass(fakeRC("rc-id-2", "my-org/arm-runner", "ARM runner"))

	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-1", "my-org/linux-runner", "prod-server-1"))
	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-2", "my-org/linux-runner", "prod-server-2"))

	fake.AddRunnerInstance(fakeInstance("my-org/linux-runner", "host-1.example.com", "runner-1", "1.0.0"))
	fake.AddRunnerInstance(fakeInstance("my-org/arm-runner", "arm-host.example.com", "runner-2", "1.0.0"))

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
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org"},
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
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerResourceClassList_Namespace(t *testing.T) {
	_, env := setupRunnerFake(t)

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

func TestRunnerResourceClassList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org", "--json"},
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
		Args:    []string{"runner", "resource-class", "list", "--namespace", "my-org", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
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
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/resource"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"description":"New runner","resource_class":"my-org/new-runner"}`),
		}, ignoreCommonHeaders))
	})
}

func TestRunnerResourceClassCreate_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner"},
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
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner", "--json"},
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
}

func TestRunnerResourceClassCreate_JSON_Color(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
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

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v3/runner/resource/my-org/linux-runner"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
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

// --- token list ---

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

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/runner/token"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"nickname":"my-server","resource_class":"my-org/linux-runner"}`),
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
			URL:    url.URL{Path: "/api/v3/runner/token/tok-id-1"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
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

func TestRunnerConfig(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".yaml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

}

func TestRunnerConfig_Nickname(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--nickname", "prod-server-1"},
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
		Args:    []string{"runner", "config", "my-org/linux-runner", "--token", "my-existing-token-value"},
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
	outPath := dir + "/launch-agent-config.yaml"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "config", "my-org/linux-runner", "--output", outPath},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	contents, err := os.ReadFile(outPath)
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(contents), t.Name()+".yaml"))
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

// verify the fake server returns a proper 202 for rerun (used by TestRunnerResourceClassDelete_Force indirectly)
var _ = http.StatusAccepted
