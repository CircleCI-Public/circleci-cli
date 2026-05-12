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
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

func TestInit_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	assert.NilError(t, err, "git init failed: %s", out)

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	for _, step := range []string{
		"Git repository detected",
		"This will run in your selected repo: " + filepath.Base(dir),
		"[1/3] Scanning repository",
		"[2/3] Running tests in Docker",
		"[3/3] Generating config",
		"sign up for CircleCI",
		"https://circleci.com/signup",
		"Runs on git events and webhooks",
		"Pass/fail checks on PRs",
		"Optimize speed with test parallelism",
		"Deploy to staging or prod",
	} {
		assert.Check(t, cmp.Contains(result.Stderr, step),
			"expected %q in stderr, got: %s", step, result.Stderr)
	}
	assert.Check(t, cmp.Equal(result.Stdout, ""), "expected init progress on stderr only")
}

func TestInit_InNestedGitDirectoryUsesRepoRootName(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	assert.NilError(t, err, "git init failed: %s", out)

	nested := filepath.Join(dir, "src", "app")
	assert.NilError(t, os.MkdirAll(nested, 0o755))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: nested,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "This will run in your selected repo: "+filepath.Base(dir)),
		"expected root repo name in stderr, got: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "This will run in your selected repo: app"),
		"expected nested directory name not to be used as repo name, got: %s", result.Stderr)
}

func TestInit_Help(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init", "--help"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	for _, phase := range []string{
		"Scan your repo",
		"Docker container",
		"Generate a config",
		"Sign up for CircleCI",
	} {
		assert.Check(t, cmp.Contains(result.Stdout, phase),
			"--help is missing onboarding phase %q; stdout: %s", phase, result.Stdout)
	}
}

func TestInit_NotInGitRepo(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments=2, stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "valid git repository"),
		"expected 'valid git repository' guidance in stderr, got: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "cd <path>"),
		"expected cd suggestion in stderr, got: %s", result.Stderr)
}

func TestInit_RejectsExtraArgs(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	assert.NilError(t, err, "git init failed: %s", out)

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"init", "extra"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, result.ExitCode != 0, "expected extra args to fail")
	assert.Check(t, cmp.Contains(result.Stderr, `unknown command "extra"`),
		"expected extra arg error in stderr, got: %s", result.Stderr)
}
