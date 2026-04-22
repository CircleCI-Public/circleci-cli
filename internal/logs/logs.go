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

// Package logs implements the business logic for fetching CircleCI job logs.
package logs

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
)

// StepLog holds the output of a single job step.
type StepLog struct {
	Name     string `json:"step"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Output   string `json:"output"`
}

// ForJob fetches the log output for every step in a job, returning one StepLog
// per step. If stepFilter is non-empty, only steps whose name matches are included.
func ForJob(ctx context.Context, client *apiclient.Client, projectSlug string, jobNumber int64, stepFilter string) ([]StepLog, error) {
	job, err := client.GetJob(ctx, projectSlug, jobNumber)
	if err != nil {
		return nil, err
	}

	var results []StepLog
	for _, step := range job.Steps {
		if stepFilter != "" && step.Name != stepFilter {
			continue
		}
		sl, err := fetchStep(ctx, client, step)
		if err != nil {
			return nil, fmt.Errorf("fetching output for step %q: %w", step.Name, err)
		}
		results = append(results, sl)
	}
	return results, nil
}

// LastFailed finds the latest failed job across all workflows in a pipeline.
// If all workflows passed, it returns ErrNoneFound with a clear message so
// callers can surface a clean error rather than "no failed jobs found" after
// a full traversal.
func LastFailed(ctx context.Context, client *apiclient.Client, pipelineID string) (jobNumber int64, projectSlug string, err error) {
	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return 0, "", fmt.Errorf("fetching workflows: %w", err)
	}

	// Short-circuit: if every workflow passed, there are no failed jobs.
	allPassed := true
	for _, wf := range workflows {
		if wf.Status != "success" {
			allPassed = false
			break
		}
	}
	if allPassed {
		return 0, "", &ErrNoneFound{Reason: "all workflows in this pipeline passed"}
	}

	for _, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return 0, "", fmt.Errorf("fetching jobs for workflow %q: %w", wf.Name, err)
		}
		for _, job := range jobs {
			if job.Status == "failed" && job.JobNumber != 0 {
				return job.JobNumber, job.ProjectSlug, nil
			}
		}
	}

	return 0, "", &ErrNoneFound{Reason: "no failed jobs found in the latest pipeline"}
}

// LastJob finds the most recently completed job across all workflows in a pipeline.
func LastJob(ctx context.Context, client *apiclient.Client, pipelineID string) (jobNumber int64, projectSlug string, err error) {
	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return 0, "", fmt.Errorf("fetching workflows: %w", err)
	}

	var latestJob *apiclient.WorkflowJob
	for _, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return 0, "", fmt.Errorf("fetching jobs for workflow %q: %w", wf.Name, err)
		}
		for i, job := range jobs {
			if job.JobNumber == 0 || job.StoppedAt.IsZero() {
				continue
			}
			if latestJob == nil || job.StoppedAt.After(latestJob.StoppedAt) {
				latestJob = &jobs[i]
			}
		}
	}

	if latestJob == nil {
		return 0, "", &ErrNoneFound{Reason: "no completed jobs found in the latest pipeline"}
	}
	return latestJob.JobNumber, latestJob.ProjectSlug, nil
}

// ErrNoneFound is returned when an inference strategy finds no matching job.
type ErrNoneFound struct {
	Reason string
}

func (e *ErrNoneFound) Error() string {
	return e.Reason
}

// taskUnavailable is the step name CircleCI uses when a job was queued but
// never executed — typically because the account has no remaining credits or
// no available self-hosted runner.
const taskUnavailable = "Error: Task information unavailable"

// fetchStep builds a StepLog from a job step by fetching all of its action outputs.
func fetchStep(ctx context.Context, client *apiclient.Client, step apiclient.JobStep) (StepLog, error) {
	if step.Name == taskUnavailable {
		return StepLog{
			Name:   step.Name,
			Status: "failed",
			Output: "(job did not execute — account may have no remaining credits, or no runner was available)\n",
		}, nil
	}

	sl := StepLog{Name: step.Name, Status: "success"}
	for _, action := range step.Actions {
		// Inherit the worst status and exit code from the actions.
		if action.Status == "failed" {
			sl.Status = "failed"
			sl.ExitCode = action.ExitCode
		}
		if action.OutputURL == "" {
			continue
		}
		lines, err := client.GetStepOutput(ctx, action.OutputURL)
		if err != nil {
			return StepLog{}, err
		}
		for _, l := range lines {
			sl.Output += l.Message
		}
	}
	return sl, nil
}
