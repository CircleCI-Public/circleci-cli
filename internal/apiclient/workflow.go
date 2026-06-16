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
	"time"
)

// --- V3 wire types ---

type workflowAttributesWire struct {
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	EndedAt        *time.Time `json:"ended_at"`
}

type workflowReferencesWire struct {
	Run struct {
		ID string `json:"id"`
	} `json:"run"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
}

type workflowWire struct {
	ID         string                 `json:"id"`
	Attributes workflowAttributesWire `json:"attributes"`
	References workflowReferencesWire `json:"references"`
}

func (w workflowWire) toWorkflowV3() WorkflowV3 {
	a := w.Attributes
	return WorkflowV3{
		ID:             w.ID,
		Name:           a.Name,
		Phase:          a.Phase,
		Outcome:        a.Outcome,
		CurrentOutcome: a.CurrentOutcome,
		CreatedAt:      a.CreatedAt,
		EndedAt:        a.EndedAt,
		EventID:        w.References.Run.ID,
		ProjectID:      w.References.Project.ID,
	}
}

// --- V3 domain types ---

// WorkflowV3 holds workflow detail from the V3 API.
type WorkflowV3 struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	EventID        string     `json:"event_id"`
	ProjectID      string     `json:"project_id"`
}

// Status derives a display status from phase and outcome.
func (w WorkflowV3) Status() string {
	return PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome)
}

// GetWorkflowV3 fetches a single workflow by UUID from the V3 API.
func (c *Client) GetWorkflowV3(ctx context.Context, id string) (*WorkflowV3, error) {
	var env v3Entity[workflowWire]
	if err := c.getV3(ctx, "/workflows/%s", &env, routeParams(id)); err != nil {
		return nil, err
	}
	wf := env.Data.toWorkflowV3()
	return &wf, nil
}

// GetEventWorkflows fetches the workflows grouped under a trigger event
// from the V3 API.
func (c *Client) GetEventWorkflows(ctx context.Context, eventID string) ([]WorkflowV3, error) {
	var resp v3List[workflowWire]
	// Target filter: event_id (the wire still calls the event a run).
	if err := c.getV3(ctx, "/workflows", &resp, filterParam("run_id", eventID)); err != nil {
		return nil, err
	}
	workflows := make([]WorkflowV3, len(resp.Data))
	for i, w := range resp.Data {
		workflows[i] = w.toWorkflowV3()
	}
	return workflows, nil
}

// RerunWorkflow triggers a rerun of the given workflow. When fromFailed is
// true only the failed jobs are rerun; otherwise all jobs restart from scratch.
func (c *Client) RerunWorkflow(ctx context.Context, id string, fromFailed bool) error {
	body := map[string]any{"from_failed": fromFailed}
	var resp struct {
		Message string `json:"message"`
	}
	return c.post(ctx, "/workflow/%s/rerun", body, &resp,
		routeParams(id),
	)
}

// CancelWorkflow requests cancellation of a running workflow. Cancellation
// is processed asynchronously; the V3 API acknowledges with the workflow id.
func (c *Client) CancelWorkflow(ctx context.Context, id string) error {
	var resp v3Entity[struct {
		ID string `json:"id"`
	}]
	return c.postV3(ctx, "/workflows/%s/cancel", nil, &resp,
		routeParams(id),
	)
}

// WorkflowJob is a job belonging to a workflow (V2 API).
// Used by artifacts and logs which need JobNumber and ProjectSlug.
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

// GetWorkflowJobs returns all jobs belonging to a workflow via V2.
func (c *Client) GetWorkflowJobs(ctx context.Context, workflowID string) ([]WorkflowJob, error) {
	var resp struct {
		Items []WorkflowJob `json:"items"`
	}
	err := c.get(ctx, "/workflow/%s/job", &resp,
		routeParams(workflowID),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// --- V3 workflow jobs ---

type workflowJobAttributesWire struct {
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Type           string     `json:"type,omitempty"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
}

type workflowJobReferencesWire struct {
	Workflow struct {
		ID string `json:"id"`
	} `json:"workflow"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
}

type workflowJobWire struct {
	ID         string                    `json:"id"`
	Attributes workflowJobAttributesWire `json:"attributes"`
	References workflowJobReferencesWire `json:"references"`
}

// WorkflowJobV3 is a job belonging to a workflow from the V3 API.
type WorkflowJobV3 struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	Type           string     `json:"type,omitempty"`
	ProjectID      string     `json:"project_id"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
}

// Status derives a display status from phase and outcome.
func (w WorkflowJobV3) Status() string {
	return PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome)
}

func (w workflowJobWire) toDomain() WorkflowJobV3 {
	a := w.Attributes
	return WorkflowJobV3{
		ID:             w.ID,
		Name:           a.Name,
		Phase:          a.Phase,
		Outcome:        a.Outcome,
		CurrentOutcome: a.CurrentOutcome,
		Type:           a.Type,
		ProjectID:      w.References.Project.ID,
		StartedAt:      a.StartedAt,
		EndedAt:        a.EndedAt,
	}
}

// GetWorkflowJobsV3 returns all jobs for a workflow via the V3 API.
func (c *Client) GetWorkflowJobsV3(ctx context.Context, workflowID string) ([]WorkflowJobV3, error) {
	var resp v3List[workflowJobWire]
	err := c.getV3(ctx, "/jobs", &resp,
		filterParam("workflow_id", workflowID),
	)
	if err != nil {
		return nil, err
	}
	jobs := make([]WorkflowJobV3, len(resp.Data))
	for i, w := range resp.Data {
		jobs[i] = w.toDomain()
	}
	return jobs, nil
}
