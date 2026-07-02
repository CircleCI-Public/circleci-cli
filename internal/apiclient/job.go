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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
	Command   string     `json:"command"`
}

type jobReferencesWire struct {
	Project struct {
		ID uuid.UUID `json:"id"`
	} `json:"project"`
	Pipeline struct {
		ID uuid.UUID `json:"id"`
	} `json:"pipeline"`
	Workflow struct {
		ID uuid.UUID `json:"id"`
	} `json:"workflow"`
	User struct {
		ID uuid.UUID `json:"id"`
	} `json:"user"`
}

type jobWire struct {
	ID         uuid.UUID         `json:"id"`
	Attributes jobAttributesWire `json:"attributes"`
	References jobReferencesWire `json:"references"`
}

// --- V3 domain types ---

// JobV3 holds job detail from the V3 API.
type JobV3 struct {
	ID         uuid.UUID        `json:"id"`
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Phase      string           `json:"phase"`
	Outcome    string           `json:"outcome,omitempty"`
	StartedAt  time.Time        `json:"started_at"`
	StoppedAt  *time.Time       `json:"stopped_at,omitempty"`
	Executions []JobV3Execution `json:"executions"`
	ProjectID  uuid.UUID        `json:"project_id"`
	PipelineID uuid.UUID        `json:"pipeline_id"`
	WorkflowID uuid.UUID        `json:"workflow_id"`
}

// Status derives a display status from phase and outcome.
func (j JobV3) Status() string {
	return PhaseOutcomeStatus(j.Phase, j.Outcome, "")
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
	Phase     string     `json:"phase"`
	Outcome   string     `json:"outcome,omitempty"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	Command   string     `json:"command,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
}

// Status derives a display status from phase and outcome.
func (s JobV3Step) Status() string {
	return PhaseOutcomeStatus(s.Phase, s.Outcome, "")
}

// GetJobV3 fetches job detail from the V3 API by UUID.
func (c *Client) GetJobV3(ctx context.Context, id uuid.UUID) (*JobV3, error) {
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
		Phase:      a.Phase,
		Outcome:    a.Outcome,
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
				Phase:     s.Phase,
				Outcome:   s.Outcome,
				ExitCode:  s.ExitCode,
				StartedAt: s.StartedAt,
				StoppedAt: s.EndedAt,
				Command:   s.Command,
			}
			exec.Steps = append(exec.Steps, step)
		}
		j.Executions = append(j.Executions, exec)
	}
	return j
}

// PhaseOutcomeStatus derives a human-readable status string from V3
// phase, outcome, and current_outcome fields, prefixed with a status emoji.
func PhaseOutcomeStatus(phase, outcome, currentOutcome string) string {
	emoji, text := phaseOutcomeParts(phase, outcome, currentOutcome)
	if emoji == "" {
		return text
	}
	return emoji + " " + text
}

// PhaseOutcomeText is PhaseOutcomeStatus without the leading emoji — the plain
// status word (e.g. "running", "not run", "succeeded"). Use it where emoji
// shortcodes cannot be rendered, such as the interactive list pickers.
func PhaseOutcomeText(phase, outcome, currentOutcome string) string {
	_, text := phaseOutcomeParts(phase, outcome, currentOutcome)
	return text
}

// phaseOutcomeParts splits a V3 phase/outcome/current_outcome into its status
// emoji and human word, the single source of truth behind PhaseOutcomeStatus
// and PhaseOutcomeText. An empty emoji means the word stands alone.
func phaseOutcomeParts(phase, outcome, currentOutcome string) (emoji, text string) {
	switch phase {
	case "created":
		return ":hourglass_flowing_sand:", "created"
	case "queued":
		return ":hourglass:", "queued"
	case "started":
		switch currentOutcome {
		case "failed":
			return ":red_circle:", "failing"
		case "canceled":
			return ":no_entry_sign:", "canceling"
		case "errored":
			return ":warning:", "erroring"
		default:
			return ":large_blue_circle:", "running"
		}
	case "ended":
		// The V3 runs API reports only current_outcome, never outcome,
		// even once a run has ended (a rerun can change it later).
		if outcome == "" {
			return outcomeParts(currentOutcome)
		}
		return outcomeParts(outcome)
	default:
		return "", phase
	}
}

