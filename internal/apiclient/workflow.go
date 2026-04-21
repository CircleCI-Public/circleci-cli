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
	var resp struct{ Message string `json:"message"` }
	return c.post(ctx, fmt.Sprintf("/workflow/%s/rerun", id), body, &resp)
}

// CancelWorkflow requests cancellation of a running workflow.
func (c *Client) CancelWorkflow(ctx context.Context, id string) error {
	var resp struct{ Message string `json:"message"` }
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
