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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Job holds the details of a CircleCI job including its steps.
type Job struct {
	Number    int64      `json:"job_number"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at"`
	Steps     []JobStep  `json:"steps"`
}

// JobStep is a named step within a job.
type JobStep struct {
	Name    string       `json:"name"`
	Actions []StepAction `json:"actions"`
}

// StepAction is a single action within a step, carrying the output URL.
type StepAction struct {
	Index     int        `json:"index"`
	Step      int        `json:"step"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	ExitCode  *int       `json:"exit_code"`
	StartedAt time.Time  `json:"start_time"`
	StoppedAt *time.Time `json:"end_time"`
}

// LogLine is a single line of output from a step action.
type LogLine struct {
	Type    string `json:"type"` // "out" or "err"
	Time    string `json:"time"`
	Message string `json:"message"`
}

// GetJob fetches full job details including steps and their output URLs.
// The v2 API returns job metadata but often omits step output; if steps are
// absent, GetJob transparently retries against the v1.1 API which always
// includes step output.
func (c *Client) GetJob(ctx context.Context, projectSlug string, jobNumber int64) (*Job, error) {
	var job Job
	err := c.get(ctx, "/project/%s/job/%d", &job,
		routeParams(projectSlug, jobNumber),
	)
	if err != nil {
		return nil, err
	}

	if len(job.Steps) == 0 {
		if steps, err := c.getJobStepsV1(ctx, projectSlug, jobNumber); err == nil {
			job.Steps = steps
		}
	}

	return &job, nil
}

// getJobStepsV1 fetches step output from the v1.1 API, which includes output_url
// fields not present in the v2 job response.
func (c *Client) getJobStepsV1(ctx context.Context, projectSlug string, jobNumber int64) ([]JobStep, error) {
	var resp struct {
		Steps []JobStep `json:"steps"`
	}
	err := c.getV1(ctx, v1ProjectPath(projectSlug, jobNumber), &resp)
	if err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

// v1ProjectPath converts a v2 project slug and job number into a v1.1 API path.
// v2 slugs use short VCS prefixes (gh, bb, gl); v1.1 uses full names.
func v1ProjectPath(projectSlug string, jobNumber int64) string {
	vcs, rest, ok := strings.Cut(projectSlug, "/")
	if !ok {
		return fmt.Sprintf("/project/%s/%d", url.PathEscape(projectSlug), jobNumber)
	}
	switch vcs {
	case "gh":
		vcs = "github"
	case "bb":
		vcs = "bitbucket"
	case "gl":
		vcs = "gitlab"
	}
	return fmt.Sprintf("/project/%s/%s/%d", vcs, rest, jobNumber)
}

// --- V3 wire types ---

type jobAttributesWire struct {
	Name               string                  `json:"name"`
	Type               string                  `json:"type"`
	Phase              string                  `json:"phase"`
	Outcome            string                  `json:"outcome"`
	StartedAt          time.Time               `json:"started_at"`
	EndedAt            *time.Time              `json:"ended_at"`
	ParallelExecutions []parallelExecutionWire `json:"parallel_executions"`
}

type parallelExecutionWire struct {
	Steps []jobStepWire `json:"steps"`
}

type jobStepWire struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Num       int        `json:"num"`
	Phase     string     `json:"phase"`
	Outcome   string     `json:"outcome"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

type jobReferencesWire struct {
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	Pipeline struct {
		ID string `json:"id"`
	} `json:"pipeline"`
	Workflow struct {
		ID string `json:"id"`
	} `json:"workflow"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
}

type jobWire struct {
	ID         string            `json:"id"`
	Attributes jobAttributesWire `json:"attributes"`
	References jobReferencesWire `json:"references"`
}

// --- V3 domain types ---

// JobV3 holds job detail from the V3 API.
type JobV3 struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Status     string           `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	StoppedAt  *time.Time       `json:"stopped_at,omitempty"`
	Executions []JobV3Execution `json:"executions"`
	ProjectID  string           `json:"project_id"`
	PipelineID string           `json:"pipeline_id"`
	WorkflowID string           `json:"workflow_id"`
}

// JobV3Execution groups the steps that ran on a single executor.
type JobV3Execution struct {
	Index int         `json:"index"`
	Steps []JobV3Step `json:"steps"`
}

// JobV3Step is a single step within a V3 job response.
type JobV3Step struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Num       int        `json:"num"`
	Status    string     `json:"status"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	Duration  float64    `json:"duration_seconds"`
}

// GetJobV3 fetches job detail from the V3 API by UUID.
func (c *Client) GetJobV3(ctx context.Context, id string) (*JobV3, error) {
	var env v3Entity[jobWire]
	if err := c.getV3(ctx, "/jobs/%s", &env, routeParams(id)); err != nil {
		return nil, err
	}
	return env.Data.toJobV3(), nil
}

func (w jobWire) toJobV3() *JobV3 {
	a := w.Attributes
	j := &JobV3{
		ID:         w.ID,
		Name:       a.Name,
		Type:       a.Type,
		Status:     phaseOutcomeStatus(a.Phase, a.Outcome),
		StartedAt:  a.StartedAt,
		StoppedAt:  a.EndedAt,
		ProjectID:  w.References.Project.ID,
		PipelineID: w.References.Pipeline.ID,
		WorkflowID: w.References.Workflow.ID,
	}
	for i, pe := range a.ParallelExecutions {
		exec := JobV3Execution{Index: i}
		for _, s := range pe.Steps {
			step := JobV3Step{
				Name:      s.Name,
				Type:      s.Type,
				Num:       s.Num,
				Status:    phaseOutcomeStatus(s.Phase, s.Outcome),
				ExitCode:  s.ExitCode,
				StartedAt: s.StartedAt,
				StoppedAt: s.EndedAt,
			}
			if s.EndedAt != nil {
				step.Duration = s.EndedAt.Sub(s.StartedAt).Seconds()
			}
			exec.Steps = append(exec.Steps, step)
		}
		j.Executions = append(j.Executions, exec)
	}
	return j
}

// phaseOutcomeStatus maps V3 phase+outcome to a status string compatible
// with V2 conventions.
func phaseOutcomeStatus(phase, outcome string) string {
	switch phase {
	case "queued":
		return "queued"
	case "not_run":
		return "not_run"
	case "running":
		return "running"
	case "ended":
		switch outcome {
		case "succeeded":
			return "success"
		case "failed":
			return "failed"
		case "canceled":
			return "canceled"
		case "infrastructure_fail":
			return "infrastructure_fail"
		case "timedout":
			return "timedout"
		default:
			return outcome
		}
	default:
		return phase
	}
}

// GetStepOutput fetches the log lines from a step action's output URL.
func (c *Client) GetStepOutput(ctx context.Context, slug string, number int64, taskIndex, stepID int) ([]byte, error) {
	var output []byte
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/private/output/raw/%s/%d/output/%d/%d",
		httpcl.RouteParams(slug, number, taskIndex, stepID),
		httpcl.BytesDecoder(&output),
	))
	if err != nil {
		return nil, err
	}

	return output, nil
}

// GetStepError fetches the error lines from a step action's error URL.
func (c *Client) GetStepError(ctx context.Context, slug string, number int64, taskIndex, stepID int) ([]byte, error) {
	var output []byte
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/private/output/raw/%s/%d/error/%d/%d",
		httpcl.RouteParams(slug, number, taskIndex, stepID),
		httpcl.BytesDecoder(&output),
	))
	if err != nil {
		return nil, err
	}

	return output, nil
}
