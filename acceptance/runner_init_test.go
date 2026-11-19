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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func runRunnerInit(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"runner", "init", "--org", "gh/my-org"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestRunnerInit_Debian(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "debian", "--force")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, fake.RunnerTermsAccepted(testRunnerOrgID), "terms must be stored as accepted")

	leaked := strings.Contains(result.Stderr, "fake-runner-token-value")
	assert.Check(t, !leaked, "token value leaked into stderr: %s", result.Stderr)
}

func TestRunnerInit_Kubernetes(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "kubernetes", "--force")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunnerInit_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "macos", "--force", "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""), "--json must suppress human output")

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	steps, _ := out["steps"].([]any)
	delete(out, "steps")
	delete(out, "token_id")
	delete(out, "resource_class_id")
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"org_id":                 testRunnerOrgID,
		"terms_already_accepted": false,
		"resource_class":         "my-org/new-runner",
		"token":                  "fake-runner-token-value",
		"platform":               "macos",
		"job_config":             out["job_config"],
		"docs_url":               "https://circleci.com/docs/runner/install-machine-runner-3-on-macos/",
	}))
	assert.Check(t, cmp.Contains(out["job_config"], "resource_class: my-org/new-runner"))
	assert.Check(t, len(steps) >= 2, "expected install steps in JSON")
}

func TestRunnerInit_TermsAlreadyAccepted(t *testing.T) {
	fake, env := setupRunnerFake(t)
	fake.SetOrgSettings(testRunnerOrgID, map[string]any{"is_runner_terms_of_service_accepted": true})
	// A write would fail, proving init does not touch accepted terms.
	fake.SetUpdateOrgSettingsStatus(http.StatusInternalServerError)

	result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "docker", "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["terms_already_accepted"], true))
}

func TestRunnerInit_TermsNeedConfirmation(t *testing.T) {
	fake, env := setupRunnerFake(t)

	result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "debian")

	assert.Check(t, cmp.Equal(result.ExitCode, 6), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "needs confirmation"))
	assert.Check(t, !fake.RunnerTermsAccepted(testRunnerOrgID), "terms must not be accepted without confirmation")
	for _, r := range fake.AllRequests() {
		assert.Check(t, !strings.HasPrefix(r.URL.Path, "/api/v3/runner/resource-classes") || r.Method != http.MethodPost,
			"no resource class may be created when the terms were not accepted")
	}
}

func TestRunnerInit_MissingInput(t *testing.T) {
	t.Run("platform", func(t *testing.T) {
		fake, env := setupRunnerFake(t)

		result := runRunnerInit(t, env, "my-org/new-runner", "--force")

		assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, "--platform"))
		assert.Check(t, cmp.Len(fake.AllRequests(), 0), "bad input must fail before any API call")
	})

	t.Run("resource class", func(t *testing.T) {
		fake, env := setupRunnerFake(t)

		result := runRunnerInit(t, env, "--platform", "debian", "--force")

		assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, "namespace/name"))
		assert.Check(t, cmp.Len(fake.AllRequests(), 0), "bad input must fail before any API call")
	})

	t.Run("invalid platform", func(t *testing.T) {
		fake, env := setupRunnerFake(t)

		result := runRunnerInit(t, env, "my-org/new-runner", "--platform", "amiga", "--force")

		assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, `"amiga"`))
		assert.Check(t, cmp.Len(fake.AllRequests(), 0), "bad input must fail before any API call")
	})

	t.Run("malformed resource class", func(t *testing.T) {
		fake, env := setupRunnerFake(t)

		result := runRunnerInit(t, env, "justaname", "--platform", "debian", "--force")

		assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, "namespace/name"))
		assert.Check(t, cmp.Len(fake.AllRequests(), 0), "bad input must fail before any API call")
	})
}
