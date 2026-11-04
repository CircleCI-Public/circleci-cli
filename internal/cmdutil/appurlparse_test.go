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

package cmdutil

import (
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseAppURL(t *testing.T) {
	wf := uuid.MustParse("4d5b2cb0-8a5b-42d5-ac03-45881a599089")
	job := uuid.MustParse("0a0762cc-1a16-481f-81dc-b306afbac4a2")
	run := uuid.MustParse("5c08c1e0-85dc-4254-9b47-7d3d6ec865bc")

	tests := []struct {
		name string
		url  string
		want AppURLRef
	}{
		{
			name: "pipeline page",
			url:  "https://app.circleci.com/pipelines/gh/circleci/k8s-release-agent/7722",
			want: AppURLRef{ProjectSlug: "gh/circleci/k8s-release-agent", PipelineNumber: 7722},
		},
		{
			name: "pipeline page without scheme",
			url:  "app.circleci.com/pipelines/gh/circleci/k8s-release-agent/7722",
			want: AppURLRef{ProjectSlug: "gh/circleci/k8s-release-agent", PipelineNumber: 7722},
		},
		{
			name: "github app project slug",
			url:  "https://app.circleci.com/pipelines/circleci/AbCdEf123/GhIjKl456/12",
			want: AppURLRef{ProjectSlug: "circleci/AbCdEf123/GhIjKl456", PipelineNumber: 12},
		},
		{
			name: "workflow page",
			url:  "https://app.circleci.com/pipelines/gh/org/repo/7722/workflows/" + wf.String(),
			want: AppURLRef{ProjectSlug: "gh/org/repo", PipelineNumber: 7722, WorkflowID: wf},
		},
		{
			name: "job page by number",
			url:  "https://app.circleci.com/pipelines/gh/org/repo/7722/workflows/" + wf.String() + "/jobs/102197",
			want: AppURLRef{ProjectSlug: "gh/org/repo", PipelineNumber: 7722, WorkflowID: wf, JobNumber: 102197},
		},
		{
			name: "details page with workflow and job query",
			url:  "https://app.circleci.com/pipelines/gh/org/repo/7722/details?workflowId=" + wf.String() + "&job=" + job.String() + "&buildNumber=102197&jobType=build#step-103-",
			want: AppURLRef{ProjectSlug: "gh/org/repo", PipelineNumber: 7722, WorkflowID: wf, JobID: job},
		},
		{
			name: "cli run url",
			url:  "https://app.circleci.com/pipeline/" + run.String(),
			want: AppURLRef{RunID: run},
		},
		{
			name: "cli workflow url",
			url:  "https://app.circleci.com/workflow/" + wf.String(),
			want: AppURLRef{WorkflowID: wf},
		},
		{
			name: "cli job url",
			url:  "https://app.circleci.com/workflow/" + wf.String() + "/job/" + job.String(),
			want: AppURLRef{WorkflowID: wf, JobID: job},
		},
		{
			name: "jobs page by number",
			url:  "https://app.circleci.com/jobs/gh/org/repo/102197",
			want: AppURLRef{ProjectSlug: "gh/org/repo", JobNumber: 102197},
		},
		{
			name: "legacy job web_url",
			url:  "https://circleci.com/gh/org/repo/102197",
			want: AppURLRef{ProjectSlug: "gh/org/repo", JobNumber: 102197},
		},
		{
			name: "server install host",
			url:  "https://circleci.example.com/pipelines/gh/org/repo/5",
			want: AppURLRef{ProjectSlug: "gh/org/repo", PipelineNumber: 5},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAppURL(tc.url)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(*got, tc.want))
		})
	}

	t.Run("rejects", func(t *testing.T) {
		for _, bad := range []string{
			"https://app.circleci.com/projects/gh/org/repo",
			"https://app.circleci.com/pipelines/gh/org/repo/notanumber",
			"https://app.circleci.com/pipelines/gh/org/repo/7/workflows/not-a-uuid",
			"https://app.circleci.com/pipeline/not-a-uuid",
			"https://example.com/",
		} {
			_, err := ParseAppURL(bad)
			assert.Check(t, err != nil, "expected an error for %q", bad)
		}
	})
}

func TestLooksLikeURL(t *testing.T) {
	assert.Check(t, LooksLikeURL("https://app.circleci.com/pipelines/gh/o/r/1"))
	assert.Check(t, LooksLikeURL("app.circleci.com/pipelines/gh/o/r/1"))
	assert.Check(t, !LooksLikeURL("5034460f-c7c4-4c43-9457-de07e2029e7b"))
	assert.Check(t, !LooksLikeURL("7722"))
}
