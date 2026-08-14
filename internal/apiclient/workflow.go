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
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
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
		ID uuid.UUID `json:"id"`
	} `json:"run"`
	Project struct {
		ID uuid.UUID `json:"id"`
	} `json:"project"`
	User struct {
		ID uuid.UUID `json:"id"`
	} `json:"user"`
}

type workflowWire struct {
	ID         uuid.UUID              `json:"id"`
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
		RunID:          w.References.Run.ID,
		ProjectID:      w.References.Project.ID,
	}
}

// --- V3 domain types ---

// WorkflowV3 holds workflow detail from the V3 API.
type WorkflowV3 struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	RunID          uuid.UUID  `json:"run_id"`
	ProjectID      uuid.UUID  `json:"project_id"`
}

// Status derives a display status from phase and outcome.
func (w WorkflowV3) Status() string {
	return PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome)
}

// GetWorkflowV3 fetches a single workflow by UUID from the V3 API.
func (c *Client) GetWorkflowV3(ctx context.Context, id uuid.UUID) (*WorkflowV3, error) {
	var env v3Entity[workflowWire]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/workflows/%s",
		httpcl.RouteParams(id),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	wf := env.Data.toWorkflowV3()
	return &wf, nil
}

// GetRunWorkflowsV3 fetches workflows for a run from the V3 API.
func (c *Client) GetRunWorkflowsV3(ctx context.Context, runID uuid.UUID) ([]WorkflowV3, error) {
	var resp v3List[workflowWire]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/workflows",
		filterParam("run_id", runID.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
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
//
// It returns the id of the *new* workflow the rerun created, which is what the
// caller needs to follow the run they just started — the id passed in belongs to
// the old workflow and is of no further use.
//
// The request field is "is_from_failed". Not "from_failed": that is the name the
// v2 endpoint and the service's own internal client use, and the v3 handler
// tolerates unknown fields rather than rejecting them — so sending the v2 name
// here silently reran everything from scratch, with a 201 and a new workflow to
// make it look like it had worked.
func (c *Client) RerunWorkflow(ctx context.Context, id string, fromFailed bool) (string, error) {
	body := map[string]any{"is_from_failed": fromFailed}
	var resp v3Entity[struct {
		ID string `json:"id"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/workflows/%s/rerun",
		httpcl.RouteParams(id),
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

// CancelWorkflow requests cancellation of a running workflow. Cancellation
// is processed asynchronously; the V3 API acknowledges with the workflow id.
func (c *Client) CancelWorkflow(ctx context.Context, id uuid.UUID) error {
	var resp v3Entity[struct {
		ID uuid.UUID `json:"id"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/workflows/%s/cancel",
		httpcl.RouteParams(id),
		httpcl.JSONDecoder(&resp),
	))
	return err
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
		ID uuid.UUID `json:"id"`
	} `json:"workflow"`
	Project struct {
		ID uuid.UUID `json:"id"`
	} `json:"project"`
}

type workflowJobWire struct {
	ID         uuid.UUID                 `json:"id"`
	Attributes workflowJobAttributesWire `json:"attributes"`
	References workflowJobReferencesWire `json:"references"`
}

// WorkflowJobV3 is a job belonging to a workflow from the V3 API.
type WorkflowJobV3 struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	Type           string     `json:"type,omitempty"`
	ProjectID      uuid.UUID  `json:"project_id"`
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
		Phase:          effectiveJobPhase(a.Phase, a.StartedAt),
		Outcome:        a.Outcome,
		CurrentOutcome: a.CurrentOutcome,
		Type:           a.Type,
		ProjectID:      w.References.Project.ID,
		StartedAt:      a.StartedAt,
		EndedAt:        a.EndedAt,
	}
}

// effectiveJobPhase corrects the phase the jobs *list* endpoint reports, which is
// optimistic: it flips a job to "started" as soon as its workflow dispatches the
// job, while the job is still waiting for an executor. Observed on one workflow's
// list response, all six jobs at once:
//
//	list   GET /jobs?filter[workflow_id]=…  → phase "started", started_at null
//	detail GET /jobs/{id}                   → phase "queued",  started_at null,
//	                                          parallel_executions []
//
// started_at is stamped only when the job really begins. Taking the list's phase
// verbatim therefore shows a queued job as running — a blue ● in the interactive
// picker, "🔵 running" in the summary tables, "phase":"started" in --json — and
// then finds no steps to drill into. A "started" job with no start time is
// reported as queued instead, which is what the job's own detail says.
//
// The list also lags by a second or two the other way: a job that has just begun
// can still show a null started_at, so it reads as queued until the next poll.
// That is the lesser error — the phase and started_at it returns contradict each
// other, and the row is only ever mislabelled, never made unreachable.
func effectiveJobPhase(phase string, startedAt *time.Time) string {
	if phase == PhaseStarted && startedAt == nil {
		return PhaseQueued
	}
	return phase
}

// GetWorkflowJobsV3 returns all jobs for a workflow via the V3 API.
func (c *Client) GetWorkflowJobsV3(ctx context.Context, workflowID uuid.UUID) ([]WorkflowJobV3, error) {
	var resp v3List[workflowJobWire]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs",
		filterParam("workflow_id", workflowID.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	jobs := make([]WorkflowJobV3, len(resp.Data))
	for i, w := range resp.Data {
		jobs[i] = w.toDomain()
	}
	return jobs, nil
}
