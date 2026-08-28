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

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// TestCreatedWindow verifies the created-filter time window always carries an
// explicit lower bound, so an "older than" query does not inherit the runs
// endpoints' implicit ~14-day default from (which would hide older runs, and for
// "my runs" return nothing — see RUN_DATE_RANGES.md).
func TestCreatedWindow(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	t.Run("older than floors the lower bound at 90 days", func(t *testing.T) {
		// "older than 2 weeks" is the case that previously returned no runs: the
		// upper bound is 2 weeks ago and the lower bound must be an explicit 90-day
		// floor, not left unset.
		from, to := createdWindow(ui.RunCreatedFilter{Duration: 14 * 24 * time.Hour}, now)
		assert.Check(t, is.Equal(from, now.AddDate(0, 0, -90)))
		assert.Check(t, is.Equal(to, now.Add(-14*24*time.Hour)))
		assert.Check(t, from.Before(to), "the window must be non-empty")
	})

	t.Run("newer than spans the age up to now", func(t *testing.T) {
		from, to := createdWindow(ui.RunCreatedFilter{Newer: true, Duration: time.Hour}, now)
		assert.Check(t, is.Equal(from, now.Add(-time.Hour)))
		assert.Check(t, is.Equal(to, now))
	})
}

// TestStepRows_UnfinishedStep verifies that a step with no stop time renders "~"
// in the duration column (rather than a blank gap), while a finished step shows
// its elapsed time, both right-padded to the same width.
func TestStepRows_UnfinishedStep(t *testing.T) {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute + 4*time.Second)

	rows := stepRows(apiclient.JobV3Execution{
		Index: 0,
		Steps: []apiclient.JobV3Step{
			{Name: "run tests", Num: 101, Phase: "ended", Outcome: "succeeded", StartedAt: start, StoppedAt: &end},
			{Name: "deploy", Num: 102, Phase: "running", StartedAt: start}, // no StoppedAt
		},
	})

	assert.Assert(t, is.Len(rows, 2))
	assert.Check(t, strings.Contains(rows[0].Label, "1m4s"), "finished step should show duration: %q", rows[0].Label)
	assert.Check(t, strings.Contains(rows[1].Label, "~"), "unfinished step should show ~: %q", rows[1].Label)
	// The "~" occupies the duration column, padded to the finished step's width.
	assert.Check(t, strings.Contains(rows[1].Label, "~    "), "~ should be padded to the duration column width: %q", rows[1].Label)
}

// TestPendingJobStatus covers the status word the job picker appends to a job that
// has not started, so it does not read like a running one. The phase arrives
// already corrected for the jobs list's optimistic "started" (see
// apiclient.effectiveJobPhase and TestGetWorkflowJobsV3_QueuedJobPhase), so
// "queued" here is the state that actually caused the confusion. An unrecognised
// phase is passed through verbatim rather than dropped: it has no glyph of its own,
// so the word is all the user gets. A running or finished job gets no word — its
// glyph already says it.
func TestPendingJobStatus(t *testing.T) {
	tests := []struct {
		phase, outcome, current string
		want                    string
	}{
		{phase: "queued", want: "queued"},
		{phase: "created", want: "created"},
		{phase: "pending", want: "pending"},    // unrecognised pre-start phase, verbatim
		{phase: "blocked", want: "blocked"},    // ditto
		{phase: "started", want: ""},           // running: the ● glyph is enough
		{phase: "started", current: "failed"},  // failing
		{phase: "ended", outcome: "succeeded"}, // finished
		{phase: "ended", current: "not_run"},   // finished without running
		{phase: "", want: ""},                  // no phase reported: nothing to say
	}

	for _, tt := range tests {
		t.Run(tt.phase+"/"+tt.outcome+tt.current, func(t *testing.T) {
			got := pendingJobStatus(apiclient.WorkflowJobV3{
				Phase: tt.phase, Outcome: tt.outcome, CurrentOutcome: tt.current,
			})
			assert.Check(t, is.Equal(got, tt.want))
		})
	}
}

