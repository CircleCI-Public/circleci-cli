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
	"fmt"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// fakeMyRunV3 builds a V3 run payload with an origin_repository_url and an
// explicit created_at, as returned by GET /api/v3/runs?filter[user_id]=me.
// Output groups these by repository, most recent run first.
func fakeMyRunV3(id, projectID, phase, outcome, repoURL, branch, revision, createdAt string) fakes.RunV3 {
	run := fakeRunV3(id, projectID, phase, outcome, branch, revision)
	run.CreatedAt = createdAt
	run.OriginRepoURL = repoURL
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

// TestMyRuns_UnknownProject covers a run the my-runs API returns without an
// origin_repository_url: the Project column shows "(unknown)" (the project name
// is not looked up), and the run still lists.
func TestMyRuns_UnknownProject(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// fakeRunV3 carries no origin_repository_url.
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

// TestMyRuns_PaginatesMultiPage verifies that my runs transparently paginates
// when the requested limit exceeds the API's max page size (20). With 25 user
// runs in the fake and --limit 25, the client must make two requests and return
// all 25 results — not just the first page of 20.
func TestMyRuns_PaginatesMultiPage(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runs := make([]fakes.RunV3, 25)
	for i := range runs {
		id := fmt.Sprintf("e0000000-0000-4000-8000-%012x", i+1)
		rev := fmt.Sprintf("%016x", i+1)
		runs[i] = fakeMyRunV3(id, "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded",
			"https://github.com/acme/web", "main", rev, "2026-06-19T10:00:00Z")
	}
	fake.SetUserRuns(runs...)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs", "--limit", "25", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Len(out, 25), "expected 25 runs across two pages, got %d", len(out))
	assert.Check(t, cmp.Equal(out[0]["id"], "e0000000-0000-4000-8000-000000000001"))
	assert.Check(t, cmp.Equal(out[24]["id"], "e0000000-0000-4000-8000-000000000019"))
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

// --- my runs --failure-report ---

// TestMyRuns_FailureReport picks the newest run with a failed outcome — here
// one still running with a failed job, ahead of an older run that ended failed
// and skipping a newer run that succeeded — and prints a header naming it
// followed by its failure report.
func TestMyRuns_FailureReport(t *testing.T) {
	const (
		failingRunID = "e0000000-0000-4000-8000-00000000b002"
		wfID         = "b0000000-0000-4000-8000-00000000b002"
		jobID        = "d0000000-0000-4000-8000-00000000b002"
		projectID    = "a0000000-0000-4000-8000-00000000aa02"
	)
	fake := fakes.NewCircleCI(t)
	fake.SetUserRuns(
		fakeMyRunV3("e0000000-0000-4000-8000-00000000b001", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "1111111122222222", "2026-06-19T11:00:00Z"),
		fakeMyRunV3(failingRunID, projectID, "started", "failed", "https://github.com/acme/api", "feature", "deadbeef12345678", "2026-06-19T10:00:00Z"),
		fakeMyRunV3("e0000000-0000-4000-8000-00000000b003", projectID, "ended", "failed", "https://github.com/acme/api", "main", "abc1234def5678", "2026-06-19T09:00:00Z"),
	)
	fake.AddRunWorkflowsV3(failingRunID, fakeWorkflowV3(wfID, "build", failingRunID, projectID, "started", "failed"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3(jobID, "compile", wfID, projectID))

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: jobID, Name: "compile", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: wfID, ProjectID: projectID,
		Executions: [][]fakes.JobStep{{
			{Name: "go build", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobStdoutCondensed(jobID, 0, 101, []byte("internal/cmd/run/trigger.go:90:9: undefined: cmdd\n"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs", "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestMyRuns_FailureReport_NoFailedRuns says so on stderr and exits 0 when none
// of the user's runs failed, as an empty list does.
func TestMyRuns_FailureReport_NoFailedRuns(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetUserRuns(
		fakeMyRunV3("e0000000-0000-4000-8000-00000000b001", "a0000000-0000-4000-8000-00000000aa01", "ended", "succeeded", "https://github.com/acme/web", "main", "1111111122222222", "2026-06-19T11:00:00Z"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"my", "runs", "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestMyRuns_FailureReport_RejectsOtherFlags exits 2 when --failure-report is
// combined with a flag it cannot honour: the report is plain text about one run.
func TestMyRuns_FailureReport_RejectsOtherFlags(t *testing.T) {
	for _, flag := range [][]string{{"--json"}, {"--jq", ".[]"}, {"--limit", "5"}} {
		t.Run(flag[0], func(t *testing.T) {
			env := testenv.New(t)
			env.Token = testToken

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    append([]string{"my", "runs", "--failure-report"}, flag...),
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 2))
			assert.Check(t, cmp.Equal(result.Stdout, ""))
			assert.Check(t, cmp.Contains(result.Stderr, "failure-report"))
		})
	}
}
