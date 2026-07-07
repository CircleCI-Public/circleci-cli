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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// alphaProjectID is the UUID of the "gh/myorg/alpha" project registered in setupProjectFake.
const alphaProjectID = "a0000000-0000-4000-8000-0000000c0001"

func setupSettingsFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake, env := setupProjectFake(t)
	// Settings are keyed by project UUID (the v3 endpoint takes :id, not slug).
	fake.SetProjectSettings(alphaProjectID, map[string]any{
		"enable_ai_error_summarization":          false,
		"enable_auto_cancel_redundant_workflows": false,
		"enable_building_fork_prs":               true,
		"is_build_prs_only":                      false,
		"can_pass_secrets_to_fork_pr_jobs":       false,
		"can_set_github_status":                  true,
		"is_running_disabled":                    false,
		"is_ssh_disabled":                        false,
		"enable_dynamic_config":                  false,
		"is_admin_required_for_writing_settings": false,
		"is_oss":                                 false,
		"pr_only_branch_overrides":               []string{},
		"enable_unversioned_config":              false,
	})
	return fake, env
}

func TestProjectSettingsList(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectSettingsList_JSON(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["enable_building_fork_prs"], true))
	assert.Check(t, cmp.Equal(out["can_set_github_status"], true))
	assert.Check(t, cmp.Equal(out["is_oss"], false))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestProjectSettingsList_NotFound(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "list", "--project", "gh/myorg/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Slug resolution fails with ErrProjectNotFound → exit 5.
	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

func TestProjectSettingsBuildForkedPRs_Get(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "build-forked-pull-requests", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "build-forked-pull-requests: true"))
}

func TestProjectSettingsBuildForkedPRs_Enable(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "build-forked-pull-requests", "--project", "gh/myorg/alpha", "--enable"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "build-forked-pull-requests: true"))
}

func TestProjectSettingsBuildForkedPRs_Disable(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "build-forked-pull-requests", "--project", "gh/myorg/alpha", "--disable"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "build-forked-pull-requests: false"))
}

func TestProjectSettingsBuildForkedPRs_JSON(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "build-forked-pull-requests", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["name"], "build-forked-pull-requests"))
	assert.Check(t, cmp.Equal(out["value"], true))
}

func TestProjectSettingsBuildForkedPRs_ConflictingFlags(t *testing.T) {
	_, env := setupSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "settings", "build-forked-pull-requests", "--project", "gh/myorg/alpha", "--enable", "--disable"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2)
}
