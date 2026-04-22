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
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

// missingArgEnv returns a minimal env with a token set (token is not the
// thing being tested here; we want to reach RequireArgs, not the auth check).
func missingArgEnv(t *testing.T) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	env.Token = "testtoken"
	return env
}

func assertMissingArg(t *testing.T, result binary.CLIResult, argName string) {
	t.Helper()
	assert.Equal(t, result.ExitCode, 2, "expected exit code 2, stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, argName),
		"expected %q in stderr, got: %s", argName, result.Stderr)
}

// --- workflow ---

func TestWorkflowGet_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"workflow", "get"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "workflow-id")
}

func TestWorkflowRerun_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"workflow", "rerun"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "workflow-id")
}

func TestWorkflowCancel_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"workflow", "cancel"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "workflow-id")
}

// --- runner resource-class ---

func TestRunnerResourceClassCreate_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"runner", "resource-class", "create"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "namespace/name")
}

func TestRunnerResourceClassDelete_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"runner", "resource-class", "delete"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "namespace/name")
}

// --- runner token ---

func TestRunnerTokenCreate_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"runner", "token", "create"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "resource-class")
}

func TestRunnerTokenDelete_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"runner", "token", "delete"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "token-id")
}

// --- job ---

func TestJobLogs_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"job", "logs"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "job-number")
}

func TestJobArtifacts_MissingArg(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"job", "artifacts"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "job-number")
}

// --- settings set ---

func TestSettingsSet_MissingBothArgs(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"settings", "set"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "key")
}

func TestSettingsSet_MissingValue(t *testing.T) {
	env := missingArgEnv(t)
	result := binary.RunCLI(t, []string{"settings", "set", "token"}, env.Environ(), t.TempDir())
	assertMissingArg(t, result, "value")
}
