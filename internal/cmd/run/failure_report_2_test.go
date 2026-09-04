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
	"errors"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestFailureReport2_MutualExclusion verifies --failure-report-2 is rejected
// when combined with any of the other output-mode flags, before any client is
// constructed (so this never reaches the network).
func TestFailureReport2_MutualExclusion(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"with --failure-report", []string{"--failure-report-2", "--failure-report"}},
		{"with --json", []string{"--failure-report-2", "--json"}},
		{"with --jq", []string{"--failure-report-2", "--jq", "."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newGetCmd()
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs(tc.args)

			err := cmd.Execute()

			var cliErr *clierrors.CLIError
			assert.Assert(t, errors.As(err, &cliErr), "expected a *clierrors.CLIError, got %v", err)
			assert.Check(t, cmp.Equal(cliErr.Code, "run.failure_report_2_exclusive"))
			assert.Check(t, cmp.Equal(cliErr.ExitCode, clierrors.ExitBadArguments))
		})
	}
}

// TestMapRunErrorKind covers the best-effort string-match table that guesses a
// failure-report kind from a RunV3 error's opaque Type string.
func TestMapRunErrorKind(t *testing.T) {
	tests := []struct {
		errType string
		want    string
	}{
		{"config", "invalid_config"},
		{"config-fetch", "missing_config"},
		{"fetch-config-error", "missing_config"},
		{"trigger-error", "trigger_failure"},
		{"some-unrecognised-upstream-string", "invalid_config"}, // fallback
	}
	for _, tc := range tests {
		t.Run(tc.errType, func(t *testing.T) {
			assert.Check(t, cmp.Equal(mapRunErrorKind(tc.errType), tc.want))
		})
	}
}

// TestRunErrorItems verifies run-level errors are mapped to items carrying
// the guessed kind, the trimmed message, and the raw unmapped type for
// reviewer sanity-checking, with a deterministic synthetic ID per error.
func TestRunErrorItems(t *testing.T) {
	r := &apiclient.RunV3{
		ID: uuid.MustParse("e0000000-0000-4000-8000-0000000000f1"),
		Errors: []apiclient.RunError{
			{Type: "config-fetch", Message: "  could not fetch config  \n"},
			{Type: "trigger-error", Message: "bad trigger"},
		},
	}

	items := runErrorItems(r)

	assert.Assert(t, cmp.Len(items, 2))
	assert.Check(t, cmp.Equal(items[0].Attributes.Kind, "missing_config"))
	assert.Check(t, cmp.Equal(items[0].Attributes.Message, "could not fetch config"))
	assert.Check(t, cmp.Equal(items[0].Attributes.RawType, "config-fetch"))
	assert.Check(t, cmp.Equal(items[1].Attributes.Kind, "trigger_failure"))
	// Deterministic: recomputing from the same inputs yields the same ID.
	assert.Check(t, cmp.Equal(items[0].ID, syntheticID(r.ID.String(), "run-error", "0", "config-fetch")))
	assert.Check(t, items[0].ID != items[1].ID, "distinct errors must not collide")
}

// TestDedupLatestJobs verifies the documented spec's dedup rule: only the
// latest attempt per (workflow name, job name) survives, keeping the last
// occurrence in traversal order and preserving relative order of the
// survivors.
func TestDedupLatestJobs(t *testing.T) {
	wfA := apiclient.WorkflowV3{ID: uuid.New(), Name: "build"}
	wfB := apiclient.WorkflowV3{ID: uuid.New(), Name: "deploy"}

	firstAttempt := apiclient.WorkflowJobV3{ID: uuid.New(), Name: "test", Outcome: "failed"}
	retryAttempt := apiclient.WorkflowJobV3{ID: uuid.New(), Name: "test", Outcome: "succeeded"}
	otherJob := apiclient.WorkflowJobV3{ID: uuid.New(), Name: "release", Outcome: "succeeded"}

	pairs := []wfJob{
		{wf: wfA, job: firstAttempt},
		{wf: wfB, job: otherJob},
		{wf: wfA, job: retryAttempt}, // retry of the same (workflow, job) name
	}

	got := dedupLatestJobs(pairs)

	// Order is preserved by each survivor's original position: otherJob sits
	// at index 1 (its only occurrence) and the wfA/"test" retry at index 2
	// (its last), so otherJob comes first even though it was registered after
	// the (discarded) first attempt.
	assert.Assert(t, cmp.Len(got, 2))
	assert.Check(t, cmp.Equal(got[0].job.ID, otherJob.ID))
	assert.Check(t, cmp.Equal(got[1].job.ID, retryAttempt.ID))
	// The wfA/"test" survivor is the retry (last occurrence), not the first attempt.
	assert.Check(t, cmp.Equal(got[1].job.Outcome, "succeeded"))
}

// TestSyntheticID verifies syntheticID is a pure, deterministic function of
// its inputs — required so that re-running the report for the same run
// produces stable IDs, and so distinct inputs don't collide.
func TestSyntheticID(t *testing.T) {
	a := syntheticID("job-1", "test", "pkg", "TestFoo")
	b := syntheticID("job-1", "test", "pkg", "TestFoo")
	c := syntheticID("job-1", "test", "pkg", "TestBar")

	assert.Check(t, cmp.Equal(a, b))
	assert.Check(t, a != c)
}
