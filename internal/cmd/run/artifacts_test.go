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
)

func TestJobMayHaveArtifacts(t *testing.T) {
	id := uuid.MustParse("c0000000-0000-4000-8000-0000000000a1")
	tests := []struct {
		name string
		job  jobOutput
		want bool
	}{
		{
			name: "ended build job",
			job:  jobOutput{ID: id, Type: "build", Phase: "ended"},
			want: true,
		},
		{
			name: "running build job",
			job:  jobOutput{ID: id, Type: "build", Phase: "started"},
			want: true,
		},
		{
			name: "approval job",
			job:  jobOutput{ID: id, Type: "approval", Phase: "ended"},
			want: false,
		},
		{
			name: "job with no ID",
			job:  jobOutput{Type: "build", Phase: "ended"},
			want: false,
		},
		{
			name: "created job",
			job:  jobOutput{ID: id, Type: "build", Phase: "created"},
			want: false,
		},
		{
			name: "queued job",
			job:  jobOutput{ID: id, Type: "build", Phase: "queued"},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, is.Equal(jobMayHaveArtifacts(tc.job), tc.want))
		})
	}
}

func TestWorkflowLabels(t *testing.T) {
	wf := func(id, name string) workflowArtifactsOutput {
		return workflowArtifactsOutput{workflowOutput: workflowOutput{ID: uuid.MustParse(id), Name: name}}
	}

	t.Run("unique names stand alone", func(t *testing.T) {
		got := workflowLabels([]workflowArtifactsOutput{
			wf("b0000000-0000-4000-8000-000000000001", "build"),
			wf("b0000000-0000-4000-8000-000000000002", "deploy"),
		})
		assert.Check(t, is.DeepEqual(got, []string{"build", "deploy"}))
	})

	t.Run("a repeated name carries the short workflow ID", func(t *testing.T) {
		got := workflowLabels([]workflowArtifactsOutput{
			wf("034f571e-d384-4a7d-8167-6dd3c700aa39", "ci"),
			wf("fc2ae085-24e3-4167-99a4-50113c3149f5", "ci"),
			wf("b0000000-0000-4000-8000-000000000003", "nightly"),
		})
		assert.Check(t, is.DeepEqual(got, []string{"ci-034f571e", "ci-fc2ae085", "nightly"}))
	})
}
