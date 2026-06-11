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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// missingArgEnv returns a minimal env with a token set (token is not the
// thing being tested here; we want to reach RequireArgs, not the auth check).
func missingArgEnv(t *testing.T) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	return env
}

func assertMissingArg(t *testing.T, result binary.CLIResult, argName string) {
	t.Helper()
	assert.Equal(t, result.ExitCode, 2, "expected exit code 2, stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, argName),
		"expected %q in stderr, got: %s", argName, result.Stderr)
}

// --- workflow ---

func TestWorkflowGet_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "workflow-id")
}

func TestWorkflowRerun_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "workflow-id")
}

func TestWorkflowCancel_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "workflow-id")
}

// --- runner resource-class ---

func TestRunnerResourceClassCreate_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "create"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "namespace/name")
}

func TestRunnerResourceClassDelete_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "delete"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "namespace/name")
}

// --- runner token ---

func TestRunnerTokenCreate_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "create"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "resource-class")
}

func TestRunnerTokenDelete_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "token", "delete"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "token-id")
}

// --- job ---

func TestJobArtifact_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "artifact"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "job-id")
}

// --- setting set ---

func TestSettingSet_MissingBothArgs(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"setting", "set"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "key")
}

func TestSettingSet_MissingValue(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"setting", "set", "token"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assertMissingArg(t, result, "value")
}