// TestRunItemLabel covers the picker label for well-formed runs and for runs
// that resolved no commit — an errored/not-run pipeline whose config could not
// be fetched — where the old "%s [%s]" format left a blank "[]" row.
func TestRunItemLabel(t *testing.T) {
	created := time.Now().Add(-2 * time.Hour)

	tests := []struct {
		name string
		run  apiclient.RunV3
		want string // the descriptive part, before " - <relative time>"
	}{
		{
			name: "revision and branch",
			run:  apiclient.RunV3{Revision: "03d8295abc", Branch: "main", Phase: "ended", Outcome: "succeeded", CreatedAt: created},
			want: "[main] 03d8295",
		},
		{
			name: "tag, no branch",
			run:  apiclient.RunV3{Revision: "03d8295abc", Tag: "v1.2.3", Phase: "ended", Outcome: "succeeded", CreatedAt: created},
			want: "[v1.2.3] 03d8295",
		},
		{
			name: "commit subject leads the row",
			run: apiclient.RunV3{
				Revision: "03d8295abc", Branch: "main", Phase: "ended", Outcome: "succeeded", CreatedAt: created,
				Commit: &apiclient.RunCommit{Subject: "Fix the auth bug"},
			},
			want: "Fix the auth bug [main] 03d8295",
		},
		{
			name: "revision only",
			run:  apiclient.RunV3{Revision: "03d8295abc", Phase: "ended", Outcome: "succeeded", CreatedAt: created},
			want: "03d8295",
		},
		{
			name: "errored run with no VCS falls back to the error",
			run: apiclient.RunV3{
				Phase: "ended", CurrentOutcome: "not_run", CreatedAt: created,
				Errors: []apiclient.RunError{{Type: "config-fetch", Message: "No configuration was found in your project. Please refer to https://circleci.com/docs to get started."}},
			},
			want: "No configuration was found in your project.",
		},
		{
			name: "no VCS and no errors falls back to the status word",
			run:  apiclient.RunV3{Phase: "ended", CurrentOutcome: "not_run", CreatedAt: created},
			want: "not run",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label := runItemLabel(&tc.run, "")
			assert.Check(t, strings.HasPrefix(label, tc.want+" - "), "got %q, want prefix %q", label, tc.want+" - ")
			assert.Check(t, !strings.Contains(label, "[]"), "label should never show empty brackets: %q", label)
		})
	}
}

// TestErrorSummary verifies a run error is condensed to a single short line: its
// first sentence, capped, falling back to the type when the message is empty.
// TestDeriveDisplayStatus covers the overall status a run is reported with: its
// workflows decide it, except when the run carries errors of its own — a config
// the platform would not compile, or the rejected continuation of a
// dynamic-config run, which the API reports as a succeeded run with a succeeded
// setup workflow.
func TestDeriveDisplayStatus(t *testing.T) {
	t.Run("succeeded workflows are a succeeded run", func(t *testing.T) {
		status := deriveDisplayStatus(runGetOutput{
			Phase: "ended", CurrentOutcome: "succeeded",
			Workflows: []workflowOutput{{Name: "build", Phase: "ended", Outcome: "succeeded"}},
		})

		assert.Check(t, is.Equal(status, "succeeded"))
	})

	t.Run("a rejected continuation fails the run its setup workflow succeeded in", func(t *testing.T) {
		status := deriveDisplayStatus(runGetOutput{
			Phase: "ended", CurrentOutcome: "succeeded",
			Errors:    []errorOutput{{Type: "config", Message: "Cannot find a definition for job named release"}},
			Workflows: []workflowOutput{{Name: "setup", Phase: "ended", Outcome: "succeeded"}},
		})

		assert.Check(t, is.Equal(status, "failed"))
	})

	t.Run("a run with no workflows is described by its own phase and outcome", func(t *testing.T) {
		status := deriveDisplayStatus(runGetOutput{Phase: "ended", CurrentOutcome: "errored"})

		assert.Check(t, is.Equal(status, "⚠️ errored"))
	})
}

func TestErrorSummary(t *testing.T) {
	assert.Check(t, is.Equal(errorSummary(apiclient.RunError{
		Type: "config-fetch", Message: "No config found. See the docs.",
	}), "No config found."))

	assert.Check(t, is.Equal(errorSummary(apiclient.RunError{
		Type: "config-fetch", Message: "",
	}), "config-fetch"))

	long := errorSummary(apiclient.RunError{Message: strings.Repeat("x", 100)})
	assert.Check(t, strings.HasSuffix(long, "…"), "long messages are truncated: %q", long)
	assert.Check(t, len([]rune(long)) <= 61, "truncated to the cap plus ellipsis: %q", long)
}

// TestCommitSubject verifies the picker's commit subject is empty when no commit
// resolved, reduced to its first line, and capped for long subjects.
func TestCommitSubject(t *testing.T) {
	assert.Check(t, is.Equal(commitSubject(&apiclient.RunV3{}), ""), "no commit yields no subject")

	assert.Check(t, is.Equal(commitSubject(&apiclient.RunV3{
		Commit: &apiclient.RunCommit{Subject: "  Fix the auth bug  "},
	}), "Fix the auth bug"))

	assert.Check(t, is.Equal(commitSubject(&apiclient.RunV3{
		Commit: &apiclient.RunCommit{Subject: "Add retries\n\nThe body is dropped."},
	}), "Add retries"), "only the first line is kept")

	long := commitSubject(&apiclient.RunV3{Commit: &apiclient.RunCommit{Subject: strings.Repeat("x", 100)}})
	assert.Check(t, strings.HasSuffix(long, "…"), "long subjects are truncated: %q", long)
	assert.Check(t, len([]rune(long)) <= 51, "truncated to the cap plus ellipsis: %q", long)
}
