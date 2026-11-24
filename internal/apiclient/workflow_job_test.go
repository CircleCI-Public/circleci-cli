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

package apiclient

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// TestWorkflowJobV3_Duration verifies a job's duration is the span from
// started_at to ended_at, absent until the job has both, and never negative.
func TestWorkflowJobV3_Duration(t *testing.T) {
	started := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	ended := started.Add(90 * time.Second)
	before := started.Add(-time.Second)

	tests := []struct {
		name   string
		job    WorkflowJobV3
		want   time.Duration
		wantOK bool
	}{
		{name: "ended", job: WorkflowJobV3{StartedAt: &started, EndedAt: &ended}, want: 90 * time.Second, wantOK: true},
		{name: "running", job: WorkflowJobV3{StartedAt: &started}},
		{name: "not started", job: WorkflowJobV3{}},
		{name: "ended before it started reads as zero", job: WorkflowJobV3{StartedAt: &started, EndedAt: &before}, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.job.Duration()
			assert.Check(t, cmp.Equal(ok, tt.wantOK))
			assert.Check(t, cmp.Equal(got, tt.want))
		})
	}
}

func TestWorkflowJobV3_Failed(t *testing.T) {
	tests := []struct {
		name string
		job  WorkflowJobV3
		want bool
	}{
		{name: "failed", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "failed"}, want: true},
		{name: "errored", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "errored"}, want: true},
		{name: "infrastructure failure", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "infrastructure_fail"}, want: true},
		{name: "timed out", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "timedout"}, want: true},
		{name: "succeeded", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "succeeded"}},
		{name: "canceled", job: WorkflowJobV3{Phase: PhaseEnded, Outcome: "canceled"}},
		{name: "ended with only current_outcome", job: WorkflowJobV3{Phase: PhaseEnded, CurrentOutcome: "failed"}, want: true},
		{name: "still running with a failing current_outcome", job: WorkflowJobV3{Phase: PhaseStarted, CurrentOutcome: "failed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(tt.job.Failed(), tt.want))
		})
	}
}
