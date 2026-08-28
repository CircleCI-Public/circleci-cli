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
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// watchStateWith builds a ui.RunWatchState from workflow name → jobs, the shape
// both watch paths hand to the result reporters.
func watchStateWith(workflows ...ui.RunWatchWorkflow) ui.RunWatchState {
	return ui.RunWatchState{Workflows: workflows}
}

// TestFailedJobLogSuggestions verifies the suggestion attached to a failed run
// names a job the user can actually fetch: the UUID is interpolated from the
// state already in memory, so no lookup step is left to the reader.
func TestFailedJobLogSuggestions(t *testing.T) {
	jobID := uuid.MustParse("d0000000-0000-4000-8000-00000000f001")
	otherID := uuid.MustParse("d0000000-0000-4000-8000-00000000f003")

	t.Run("interpolates the failed job's id", func(t *testing.T) {
		state := watchStateWith(ui.RunWatchWorkflow{
			Name: "build",
			Jobs: []ui.RunWatchJob{
				{ID: uuid.MustParse("d0000000-0000-4000-8000-00000000f002"), Name: "lint"},
				{ID: jobID, Name: "test-service", Failed: true},
			},
		})

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get ` + jobID.String(),
		}))
	})

	t.Run("one suggestion per failed job across workflows", func(t *testing.T) {
		state := watchStateWith(
			ui.RunWatchWorkflow{Name: "build", Jobs: []ui.RunWatchJob{{ID: jobID, Name: "test-service", Failed: true}}},
			ui.RunWatchWorkflow{Name: "deploy", Jobs: []ui.RunWatchJob{{ID: otherID, Name: "publish", Failed: true}}},
		)

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get ` + jobID.String(),
			`View logs for failed job "publish": circleci job get ` + otherID.String(),
		}))
	})

	t.Run("falls back to the placeholder for a job with no id", func(t *testing.T) {
		state := watchStateWith(ui.RunWatchWorkflow{
			Name: "build",
			Jobs: []ui.RunWatchJob{{Name: "test-service", Failed: true}},
		})

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get <job-id>`,
		}))
	})

	t.Run("no suggestions when nothing failed", func(t *testing.T) {
		state := watchStateWith(ui.RunWatchWorkflow{
			Name: "build",
			Jobs: []ui.RunWatchJob{{ID: jobID, Name: "test-service"}},
		})

		assert.Check(t, is.Len(failedJobLogSuggestions(state), 0))
	})
}

// TestWatchState verifies the adapter from a polled run to the watch table's
// rows: statuses come from the emoji-free symbol/word pair, a failed job is
// flagged for --failfast, and the run reads as done when the run itself has
// ended — not when its workflows have.
func TestWatchState(t *testing.T) {
	jobID := uuid.MustParse("d0000000-0000-4000-8000-00000000f001")

	t.Run("maps phases to glyphs and words", func(t *testing.T) {
		state := watchState(runGetOutput{Workflows: []workflowOutput{{
			Name: "build", Phase: "ended", Outcome: "succeeded", Duration: "1m2s",
			Jobs: []jobOutput{{ID: jobID, Name: "test", Phase: "started", Type: "build"}},
		}}})

		assert.Assert(t, is.Len(state.Workflows, 1))
		wf := state.Workflows[0]
		assert.Check(t, is.Equal(wf.Symbol, "✓"))
		assert.Check(t, is.Equal(wf.Status, "succeeded"))
		assert.Check(t, is.Equal(wf.Duration, "1m2s"))

		assert.Assert(t, is.Len(wf.Jobs, 1))
		assert.Check(t, is.Equal(wf.Jobs[0].Symbol, "●"))
		assert.Check(t, is.Equal(wf.Jobs[0].Status, "running"))
		assert.Check(t, is.Equal(wf.Jobs[0].Type, "build"))
		assert.Check(t, !wf.Jobs[0].Failed)
	})

	t.Run("flags a failed job and the run's outcome", func(t *testing.T) {
		state := watchState(runGetOutput{Phase: "ended", CurrentOutcome: "failed", Workflows: []workflowOutput{{
			Name: "build", Phase: "ended", Outcome: "failed",
			Jobs: []jobOutput{{ID: jobID, Name: "test", Phase: "ended", Outcome: "failed"}},
		}}})

		assert.Check(t, state.Done)
		assert.Check(t, is.Equal(state.Outcome, "failed"))
		assert.Check(t, is.Len(state.FailedJobs(), 1))
	})

	t.Run("is not done while a workflow is still running", func(t *testing.T) {
		state := watchState(runGetOutput{Phase: "started", Workflows: []workflowOutput{
			{Name: "build", Phase: "ended", Outcome: "succeeded"},
			{Name: "deploy", Phase: "started"},
		}})

		assert.Check(t, !state.Done)
		assert.Check(t, !state.AllWorkflowsEnded)
	})

	t.Run("is not done while the run is going, however its workflows ended", func(t *testing.T) {
		// A dynamic-config run between its setup workflow ending and the continued
		// workflow appearing. Stopping here is what reported the setup workflow's
		// success as the whole run's.
		state := watchState(runGetOutput{Phase: "started", Workflows: []workflowOutput{
			{Name: "setup", Phase: "ended", Outcome: "succeeded"},
		}})

		assert.Check(t, !state.Done)
		assert.Check(t, state.AllWorkflowsEnded)
	})

	t.Run("is done once the run has ended, with no workflows at all", func(t *testing.T) {
		// A run whose config was rejected outright never produces a workflow, so a
		// watch waiting on its workflows would wait until the timeout.
		state := watchState(runGetOutput{Phase: "ended", CurrentOutcome: "errored"})

		assert.Check(t, state.Done)
		assert.Check(t, !state.AllWorkflowsEnded)
	})
}

// TestWatchStateRunErrors covers the dynamic-config run whose continued config
// was rejected: the setup workflow succeeded, no job failed, and the API reports
// the run itself as succeeded — the error it carries is the only sign that the
// run did not do what the config asked.
func TestWatchStateRunErrors(t *testing.T) {
	rejected := runGetOutput{
		Phase:          "ended",
		CurrentOutcome: "succeeded",
		Errors: []errorOutput{{
			Type:    "config",
			Message: "Error calling workflow: 'deploy'\nCannot find a definition for job named release",
		}},
		Workflows: []workflowOutput{{Name: "setup", Phase: "ended", Outcome: "succeeded"}},
	}

	t.Run("the run's errors are carried into the watch state", func(t *testing.T) {
		state := watchState(rejected)

		assert.Assert(t, is.Len(state.Errors, 1))
		assert.Check(t, is.Equal(state.Errors[0].Type, "config"))
		assert.Check(t, is.Contains(state.Errors[0].Message, "Cannot find a definition for job named release"))
	})

	t.Run("a run carrying errors did not succeed", func(t *testing.T) {
		assert.Check(t, is.Equal(watchState(rejected).Outcome, "failed"))
	})

	t.Run("the error is the message, since no job failed", func(t *testing.T) {
		runID := uuid.MustParse("f0000000-0000-4000-8000-00000000f001")
		msg := runFailureMessage(runID, watchState(rejected))

		assert.Check(t, is.Contains(msg, "Run "+runID.String()+" failed."))
		assert.Check(t, is.Contains(msg, "config error: Error calling workflow: 'deploy'"))
	})

	t.Run("a rejected config exits as a validation failure", func(t *testing.T) {
		assert.Check(t, is.Equal(runFailureExitCode(watchState(rejected)), clierrors.ExitValidationFail))
	})

	t.Run("a config error alongside a failed job stays a general error", func(t *testing.T) {
		withFailedJob := rejected
		withFailedJob.Workflows = append(withFailedJob.Workflows, workflowOutput{
			Name: "build", Phase: "ended", Outcome: "failed",
			Jobs: []jobOutput{{Name: "test", Phase: "ended", Outcome: "failed"}},
		})

		assert.Check(t, is.Equal(runFailureExitCode(watchState(withFailedJob)), clierrors.ExitGeneralError))
	})

	t.Run("a run with no errors is described by its outcome alone", func(t *testing.T) {
		runID := uuid.MustParse("f0000000-0000-4000-8000-00000000f001")
		state := watchState(runGetOutput{
			Phase: "ended", CurrentOutcome: "failed",
			Workflows: []workflowOutput{{Name: "build", Phase: "ended", Outcome: "failed"}},
		})

		assert.Check(t, is.Equal(runFailureMessage(runID, state), "Run "+runID.String()+" failed."))
		assert.Check(t, is.Equal(runFailureExitCode(state), clierrors.ExitGeneralError))
	})
}
