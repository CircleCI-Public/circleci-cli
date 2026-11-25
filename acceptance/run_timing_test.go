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
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	timingRunID    = "e0000000-0000-4000-8000-0000000000e1"
	timingBuildWf  = "b0000000-0000-4000-8000-0000000000e1"
	timingDeployWf = "b0000000-0000-4000-8000-0000000000e2"
	timingLintID   = "d0000000-0000-4000-8000-0000000000e1" // 1m0s
	timingTestID   = "d0000000-0000-4000-8000-0000000000e2" // 12m0s, the slowest
	timingBuildID  = "d0000000-0000-4000-8000-0000000000e3" // still running
	timingHoldID   = "d0000000-0000-4000-8000-0000000000e4" // approval job, never starts
)

// timedJobV3 returns a listed job whose started_at and ended_at are offsets from
// the run's creation (the fake run is created at 2020-01-01 12:00:00 UTC). A
// zero ended leaves the job running; a zero started leaves it not started.
func timedJobV3(id, name, workflowID, phase, outcome string, started, ended time.Duration) fakes.JobV3 {
	created := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	j := fakes.JobV3{
		ID: id, Name: name, Type: "build", Phase: phase, Outcome: outcome,
		WorkflowID: workflowID, ProjectID: runTestProjectID,
	}
	if started > 0 {
		j.StartedAt = created.Add(started).Format(v3TimeFormat)
	}
	if ended > 0 {
		j.EndedAt = created.Add(ended).Format(v3TimeFormat)
	}
	return j
}

// setupTimingFake registers a run with two workflows: "build" with an ended
// lint job, an ended (and slowest) test job and a build job still running, and
// "deploy" held on an approval job that has no timing at all.
func setupTimingFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(timingRunID, runTestProjectID, fakeRunV3(timingRunID, runTestProjectID, "started", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(timingRunID,
		fakeWorkflowV3(timingBuildWf, "build", timingRunID, runTestProjectID, "started", "failed"),
		fakeWorkflowV3(timingDeployWf, "deploy", timingRunID, runTestProjectID, "held", ""),
	)
	fake.AddWorkflowJobsV3(timingBuildWf,
		timedJobV3(timingLintID, "lint", timingBuildWf, "ended", "succeeded", 10*time.Second, 70*time.Second),
		timedJobV3(timingTestID, "test", timingBuildWf, "ended", "failed", 12*time.Second, 12*time.Minute+12*time.Second),
		timedJobV3(timingBuildID, "build-image", timingBuildWf, "started", "", 75*time.Second, 0),
	)
	hold := timedJobV3(timingHoldID, "hold", timingDeployWf, "held", "", 0, 0)
	hold.Type = "approval"
	fake.AddWorkflowJobsV3(timingDeployWf, hold)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- run get --timing ---

// TestRunGet_Timing prints every job across the run's workflows with its
// duration, slowest first, then the running and not-started jobs, and closes
// with the wall clock and the commands that drill into the slowest job. Like
// --failure-report it bypasses the TUI and costs no request per job: the fake
// serves only the run, its workflows and each workflow's job list.
func TestRunGet_Timing(t *testing.T) {
	_, env := setupTimingFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", timingRunID, "--timing"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_Timing_RejectsJSON exits 2 when --timing is combined with --json,
// and points at the --json fields that carry the same timing.
func TestRunGet_Timing_RejectsJSON(t *testing.T) {
	_, env := setupTimingFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", timingRunID, "--timing", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	// With --json the error is itself JSON on stderr, carrying the code and the
	// message (never the title).
	assert.Check(t, cmp.Contains(result.Stderr, "run.timing_no_json"))
	assert.Check(t, cmp.Contains(result.Stderr, "started_at, stopped_at and duration"))
}

// TestRunGet_Timing_RejectsFailureReport exits 2 when both agent reports are
// asked for at once: they are separate output modes.
func TestRunGet_Timing_RejectsFailureReport(t *testing.T) {
	_, env := setupTimingFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", timingRunID, "--timing", "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Contains(result.Stderr, "failure-report"))
	assert.Check(t, cmp.Contains(result.Stderr, "timing"))
}

// TestRunGet_JSON_JobTiming verifies "run get --json" carries each job's
// started_at, stopped_at and duration from the jobs listing, omitting the ones
// a job has not reached, so no "job get" per job is needed for timing.
func TestRunGet_JSON_JobTiming(t *testing.T) {
	_, env := setupTimingFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", timingRunID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	var out struct {
		Workflows []struct {
			Name string `json:"name"`
			Jobs []struct {
				Name      string  `json:"name"`
				StartedAt *string `json:"started_at"`
				StoppedAt *string `json:"stopped_at"`
				Duration  *string `json:"duration"`
			} `json:"jobs"`
		} `json:"workflows"`
	}
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Assert(t, cmp.Len(out.Workflows, 2))
	build := out.Workflows[0].Jobs
	assert.Assert(t, cmp.Len(build, 3))

	t.Run("an ended job has all three", func(t *testing.T) {
		assert.Check(t, cmp.Equal(build[1].Name, "test"))
		assert.Check(t, cmp.DeepEqual(build[1].StartedAt, ptr("2020-01-01T12:00:12Z")))
		assert.Check(t, cmp.DeepEqual(build[1].StoppedAt, ptr("2020-01-01T12:12:12Z")))
		assert.Check(t, cmp.DeepEqual(build[1].Duration, ptr("12m0s")))
	})

	t.Run("a running job has only started_at", func(t *testing.T) {
		assert.Check(t, cmp.Equal(build[2].Name, "build-image"))
		assert.Check(t, cmp.DeepEqual(build[2].StartedAt, ptr("2020-01-01T12:01:15Z")))
		assert.Check(t, cmp.Nil(build[2].StoppedAt))
		assert.Check(t, cmp.Nil(build[2].Duration))
	})

	t.Run("an approval job has none", func(t *testing.T) {
		hold := out.Workflows[1].Jobs
		assert.Assert(t, cmp.Len(hold, 1))
		assert.Check(t, cmp.Nil(hold[0].StartedAt))
		assert.Check(t, cmp.Nil(hold[0].StoppedAt))
		assert.Check(t, cmp.Nil(hold[0].Duration))
	})
}

// ptr returns a pointer to s, for comparing against optional JSON fields.
func ptr(s string) *string { return &s }