// PhaseOutcomeSymbol is like PhaseOutcomeStatus but returns a single plain
// Unicode glyph rather than an emoji shortcode + word. Use it in contexts that
// render raw text and so cannot process emoji — e.g. the interactive list
// pickers, where ":white_check_mark:" would show literally.
func PhaseOutcomeSymbol(phase, outcome, currentOutcome string) string {
	switch phase {
	case "created", "queued":
		return "○"
	case "started":
		switch currentOutcome {
		case "failed":
			return "✗"
		case "canceled":
			return "⊘"
		case "errored":
			return "!"
		default:
			return "●"
		}
	case "ended":
		// As in PhaseOutcomeStatus, the V3 runs API reports only
		// current_outcome once a run has ended.
		if outcome == "" {
			return outcomeSymbol(currentOutcome)
		}
		return outcomeSymbol(outcome)
	default:
		return "•"
	}
}

// outcomeSymbol maps a terminal outcome to a plain Unicode glyph, the
// emoji-free counterpart of outcomeParts. Unknown outcomes fall back to a
// neutral bullet.
func outcomeSymbol(outcome string) string {
	switch outcome {
	case "succeeded":
		return "✓"
	case "failed":
		return "✗"
	case "canceled":
		return "⊘"
	case "unauthorized":
		return "⊘"
	case "not_run":
		return "⊘"
	case "errored", "infrastructure_fail", "timedout":
		return "!"
	default:
		return "•"
	}
}

// outcomeParts splits a terminal run/job outcome into its status emoji and
// human word. An empty emoji means the word stands alone, so unknown outcomes
// pass through undecorated and new API values stay readable.
func outcomeParts(outcome string) (emoji, text string) {
	switch outcome {
	case "succeeded":
		return ":white_check_mark:", "succeeded"
	case "failed":
		return ":x:", "failed"
	case "canceled":
		return ":white_circle:", "canceled"
	case "unauthorized":
		return ":lock:", "unauthorized"
	case "not_run":
		// A run that never executed (e.g. its config could not be fetched or
		// compiled) — a no-entry glyph, not a warning, since nothing ran. The
		// underscore-free wording reads better than the raw "not_run" outcome.
		return ":no_entry_sign:", "not run"
	case "errored", "infrastructure_fail", "timedout":
		return ":warning:", outcome
	default:
		return "", outcome
	}
}

// GetJobStdoutRange fetches a step's stdout starting at byte offset, returning
// the bytes from that offset and whether stdout has finished — the API reports
// completion via the "X-Terminal: true" response header. Pass offset 0 for the
// first read and the number of bytes already consumed thereafter to resume
// (sent as "Range: bytes=<offset>-"). Bytes are returned raw, with ANSI styling
// intact, for colored display in a pager.
func (c *Client) GetJobStdoutRange(ctx context.Context, jobID uuid.UUID, execution, stepNum int, offset int64) (data []byte, terminal bool, err error) {
	var output []byte
	var header http.Header
	_, err = c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs/%s/stdout",
		httpcl.RouteParams(jobID),
		filterParam("execution", strconv.Itoa(execution)),
		filterParam("step_num", strconv.Itoa(stepNum)),
		httpcl.Header("Range", fmt.Sprintf("bytes=%d-", offset)),
		httpcl.BytesDecoder(&output),
		httpcl.CaptureHeader(&header),
	))
	if err != nil {
		return nil, false, err
	}
	terminal = header.Get("X-Terminal") == "true"
	return output, terminal, nil
}

func (c *Client) GetJobStdout(ctx context.Context, jobID uuid.UUID, execution, stepNum int) ([]byte, error) {
	var output []byte
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs/%s/stdout",
		httpcl.RouteParams(jobID),
		filterParam("execution", strconv.Itoa(execution)),
		filterParam("step_num", strconv.Itoa(stepNum)),
		httpcl.BytesDecoder(&output),
	))
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c *Client) GetJobStderr(ctx context.Context, jobID uuid.UUID, execution, stepNum int) ([]byte, error) {
	var output []byte
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs/%s/stderr",
		httpcl.RouteParams(jobID),
		filterParam("execution", strconv.Itoa(execution)),
		filterParam("step_num", strconv.Itoa(stepNum)),
		httpcl.BytesDecoder(&output),
	))
	if err != nil {
		return nil, err
	}

	return output, nil
}
