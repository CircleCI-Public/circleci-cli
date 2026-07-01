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
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const testTestsJobID = "8e50c384-0083-43d0-bc8f-93f0db589d6b"

// testResult builds a fake JSONL test-result record.
func testResult(classname, name, result string, runTime float64, message string) map[string]any {
	return map[string]any{
		"classname": classname,
		"name":      name,
		"result":    result,
		"run_time":  runTime,
		"message":   message,
	}
}

func setupTestListFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	// A mix of results, in API order, so filtering and sorting have something
	// meaningful to do.
	fake.AddJobTests(testTestsJobID,
		testResult("pkg/foo", "TestAlpha", "success", 0.10, ""),
		testResult("pkg/foo", "TestBravo", "failure", 1.50, "assertion failed\nexpected 1 got 2"),
		testResult("pkg/bar", "TestCharlie", "skipped", 0, "not supported on darwin"),
		testResult("pkg/bar", "TestDelta", "failure", 0.30, "panic: boom"),
		testResult("pkg/baz", "TestEcho", "success", 0.05, ""),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	return fake, env
}

// runTestList runs "circleci test list" with the given extra args against the
// standard fixture and returns the CLI result.
func runTestList(t *testing.T, env *testenv.TestEnv, extra ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"test", "list", testTestsJobID}, extra...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestTestList_DefaultFailures(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_JSON(t *testing.T) {
	_, env := setupTestListFake(t)

	// Default failures, streamed as JSONL (one object per line).
	result := runTestList(t, env, "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".jsonl"))
}

func TestTestList_JSON_All(t *testing.T) {
	_, env := setupTestListFake(t)

	// Every result, streamed as JSONL in API order.
	result := runTestList(t, env, "--json", "--all")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".jsonl"))
}

func TestTestList_JSON_Limit(t *testing.T) {
	_, env := setupTestListFake(t)

	// Limit is applied inline while streaming; only the first matching line prints.
	result := runTestList(t, env, "--json", "--all", "--limit", "2")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".jsonl"))
}

func TestTestList_JSON_JQPerRecord(t *testing.T) {
	_, env := setupTestListFake(t)

	// A plain filter runs once per record: one name per line, in API order.
	result := runTestList(t, env, "--json", "--all", "--jq", ".name")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_JSON_JQAggregate(t *testing.T) {
	_, env := setupTestListFake(t)

	// inputs pulls the rest of the JSONL stream so jq can aggregate across
	// records: tally every outcome by result.
	result := runTestList(t, env, "--json", "--all", "--jq",
		"[.,inputs] | group_by(.result) | map({(.[0].result): length})")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_JSON_JQEvalError(t *testing.T) {
	_, env := setupTestListFake(t)

	// A runtime type error in the expression must be reported as an invalid
	// --jq expression (exit 2), not mislabeled as a CircleCI API error (exit 4).
	result := runTestList(t, env, "--json", "--all", "--jq", ".name + 1")

	assert.Equal(t, result.ExitCode, 2, "stdout: %s stderr: %s", result.Stdout, result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_JSON_JQParseError(t *testing.T) {
	_, env := setupTestListFake(t)

	// A malformed expression is likewise an invalid argument (exit 2).
	result := runTestList(t, env, "--json", "--all", "--jq", "[1,")

	assert.Equal(t, result.ExitCode, 2, "stdout: %s stderr: %s", result.Stdout, result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_JSON_SortConflict(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--json", "--sort", "name")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_FilterResult(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--filter", "result=skipped")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_FilterName(t *testing.T) {
	_, env := setupTestListFake(t)

	// Substring, case-insensitive; narrows within the failed-only default.
	result := runTestList(t, env, "--filter", "name=bravo")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_All(t *testing.T) {
	_, env := setupTestListFake(t)

	// --all shows every outcome, in API order.
	result := runTestList(t, env, "--all")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_AllWithName(t *testing.T) {
	_, env := setupTestListFake(t)

	// --all plus a name filter: every result whose name matches, including passes.
	result := runTestList(t, env, "--all", "--filter", "name=alpha")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_AllConflictsWithResult(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--all", "--filter", "result=success")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_FilterClassnameAndResult(t *testing.T) {
	_, env := setupTestListFake(t)

	// AND across keys: failures in the pkg/bar suite.
	result := runTestList(t, env, "--filter", "result=failure", "--filter", "classname=bar")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_SortRunTime(t *testing.T) {
	_, env := setupTestListFake(t)

	// Default failures (TestBravo 1.50, TestDelta 0.30) sorted ascending by run_time.
	result := runTestList(t, env, "--sort", "run_time")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_Limit(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--limit", "1")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_NoMatches(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// A job whose tests all pass — the failed-only default yields nothing.
	fake.AddJobTests(testTestsJobID,
		testResult("pkg/foo", "TestAlpha", "success", 0.10, ""),
	)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := runTestList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestList_InvalidFilterKey(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--filter", "status=failure")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_InvalidSort(t *testing.T) {
	_, env := setupTestListFake(t)

	result := runTestList(t, env, "--sort", "duration")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestList_MissingArg(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
