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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func TestInit_HappyPath_WithFakeCollaborators(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	env.Extra["CIRCLECI_INIT_FAKE_SIGNUP"] = "success"
	env.Extra["CIRCLECI_INIT_FAKE_PROJECT_CREATOR"] = "success"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Git repository detected"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "circleci init will:"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "This will run in your selected repo"), "stderr: %s", result.Stderr)
	assertInOrder(t, result.Stderr,
		"[1/4] Scanning repository",
		"[2/4] Running tests in Docker",
		"[3/4] Generating config",
		"[4/4] Sign up for CircleCI",
		"Your first pipeline is running:",
	)
	assert.Check(t, strings.Contains(result.Stderr, "Tests passed"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "https://app.circleci.com/pipelines/github/example/initfixture/1"), "stderr: %s", result.Stderr)
	_, err := os.Stat(filepath.Join(workDir, ".circleci", "config.yml"))
	assert.NilError(t, err)
}

func TestInit_ExistingConfigSkipsGeneration(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	env.Extra["CIRCLECI_INIT_FAKE_CONFIGGEN"] = "fail"
	env.Extra["CIRCLECI_INIT_FAKE_SIGNUP"] = "success"
	env.Extra["CIRCLECI_INIT_FAKE_PROJECT_CREATOR"] = "success"
	workDir := initGitRepo(t)

	configDir := filepath.Join(workDir, ".circleci")
	assert.NilError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.yml")
	original := []byte("# existing config\nversion: 2.1\n")
	assert.NilError(t, os.WriteFile(configPath, original, 0o644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Using existing config"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "[4/4] Sign up for CircleCI"), "stderr: %s", result.Stderr)
	got, err := os.ReadFile(configPath)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, original)
}

func TestInit_TestsFail_PrintsAgentPrompt(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "fail"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 7, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Agent-ready prompt"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "go test ./..."), "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "[3/4] Generating config"), "stderr: %s", result.Stderr)
}

func TestInit_RunnerError_DockerMissing(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "unavailable"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Docker is required"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Install and start Docker"), "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "[3/4] Generating config"), "stderr: %s", result.Stderr)
}

func TestInit_ConfigGenerationFails(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	env.Extra["CIRCLECI_INIT_FAKE_CONFIGGEN"] = "fail"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "fake config generation failed"), "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "[4/4] Sign up for CircleCI"), "stderr: %s", result.Stderr)
}

func TestInit_SignupCancelled(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	env.Extra["CIRCLECI_INIT_FAKE_SIGNUP"] = "cancel"
	env.Extra["CIRCLECI_INIT_FAKE_PROJECT_CREATOR"] = "success"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Signup cancelled"), "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "Your first pipeline is running"), "stderr: %s", result.Stderr)
}

func TestInit_ProjectCreationFails(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	env.Extra["CIRCLECI_INIT_FAKE_SIGNUP"] = "success"
	env.Extra["CIRCLECI_INIT_FAKE_PROJECT_CREATOR"] = "fake project creation failed"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "fake project creation failed"), "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "Your first pipeline is running"), "stderr: %s", result.Stderr)
}

func TestInit_NotInGitRepo_ExitsBadArgs(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "valid git repository"), "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "cd <path>"), "stderr: %s", result.Stderr)
}

func TestInit_RejectsExtraArgs(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_INIT_FAKE_RUNNER"] = "pass"
	workDir := initGitRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init", "extra"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	workDir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module example.com/initfixture\n\ngo 1.22\n"), 0o644))
	cmd := exec.Command("git", "init") //#nosec:G204 // fixed git invocation in a test temp dir
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(out))

	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/example/initfixture.git") //#nosec:G204 // fixed git invocation in a test temp dir
	cmd.Dir = workDir
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, string(out))
	return workDir
}

func assertInOrder(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	last := -1
	for _, needle := range needles {
		idx := strings.Index(haystack, needle)
		assert.Assert(t, idx >= 0, "missing %q in stderr:\n%s", needle, haystack)
		assert.Assert(t, idx > last, "%q appeared out of order in stderr:\n%s", needle, haystack)
		last = idx
	}
}
