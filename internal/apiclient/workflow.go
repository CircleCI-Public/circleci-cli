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
	"context"
	"fmt"
	"time"
)

// WorkflowDetail holds the full details of a single workflow.
type WorkflowDetail struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	PipelineID     string     `json:"pipeline_id"`
	PipelineNumber int64      `json:"pipeline_number"`
	ProjectSlug    string     `json:"project_slug"`
	StartedBy      string     `json:"started_by"`
	CreatedAt      time.Time  `json:"created_at"`
	StoppedAt      *time.Time `json:"stopped_at"`
}

// GetWorkflow fetches a single workflow by its UUID.
func (c *Client) GetWorkflow(ctx context.Context, id string) (*WorkflowDetail, error) {
	var wf WorkflowDetail
	if err := c.get(ctx, "/workflow/"+id, &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

// RerunWorkflow triggers a rerun of the given workflow. When fromFailed is
// true only the failed jobs are rerun; otherwise all jobs restart from scratch.
func (c *Client) RerunWorkflow(ctx context.Context, id string, fromFailed bool) error {
	body := map[string]any{"from_failed": fromFailed}
	var resp struct {
		Message string `json:"message"`
	}
	return c.post(ctx, fmt.Sprintf("/workflow/%s/rerun", id), body, &resp)
}

// CancelWorkflow requests cancellation of a running workflow.
func (c *Client) CancelWorkflow(ctx context.Context, id string) error {
	var resp struct {
		Message string `json:"message"`
	}
	return c.post(ctx, fmt.Sprintf("/workflow/%s/cancel", id), map[string]any{}, &resp)
}

// WorkflowJob is a job belonging to a workflow.
type WorkflowJob struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	JobNumber   int64     `json:"job_number"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	ProjectSlug string    `json:"project_slug"`
	StartedAt   time.Time `json:"started_at"`
	StoppedAt   time.Time `json:"stopped_at"`
}

// GetWorkflowJobs returns all jobs belonging to a workflow.
func (c *Client) GetWorkflowJobs(ctx context.Context, workflowID string) ([]WorkflowJob, error) {
	var resp struct {
		Items []WorkflowJob `json:"items"`
	}
	if err := c.get(ctx, "/workflow/"+workflowID+"/job", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}
