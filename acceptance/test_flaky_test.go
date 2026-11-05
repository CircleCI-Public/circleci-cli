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
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	flakyProjectID  = "f1a00000-0000-4000-8000-000000000001"
	cleanProjectID  = "f1a00000-0000-4000-8000-000000000002"
	bigProjectID    = "f1a00000-0000-4000-8000-000000000003"
	noDataProjectID = "f1a00000-0000-4000-8000-000000000004"
)

// flakes builds n fake occurrences of one test, the last (most recent)
// starting at lastAt and each earlier one a day before the next. The last
// occurrence carries the IDs ending in lastN, so the JSON can show which
// occurrence's references were kept.
func flakes(name, classname, job string, n int, lastAt, lastN string) []fakes.TestFlake {
	last, err := time.Parse(time.RFC3339, lastAt)
	if err != nil {
		panic(err)
	}
	out := make([]fakes.TestFlake, n)
	for i := range n {
		id := "00"
		if i == n-1 {
			id = lastN
		}
		out[i] = fakes.TestFlake{
			Name:         name,
			Classname:    classname,
			File:         classname + "/" + name + "_test.go",
			JobName:      job,
			WorkflowName: "ci",
			JobStartedAt: last.AddDate(0, 0, i-n+1).Format(time.RFC3339),
			JobID:        "c0000000-0000-4000-8000-0000000001" + id,
			WorkflowID:   "c0000000-0000-4000-8000-0000000002" + id,
			RunID:        "c0000000-0000-4000-8000-0000000003" + id,
		}
	}
	return out
}

func setupFlakyFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	addProjectBySlug(fake, testSlug, flakyProjectID)
	addProjectBySlug(fake, "gh/testorg/clean", cleanProjectID)
	addProjectBySlug(fake, "gh/testorg/big", bigProjectID)
	addProjectBySlug(fake, "gh/testorg/nodata", noDataProjectID)

	// Registered out of order, with a times_flaked tie (2 and 2) for the name
	// tie-break. TestWatchTimeout's 22 occurrences span two pages of 20, so
	// the counts are only right if every page is fetched.
	fake.AddTestFlakes(flakyProjectID, flakes("TestUploadRetries", "internal/httpcl", "test-linux", 1, "2026-11-03T08:45:40Z", "01")...)
	fake.AddTestFlakes(flakyProjectID, flakes("TestLoginBrowserFlow", "acceptance", "acceptance", 2, "2026-11-01T18:02:11Z", "02")...)
	fake.AddTestFlakes(flakyProjectID, flakes("TestWatchTimeout", "internal/run", "test-linux", 22, "2026-11-02T10:31:00Z", "03")...)
	fake.AddTestFlakes(flakyProjectID, flakes("TestApplyDefaults", "internal/config", "test-linux", 2, "2026-10-30T12:00:00Z", "04")...)

	// 1,001 occurrences: one more than the 50 pages of 20 the command fetches.
	fake.AddTestFlakes(bigProjectID, flakes("TestHuge", "internal/big", "test-linux", 1001, "2026-11-03T08:00:00Z", "05")...)

	fake.SetTestFlakesUnavailable(noDataProjectID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

func runFlaky(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"testresult", "flaky"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestTestFlaky(t *testing.T) {
	env := setupFlakyFake(t)

	t.Run("table, most flaked first, counts across pages", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("limit", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--limit", "2")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("sort by last flaked", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--sort", "last_flaked")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	})

	t.Run("json is JSONL and honours sort and limit", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--json", "--sort", "name", "--limit", "3")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".jsonl"))
		assert.Check(t, cmp.Equal(result.Stderr, ""))
	})

	t.Run("jq selects across records", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--json", "--jq", "select(.times_flaked > 5) | .test_name")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	})

	t.Run("no flaky tests", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/clean")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("no flaky tests with json prints nothing", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/clean", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Equal(result.Stdout, ""))
		assert.Check(t, cmp.Equal(result.Stderr, ""))
	})

	t.Run("stops at the page cap and says the results are incomplete", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/big")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("page cap note goes to stderr under json", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/big", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stdout, `"times_flaked":1000`))
		assert.Check(t, cmp.Contains(result.Stderr, "Results are incomplete: stopped after 50 pages (1000 flake occurrences)."))
	})

	t.Run("search 404 says the data is unavailable, not that the project is missing", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/nodata")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("unknown project", func(t *testing.T) {
		result := runFlaky(t, env, "--project", "gh/testorg/missing")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("invalid sort key", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--sort", "bogus")
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, cmp.Contains(result.Stderr, "sort must be one of times_flaked, last_flaked, name"))
	})

	t.Run("negative limit", func(t *testing.T) {
		result := runFlaky(t, env, "--project", testSlug, "--limit", "-1")
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, cmp.Contains(result.Stderr, "--limit must be zero or greater"))
	})

	t.Run("no project outside a git checkout", func(t *testing.T) {
		result := runFlaky(t, env)
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, cmp.Contains(result.Stderr, "--project gh/org/repo"))
	})
}

func TestTestFlaky_NoToken(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.CircleCIURL = fake.URL()

	result := runFlaky(t, env, "--project", testSlug)
	assert.Check(t, cmp.Equal(result.ExitCode, 3))
}
