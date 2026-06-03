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
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testOwnerID = "462d67f8-b232-4da4-a7de-0c86dd667d3f"
const testPolicyCtx = "config"
const testDecisionID = "d0000001-0000-4000-8000-000000000001"

// writePolicyDir creates a temporary directory with a single .rego file and returns its path.
func writePolicyDir(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".rego"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- policy push ---

func TestPolicyPush(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "push", dir, "--owner-id", testOwnerID, "--no-prompt"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "created") || strings.Contains(result.Stdout, "updated") || strings.Contains(result.Stdout, "deleted"))

	t.Run("check request", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"policies": map[string]string{
				filepath.Join(dir, "my_policy.rego"): "package main\n",
			},
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/policy-bundle"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(string(body)),
		}, ignoreCommonHeaders))
	})
}

func TestPolicyPush_DryRun(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	// With --no-prompt, it first does a dry-run diff then applies.
	// We test the JSON output matches the diff format.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "push", dir, "--owner-id", testOwnerID, "--no-prompt", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["created"] != nil)
}

// --- policy diff ---

func TestPolicyDiff(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "diff", dir, "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "created") || strings.Contains(result.Stdout, "updated") || strings.Contains(result.Stdout, "deleted"))
}

// --- policy fetch ---

func TestPolicyFetch(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "my_policy"))
}

func TestPolicyFetch_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--owner-id", testOwnerID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["my_policy"] != nil)
}

func TestPolicyFetch_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "my_policy", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "my_policy"))
}

// --- policy logs ---

func TestPolicyLogs(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddDecisionLog(testOwnerID, testPolicyCtx, map[string]any{
		"id":     testDecisionID,
		"status": "PASS",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "logs", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, testDecisionID))
	assert.Check(t, strings.Contains(result.Stdout, "PASS"))
}

func TestPolicyLogs_ByDecisionID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddDecisionLog(testOwnerID, testPolicyCtx, map[string]any{
		"id":     testDecisionID,
		"status": "SOFT_FAIL",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "logs", testDecisionID, "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "SOFT_FAIL"))
}

// --- policy decide ---

func TestPolicyDecide(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetDecisionResult(testOwnerID, testPolicyCtx, map[string]any{
		"status": "PASS",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("version: 2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "decide", "--owner-id", testOwnerID, "--input", configFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "PASS"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/decision"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"input":"version: 2.1\n"}`),
		}, ignoreCommonHeaders))
	})
}

func TestPolicyDecide_Strict(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetDecisionResult(testOwnerID, testPolicyCtx, map[string]any{
		"status": "HARD_FAIL",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("version: 2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "decide", "--owner-id", testOwnerID, "--input", configFile, "--strict"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 7, "expected ExitValidationFail, stderr: %s", result.Stderr) // ExitValidationFail
	assert.Check(t, strings.Contains(result.Stdout, "HARD_FAIL"))
}

// --- policy settings ---

func TestPolicySettingsGet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetPolicyEnabled(testOwnerID, testPolicyCtx, true)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "get", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "true"))
}

func TestPolicySettingsGet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetPolicyEnabled(testOwnerID, testPolicyCtx, true)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "get", "--owner-id", testOwnerID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["enabled"] == true)
}

func TestPolicySettingsSet(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "set", "--owner-id", testOwnerID, "--enabled"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "enabled"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPatch,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/decision/settings"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"enabled":true}`),
		}, ignoreCommonHeaders))
	})
}

func TestPolicySettingsSet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "set", "--owner-id", testOwnerID, "--enabled", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["enabled"] == true)
}

// --- no token ---

func TestPolicy_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}
