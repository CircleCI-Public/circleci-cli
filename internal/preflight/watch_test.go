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

package preflight

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

func TestJobFailed(t *testing.T) {
	tests := []struct {
		name string
		job  apiclient.WorkflowJobV3
		want bool
	}{
		{name: "failed", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "failed"}, want: true},
		{name: "errored", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "errored"}, want: true},
		{name: "infrastructure failure", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "infrastructure_fail"}, want: true},
		{name: "timed out", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "timedout"}, want: true},
		{name: "succeeded", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "succeeded"}},
		{name: "canceled", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, Outcome: "canceled"}},
		{name: "ended with only current_outcome", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseEnded, CurrentOutcome: "failed"}, want: true},
		{name: "still running with a failing current_outcome", job: apiclient.WorkflowJobV3{Phase: apiclient.PhaseStarted, CurrentOutcome: "failed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(jobFailed(tt.job), tt.want))
		})
	}
}
