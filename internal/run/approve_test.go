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
	"gotest.tools/v3/assert/cmp"
)

func TestMatchApprovals(t *testing.T) {
	deployHold := PendingApproval{
		WorkflowID: uuid.MustParse("b0000000-0000-4000-8000-000000000001"), WorkflowName: "deploy",
		JobID: uuid.MustParse("c0000000-0000-4000-8000-000000000001"), JobName: "hold",
	}
	releaseHold := PendingApproval{
		WorkflowID: uuid.MustParse("b0000000-0000-4000-8000-000000000002"), WorkflowName: "release",
		JobID: uuid.MustParse("c0000000-0000-4000-8000-000000000002"), JobName: "hold",
	}
	gate := PendingApproval{
		WorkflowID: uuid.MustParse("b0000000-0000-4000-8000-000000000002"), WorkflowName: "release",
		JobID: uuid.MustParse("c0000000-0000-4000-8000-000000000003"), JobName: "release-gate",
	}
	pending := []PendingApproval{deployHold, releaseHold, gate}

	tests := []struct {
		name string
		job  string
		want []PendingApproval
	}{
		{name: "by name", job: "release-gate", want: []PendingApproval{gate}},
		{name: "by job UUID", job: "c0000000-0000-4000-8000-000000000002", want: []PendingApproval{releaseHold}},
		{name: "name shared by two workflows", job: "hold", want: []PendingApproval{deployHold, releaseHold}},
		{name: "no match", job: "nope", want: nil},
		{name: "workflow UUID is not a job", job: "b0000000-0000-4000-8000-000000000001", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.DeepEqual(MatchApprovals(pending, tt.job), tt.want))
		})
	}
}

func TestPendingApprovalLabel(t *testing.T) {
	a := PendingApproval{WorkflowName: "deploy", JobName: "hold"}
	assert.Check(t, cmp.Equal(a.Label(), "hold (workflow deploy)"))
}
