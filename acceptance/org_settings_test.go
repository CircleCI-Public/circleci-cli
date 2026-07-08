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

// orgSettingsOrgID is the UUID of the "gh/myorg" org registered in setupProjectFake.
const orgSettingsOrgID = "a0000000-0000-4000-8000-0000000c0002"

// orgSettingsOrgSlug is the slug for the org used in org settings tests.
const orgSettingsOrgSlug = "gh/myorg"

func setupOrgSettingsFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake, env := setupProjectFake(t)
	// Register the org so GET /api/v2/organization/gh/myorg resolves the UUID.
	fake.AddOrg(orgSettingsOrgID, orgSettingsOrgSlug, "myorg", "github")
	fake.SetOrgSettings(orgSettingsOrgID, map[string]any{
		"is_runner_terms_of_service_accepted":      false,
		"enable_ai_error_summarization":            true,
		"enable_ai_agents":                         false,
		"enable_unversioned_config":                false,
		"enable_certified_public_orbs":             true,
		"enable_chunk_ip_ranges":                   false,
		"enable_minor_ai_features":                 false,
		"enable_private_orbs":                      false,
		"enable_uncertified_public_orbs":           false,
		"is_bitbucket_workspace_member_org_member": false,
		"is_user_checkout_keys_disabled":           false,
		"is_running_disabled":                      false,
		"enable_image_brownouts":                   false,
		"is_context_group_restriction_required":    false,
		"enable_resource_class_brownouts":          false,
	})
	return fake, env
}

func TestOrgSettingsList(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "list", "--org", orgSettingsOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestOrgSettingsList_JSON(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "list", "--org", orgSettingsOrgSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["enable_ai_error_summarization"], true))
	assert.Check(t, cmp.Equal(out["enable_certified_public_orbs"], true))
	assert.Check(t, cmp.Equal(out["is_running_disabled"], false))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestOrgSettingsList_NotFound(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "list", "--org", "gh/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0, "expected non-zero exit code, got 0")
}

func TestOrgSettingsAIErrorSummarization_Get(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "get", "ai-error-summarization", "--org", orgSettingsOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "ai-error-summarization: true"))
}

func TestOrgSettingsAIErrorSummarization_SetTrue(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "set", "ai-error-summarization", "true", "--org", orgSettingsOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "true"))
}

func TestOrgSettingsAIErrorSummarization_SetFalse(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "set", "ai-error-summarization", "false", "--org", orgSettingsOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "false"))
}

func TestOrgSettingsAIErrorSummarization_JSON(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "get", "ai-error-summarization", "--org", orgSettingsOrgSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["name"], "ai-error-summarization"))
	assert.Check(t, cmp.Equal(out["value"], true))
}

func TestOrgSettingsAIErrorSummarization_InvalidValue(t *testing.T) {
	_, env := setupOrgSettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "setting", "set", "ai-error-summarization", "yes", "--org", orgSettingsOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2)
}
