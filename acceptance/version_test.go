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

	"github.com/CircleCI-Public/circleci-cli/internal/installmethod"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func TestVersionText(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// Format: `circleci <version> (<commit>)\n`. The acceptance binary is
	// built without ldflags, so the version is the default "dev".
	assert.Check(t, strings.HasPrefix(result.Stdout, "circleci dev ("),
		"unexpected version output: %q", result.Stdout)
	assert.Check(t, strings.HasSuffix(strings.TrimSpace(result.Stdout), ")"),
		"version output should end with a closing paren: %q", result.Stdout)
}

func TestVersionJSON(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var info struct {
		Version  string `json:"version"`
		Commit   string `json:"commit"`
		Modified bool   `json:"modified"`
	}
	err := json.Unmarshal([]byte(result.Stdout), &info)
	assert.NilError(t, err, "stdout was not valid JSON: %q", result.Stdout)

	assert.Equal(t, info.Version, "dev", "expected default version 'dev', got %q", info.Version)
	assert.Check(t, info.Commit != "", "commit field should be set")

	// A dev build can't be compared with a release, so both freshness fields are
	// present and null rather than missing.
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Contains(out, "latest"))
	assert.Check(t, cmp.Contains(out, "outdated"))
	assert.Check(t, cmp.DeepEqual(freshness(out), map[string]any{"latest": nil, "outdated": nil}))

	// The test binary runs from a temp dir, which is no known install method.
	assert.Check(t, cmp.Contains(out, "upgrade_command"))
	assert.Check(t, cmp.Nil(out["upgrade_command"]))
}

func TestVersionJSON_UpgradeCommand(t *testing.T) {
	env := testenv.New(t)
	env.Extra[installmethod.ForceEnv] = "homebrew"

	out := runVersionJSON(t, env)
	assert.Check(t, cmp.Equal(out["upgrade_command"], "brew upgrade circleci"))
}

// runVersionJSON runs `circleci version --json` and decodes its output.
func runVersionJSON(t *testing.T, env *testenv.TestEnv) map[string]any {
	t.Helper()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out), "stdout was not valid JSON: %q", result.Stdout)
	return out
}

// freshness picks out the fields that say whether the build is current, so a
// test can compare them whole without pinning the build's commit hash.
func freshness(out map[string]any) map[string]any {
	return map[string]any{"latest": out["latest"], "outdated": out["outdated"]}
}

func TestVersionJSON_ReportsNewerRelease(t *testing.T) {
	_, env := setupUpdateFake(t) // latest 1.3.0, this build treated as 1.2.0

	out := runVersionJSON(t, env)
	assert.Check(t, cmp.DeepEqual(freshness(out), map[string]any{"latest": "1.3.0", "outdated": true}))
}

func TestVersionJSON_UpToDate(t *testing.T) {
	_, env := setupUpdateFake(t)
	env.Extra["__CIRCLE_UPDATE_FORCE"] = "1.3.0"

	out := runVersionJSON(t, env)
	assert.Check(t, cmp.DeepEqual(freshness(out), map[string]any{"latest": "1.3.0", "outdated": false}))
}

func TestVersionJSON_UnknownWhenUpdateCheckOff(t *testing.T) {
	_, env := setupUpdateFake(t)
	env.Extra["CIRCLE_NO_UPDATE_CHECK"] = "1"

	out := runVersionJSON(t, env)
	assert.Check(t, cmp.DeepEqual(freshness(out), map[string]any{"latest": nil, "outdated": nil}))
}

func TestVersionJSON_UnknownWhenReleaseUnavailable(t *testing.T) {
	fake, env := setupUpdateFake(t)
	fake.SetReleaseStatus(http.StatusServiceUnavailable)

	out := runVersionJSON(t, env)
	assert.Check(t, cmp.DeepEqual(freshness(out), map[string]any{"latest": nil, "outdated": nil}))
}

func TestVersionFlagMatchesSubcommand(t *testing.T) {
	// Cobra exposes --version as a built-in flag and the subcommand prints
	// the same version string. Both should report the same version so the
	// two surfaces stay consistent.
	env := testenv.New(t)

	flagResult := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Equal(t, flagResult.ExitCode, 0, "stderr: %s", flagResult.Stderr)

	subResult := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Equal(t, subResult.ExitCode, 0, "stderr: %s", subResult.Stderr)

	// Both outputs contain "circleci dev"; the subcommand additionally
	// includes the commit hash in parens.
	assert.Check(t, cmp.Contains(flagResult.Stdout, "dev"),
		"--version output: %q", flagResult.Stdout)
	assert.Check(t, cmp.Contains(subResult.Stdout, "dev"),
		"version subcommand output: %q", subResult.Stdout)
}
