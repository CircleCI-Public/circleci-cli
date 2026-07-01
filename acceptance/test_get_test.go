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
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// setupTestGetFake registers a fixture that includes a name shared across two
// suites (TestDup) so the ambiguity path can be exercised.
func setupTestGetFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddJobTests(testTestsJobID,
		testResult("pkg/foo", "TestSolo", "failure", 0.42, "want 1, got 2"),
		testResult("pkg/foo", "TestDup", "success", 0.10, ""),
		testResult("pkg/bar", "TestDup", "failure", 0.20, "boom in bar"),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return env
}

// runTestGet runs "circleci test get <job-id> <extra...>" against the fixture.
func runTestGet(t *testing.T, env *testenv.TestEnv, extra ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"test", "get", testTestsJobID}, extra...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestTestGet_ByName(t *testing.T) {
	env := setupTestGetFake(t)

	result := runTestGet(t, env, "TestSolo")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestGet_JSON(t *testing.T) {
	env := setupTestGetFake(t)

	result := runTestGet(t, env, "TestSolo", "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestTestGet_Disambiguate(t *testing.T) {
	env := setupTestGetFake(t)

	// TestDup is in two suites; classname picks the pkg/bar one.
	result := runTestGet(t, env, "TestDup", "--filter", "classname=bar")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestGet_MessageRendered(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// Captured terminal output: SGR color, a carriage-return progress redraw,
	// and an erase-line — all of which termrender should resolve to plain text.
	msg := "Building\n\x1b[31mFAIL: expected 1 got 2\x1b[0m\ndownloading 10%\rdownloading 100%\n\x1b[Kdone\n"
	fake.AddJobTests(testTestsJobID, testResult("pkg/foo", "TestAnsi", "failure", 0.50, msg))
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := runTestGet(t, env, "TestAnsi")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestGet_MessageBacktickFence(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// A message that itself contains a fenced block must not break embedding.
	msg := "before\n```go\ncode()\n```\nafter\n"
	fake.AddJobTests(testTestsJobID, testResult("pkg/foo", "TestFence", "failure", 0.10, msg))
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := runTestGet(t, env, "TestFence")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestTestGet_Plain(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// Same ANSI-laden message as the rendered test; --plain must emit it
	// verbatim (escapes intact), not the termrendered form.
	msg := "Building\n\x1b[31mFAIL: expected 1 got 2\x1b[0m\ndownloading 10%\rdownloading 100%\n\x1b[Kdone\n"
	fake.AddJobTests(testTestsJobID, testResult("pkg/foo", "TestAnsi", "failure", 0.50, msg))
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := runTestGet(t, env, "TestAnsi", "--plain")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// Verbatim: the raw message is exactly the stdout.
	assert.Check(t, cmp.Equal(result.Stdout, msg))
}

func TestTestGet_PlainConflictsWithJSON(t *testing.T) {
	env := setupTestGetFake(t)

	result := runTestGet(t, env, "TestSolo", "--plain", "--json")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestGet_Ambiguous(t *testing.T) {
	env := setupTestGetFake(t)

	result := runTestGet(t, env, "TestDup")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestGet_NotFound(t *testing.T) {
	env := setupTestGetFake(t)

	result := runTestGet(t, env, "TestMissing")

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestGet_UnknownFilterKey(t *testing.T) {
	env := setupTestGetFake(t)

	// Only classname is accepted here.
	result := runTestGet(t, env, "TestSolo", "--filter", "result=failure")

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestGet_MissingArgs(t *testing.T) {
	env := setupTestGetFake(t)

	// job-id present but name missing.
	result := runTestGet(t, env)

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
