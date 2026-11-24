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

package run

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestTimingReport verifies the --timing report: one row per job across every
// workflow, ended jobs slowest first, then running, then not started, with
// durations and start offsets from the run's creation, closed by the wall
// clock and the commands that drill into the slowest job.
func TestTimingReport(t *testing.T) {
	// The header prints the run's creation time in the local zone. Pin it so the
	// expected text does not depend on the host (Windows names UTC "GMT").
	origLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = origLocal })

	t.Setenv("TZ", "UTC")
	created := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(offset time.Duration) *time.Time {
		ts := created.Add(offset)
		return &ts
	}
	runID := uuid.MustParse("e0000000-0000-4000-8000-0000000000a1")
	buildWf := uuid.MustParse("b0000000-0000-4000-8000-0000000000a1")
	deployWf := uuid.MustParse("b0000000-0000-4000-8000-0000000000a2")
	lint := uuid.MustParse("d0000000-0000-4000-8000-0000000000a1")
	test := uuid.MustParse("d0000000-0000-4000-8000-0000000000a2")
	build := uuid.MustParse("d0000000-0000-4000-8000-0000000000a3")
	hold := uuid.MustParse("d0000000-0000-4000-8000-0000000000a4")
	deploy := uuid.MustParse("d0000000-0000-4000-8000-0000000000a5")

	t.Run("sorts ended jobs slowest first, then running, then not started", func(t *testing.T) {
		out := buildOutput(&apiclient.RunV3{ID: runID, CreatedAt: created},
			[]apiclient.WorkflowV3{
				{ID: buildWf, Name: "build", Phase: "started"},
				{ID: deployWf, Name: "deploy", Phase: "held"},
			},
			[][]apiclient.WorkflowJobV3{
				{
					{ID: lint, Name: "lint", Phase: "ended", Outcome: "succeeded", StartedAt: at(10 * time.Second), EndedAt: at(70 * time.Second)},
					{ID: test, Name: "test", Phase: "ended", Outcome: "failed", StartedAt: at(12 * time.Second), EndedAt: at(12*time.Minute + 12*time.Second)},
					{ID: build, Name: "build-image", Phase: "started", StartedAt: at(75 * time.Second)},
				},
				{
					{ID: hold, Name: "hold", Type: "approval", Phase: "held"},
					{ID: deploy, Name: "deploy", Phase: "created"},
				},
			})

		got := timingReport(out)

		assert.Check(t, is.Equal(got, strings.Join([]string{
			"## job timing: 5 jobs, wall clock 12m12s, run created 2020-01-01 12:00:00 UTC",
			"",
			"| Job         | Workflow | Status    | Duration | Started | ID                                   |",
			"| ----------- | -------- | --------- | -------- | ------- | ------------------------------------ |",
			"| test        | build    | failed    | 12m0s    | +12s    | " + test.String() + " |",
			"| lint        | build    | succeeded | 1m0s     | +10s    | " + lint.String() + " |",
			"| build-image | build    | running   | ~        | +1m15s  | " + build.String() + " |",
			"| hold        | deploy   | on hold   |          |         | " + hold.String() + " |",
			"| deploy      | deploy   | created   |          |         | " + deploy.String() + " |",
			"",
			"## next steps",
			"",
			"- Wall clock is run creation to the last job end; the 2 finished jobs add up to 13m0s.",
			`- Step timings for the slowest job "test": circleci job get ` + test.String(),
			"- CPU and memory of the slowest job against its resource class: circleci job resource-usage get " + test.String(),
			"",
		}, "\n")))
	})

	t.Run("a run with no jobs says so", func(t *testing.T) {
		out := buildOutput(&apiclient.RunV3{ID: runID, CreatedAt: created}, nil, nil)

		got := timingReport(out)

		assert.Check(t, is.Equal(got, "## job timing\n\nNo jobs found for this run.\n"))
	})

	t.Run("a run with nothing finished points at run watch", func(t *testing.T) {
		out := buildOutput(&apiclient.RunV3{ID: runID, CreatedAt: created},
			[]apiclient.WorkflowV3{{ID: buildWf, Name: "build", Phase: "started"}},
			[][]apiclient.WorkflowJobV3{{
				{ID: lint, Name: "lint", Phase: "started", StartedAt: at(3 * time.Second)},
			}})

		got := timingReport(out)

		assert.Check(t, strings.HasPrefix(got, "## job timing: 1 job, run created 2020-01-01 12:00:00 UTC\n"), got)
		assert.Check(t, is.Contains(got, "| lint | build    | running | ~        | +3s     | "+lint.String()+" |\n"))
		assert.Check(t, is.Contains(got, "- No job has finished yet; follow the run: circleci run watch "+runID.String()+"\n"))
		assert.Check(t, !strings.Contains(got, "job get"), got)
	})
}

// TestBuildOutput_JobTiming verifies the per-job timing fields that "run get
// --json" carries, so an agent does not need a "job get" per job: started_at
// and stopped_at normalised to UTC and the duration between them, each left
// out when the job has not reached it.
func TestBuildOutput_JobTiming(t *testing.T) {
	paris := time.FixedZone("CET", 3600)
	started := time.Date(2020, 1, 1, 13, 0, 5, 0, paris)
	ended := started.Add(2*time.Minute + 3*time.Second)

	out := buildOutput(&apiclient.RunV3{},
		[]apiclient.WorkflowV3{{Name: "build", Phase: "started"}},
		[][]apiclient.WorkflowJobV3{{
			{Name: "ended", Phase: "ended", Outcome: "succeeded", StartedAt: &started, EndedAt: &ended},
			{Name: "running", Phase: "started", StartedAt: &started},
			{Name: "queued", Phase: "queued"},
		}})

	jobs := out.Workflows[0].Jobs
	assert.Assert(t, is.Len(jobs, 3))

	t.Run("an ended job has all three", func(t *testing.T) {
		assert.Check(t, is.Equal(jobs[0].Duration, "2m3s"))
		assert.Check(t, is.DeepEqual(*jobs[0].StartedAt, time.Date(2020, 1, 1, 12, 0, 5, 0, time.UTC)))
		assert.Check(t, is.DeepEqual(*jobs[0].StoppedAt, time.Date(2020, 1, 1, 12, 2, 8, 0, time.UTC)))
		assert.Check(t, is.Equal(jobs[0].StartedAt.Location(), time.UTC))
	})

	t.Run("a running job has only started_at", func(t *testing.T) {
		assert.Check(t, jobs[1].StartedAt != nil)
		assert.Check(t, is.Nil(jobs[1].StoppedAt))
		assert.Check(t, is.Equal(jobs[1].Duration, ""))
	})

	t.Run("a queued job has none", func(t *testing.T) {
		assert.Check(t, is.Nil(jobs[2].StartedAt))
		assert.Check(t, is.Nil(jobs[2].StoppedAt))
		assert.Check(t, is.Equal(jobs[2].Duration, ""))
	})
}
