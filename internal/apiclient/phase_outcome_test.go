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

package apiclient_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestPhaseOutcome_NotRun verifies the "not_run" terminal outcome (a run that
// never executed, e.g. its config could not be fetched/compiled) renders a
// no-entry glyph and readable wording rather than a bare bullet and the raw
// "not_run". It is a no-entry, not a warning, since nothing actually ran.
func TestPhaseOutcome_NotRun(t *testing.T) {
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeSymbol("ended", "", "not_run"), "⊘"))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeText("ended", "", "not_run"), "not run"))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeStatus("ended", "", "not_run"), "🚫 not run"))
}

// TestPhaseOutcome_StatusIsTextWithEmoji checks the emoji and plain-text status
// helpers stay in lockstep: PhaseOutcomeStatus is PhaseOutcomeText with a status
// emoji prefixed (or exactly the text when there is no emoji, e.g. an unknown
// outcome that passes through undecorated).
func TestPhaseOutcome_StatusIsTextWithEmoji(t *testing.T) {
	cases := []struct{ phase, outcome, current string }{
		{"created", "", ""},
		{"queued", "", ""},
		{"started", "", ""},
		{"started", "", "failed"},
		{"ended", "succeeded", ""},
		{"ended", "", "failed"},
		{"ended", "", "not_run"},
		{"ended", "", "some_new_outcome"}, // unknown → undecorated
		{"weird_phase", "", ""},           // unknown phase → undecorated
	}
	for _, c := range cases {
		status := apiclient.PhaseOutcomeStatus(c.phase, c.outcome, c.current)
		text := apiclient.PhaseOutcomeText(c.phase, c.outcome, c.current)
		assert.Check(t, status == text || strings.HasSuffix(status, " "+text),
			"status %q should be text %q, optionally emoji-prefixed", status, text)
	}
}

// TestPhaseNotStarted checks the "has this work begun?" predicate: only
// "started" and "ended" describe work that has produced something to look at, so
// every other phase — including one this client has no glyph or wording for —
// counts as not started. That default matters: an unrecognised phase renders as a
// neutral bullet, which reads like the running dot, so callers must be able to
// tell it apart from a job that is genuinely running.
func TestPhaseNotStarted(t *testing.T) {
	notStarted := []string{"created", "queued", "pending", "blocked", "some_new_phase", ""}
	for _, phase := range notStarted {
		assert.Check(t, apiclient.PhaseNotStarted(phase), "phase %q should count as not started", phase)
	}
	for _, phase := range []string{"started", "ended"} {
		assert.Check(t, !apiclient.PhaseNotStarted(phase), "phase %q should count as started", phase)
	}
}

// TestStatusPhaseOutcome checks the pipeline.status → (phase, current_outcome)
// reverse mapping the my-runs endpoint filters on: terminal statuses map to an
// "ended" phase with the matching outcome, in-progress statuses to their
// started/queued phase, and an empty or unknown status to no filter.
func TestStatusPhaseOutcome(t *testing.T) {
	cases := []struct{ status, phase, outcome string }{
		{apiclient.StatusSuccess, "ended", "succeeded"},
		{apiclient.StatusFailed, "ended", "failed"},
		{apiclient.StatusCanceled, "ended", "canceled"},
		{apiclient.StatusError, "ended", "errored"},
		{apiclient.StatusNotRun, "ended", "not_run"},
		{apiclient.StatusUnauthorized, "ended", "unauthorized"},
		{apiclient.StatusFailing, "started", "failed"},
		{apiclient.StatusRunning, "started", ""},
		{apiclient.StatusQueued, "queued", ""},
		{"", "", ""},
		{"bogus", "", ""},
	}
	for _, c := range cases {
		phase, outcome := apiclient.StatusPhaseOutcome(c.status)
		assert.Check(t, is.Equal(phase, c.phase), "status %q phase", c.status)
		assert.Check(t, is.Equal(outcome, c.outcome), "status %q current_outcome", c.status)
	}
}

// TestJobPhaseOutcome_Approval pins the approval-gate correction. The V3 API
// reports a pending approval job in the "started" phase, which the plain phase
// vocabulary renders as "running" — wrong twice over: nothing is executing, and
// nothing will until a person approves or cancels it. Any phase short of
// "ended" therefore reads as on hold; once ended, the outcome stands.
func TestJobPhaseOutcome_Approval(t *testing.T) {
	const approval = apiclient.JobTypeApproval

	// Waiting on a decision, whatever pre-ended phase the API reports.
	for _, phase := range []string{"created", "queued", "started"} {
		assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeStatus(approval, phase, "", ""), "⏸️ on hold"), "phase %q", phase)
		assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeText(approval, phase, "", ""), "on hold"), "phase %q", phase)
		assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeSymbol(approval, phase, "", ""), "‖"), "phase %q", phase)
	}

	// Once decided, the outcome tells the story: approved is a success, an
	// unapproved gate is canceled with the rest of the workflow.
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeStatus(approval, "ended", "succeeded", ""), "✅ succeeded"))
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeText(approval, "ended", "canceled", ""), "canceled"))
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeSymbol(approval, "ended", "canceled", ""), "⊘"))

	// A build job is untouched — the correction keys on the job type.
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeStatus("build", "started", "", ""), "🔵 running"))
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeText("", "started", "", ""), "running"))
	assert.Check(t, is.Equal(apiclient.JobPhaseOutcomeSymbol("build", "started", "", ""), "●"))
}
