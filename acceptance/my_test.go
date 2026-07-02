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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// fakeMyRunV3 builds a V3 run payload with a repository_url and an explicit
// created_at, as returned by GET /api/v3/runs?filter[user_id]=me. Output groups
// these by repository, most recent run first.
func fakeMyRunV3(id, projectID, phase, outcome, repoURL, branch, revision, createdAt string) map[string]any {
	run := fakeRunV3(id, projectID, phase, outcome, branch, revision)
	attrs := run["attributes"].(map[string]any)
	attrs["vcs"].(map[string]any)["repository_url"] = repoURL
	attrs["created_at"] = createdAt
	return run
}

func TestMyRuns(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// Three runs across two projects. Output is a single table in API order (no
	// grouping, no reordering), with each run's project in its own column.
	fake.SetUserRuns(
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a003", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "1111111122222222", "2026-06-19T08:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a001", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "abc1234def5678", "2026-06-19T10:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a002", "a0000000-0000-4000-8000-00000000aa02", "ended", "failed", "https://github.com/acme/api", "feature", "deadbeef12345678", "2026-06-19T09:00:00Z"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Project")) // column header
	assert.Check(t, strings.Contains(result.Stdout, "acme/web"))
	assert.Check(t, strings.Contains(result.Stdout, "acme/api"))
	// No per-project grouping headers.
	assert.Check(t, !strings.Contains(result.Stdout, "## "))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestMyRuns_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// No runs registered.

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "No runs found."))
}

// TestMyRuns_UnknownProject covers a run the my-runs API returns without a
// repository_url: the Project column shows "(unknown)" (the project name is not
// looked up), and the run still lists.
func TestMyRuns_UnknownProject(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// fakeRunV3 carries no repository_url.
	fake.SetUserRuns(
		fakeRunV3("e0000000-0000-4000-8000-00000000b001", "a0000000-0000-4000-8000-00000000bb01", "ended", "succeeded", "main", "abc1234def5678"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "(unknown)"))
	assert.Check(t, strings.Contains(result.Stdout, "abc1234"))
}

func TestMyRuns_Limit(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetUserRuns(
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a001", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "abc1234def5678", "2026-06-19T10:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a002", "a0000000-0000-4000-8000-00000000aa02", "ended", "failed", "https://github.com/acme/api", "feature", "deadbeef12345678", "2026-06-19T09:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a003", "a0000000-0000-4000-8000-00000000aa03", "ended", "succeeded", "https://github.com/acme/cli", "main", "1111111122222222", "2026-06-19T08:00:00Z"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs", "--limit", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "acme/web"))
	assert.Check(t, strings.Contains(result.Stdout, "acme/api"))
	assert.Check(t, !strings.Contains(result.Stdout, "acme/cli"))
}

func TestMyRuns_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetUserRuns(
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a003", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "1111111122222222", "2026-06-19T08:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a001", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "abc1234def5678", "2026-06-19T10:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000a002", "a0000000-0000-4000-8000-00000000aa02", "ended", "failed", "https://github.com/acme/api", "feature", "deadbeef12345678", "2026-06-19T09:00:00Z"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	// A flat array of runs, one per run, in the API's order (no grouping). Each
	// carries the project name and its UUID.
	assert.Check(t, cmp.Len(out, 3))

	assert.Check(t, cmp.Equal(out[0]["id"], "e0000000-0000-4000-8000-00000000a003"))
	assert.Check(t, cmp.Equal(out[0]["project"], "acme/web"))
	assert.Check(t, cmp.Equal(out[0]["project_id"], "a0000000-0000-4000-8000-00000000aa01"))
	assert.Check(t, cmp.Equal(out[1]["id"], "e0000000-0000-4000-8000-00000000a001"))
	assert.Check(t, cmp.Equal(out[1]["project"], "acme/web"))
	assert.Check(t, cmp.Equal(out[2]["id"], "e0000000-0000-4000-8000-00000000a002"))
	assert.Check(t, cmp.Equal(out[2]["project"], "acme/api"))
	assert.Check(t, cmp.Equal(out[2]["project_id"], "a0000000-0000-4000-8000-00000000aa02"))
	assert.Check(t, cmp.Equal(out[2]["current_outcome"], "failed"))
}

func TestMyRuns_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
}
