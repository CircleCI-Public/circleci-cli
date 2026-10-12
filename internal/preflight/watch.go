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

package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

const (
	// MaxFailedTests caps the failed tests listed per job, so a job that fails
	// hundreds of tests does not flood an agent's context. The full count is
	// still reported.
	MaxFailedTests = 25
	// maxTestMessage caps each failed test's message, which is often a whole
	// stack trace.
	maxTestMessage = 1000
)

// errTimedOut is returned by watch when the run did not end within the timeout.
var errTimedOut = errors.New("timed out")

// watchOptions configures watch.
type watchOptions struct {
	client   *apiclient.Client
	runID    uuid.UUID
	interval time.Duration
	timeout  time.Duration
	failFast bool
	jobURL   func(workflowID, jobID uuid.UUID) string
	observer Observer
}

// watchResult is where a watch stopped.
type watchResult struct {
	ended   bool
	outcome string
	errors  []RunError
	failed  []JobRef
	elapsed time.Duration
}

// watch polls the run until it ends — or, with failFast, until a job fails —
// reporting each failed job to the observer the first time it is seen. It
// returns errTimedOut when the timeout elapses and the context's error when the
// context is cancelled; the result describes the run as last seen either way.
func watch(ctx context.Context, opts watchOptions) (watchResult, error) {
	start := time.Now()
	deadline := start.Add(opts.timeout)
	seen := map[uuid.UUID]bool{}
	var res watchResult
	var prevProgress string

	for {
		run, workflows, jobs, err := poll(ctx, opts.client, opts.runID)
		if err != nil {
			res.elapsed = time.Since(start)
			return res, err
		}
		res.elapsed = time.Since(start)

		progress := Progress{Elapsed: res.elapsed}
		for i, wf := range workflows {
			wp := WorkflowProgress{
				Name:   wf.Name,
				Status: apiclient.PhaseOutcomeText(wf.Phase, wf.Outcome, wf.CurrentOutcome),
				Jobs:   len(jobs[i]),
			}
			for _, j := range jobs[i] {
				if j.Phase == apiclient.PhaseEnded {
					wp.Done++
				}
				if j.Outcome != "failed" || seen[j.ID] {
					continue
				}
				seen[j.ID] = true
				failure := describeFailure(ctx, opts, wf, j)
				res.failed = append(res.failed, JobRef{
					ID: failure.ID, Name: failure.Name, Workflow: failure.Workflow, URL: failure.URL,
				})
				if err := opts.observer.JobFailed(JobFailedEvent{Type: EventJobFailed, Job: failure}); err != nil {
					return res, err
				}
			}
			progress.Workflows = append(progress.Workflows, wp)
		}
		if key := progressKey(progress); key != prevProgress {
			opts.observer.Progress(progress)
			prevProgress = key
		}

		res.errors = runErrors(run)
		if run.Phase == apiclient.PhaseEnded {
			res.ended = true
			res.outcome = runOutcome(run)
			return res, nil
		}
		if opts.failFast && len(res.failed) > 0 {
			res.outcome = OutcomeFailing
			return res, nil
		}
		if time.Now().After(deadline) {
			res.outcome = OutcomeTimedOut
			return res, errTimedOut
		}
		if err := sleep(ctx, opts.interval); err != nil {
			return res, err
		}
	}
}

// poll fetches the run, its workflows, and each workflow's jobs.
func poll(ctx context.Context, client *apiclient.Client, runID uuid.UUID) (*apiclient.RunV3, []apiclient.WorkflowV3, [][]apiclient.WorkflowJobV3, error) {
	run, err := client.GetRunV3(ctx, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	workflows, err := client.GetRunWorkflowsV3(ctx, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	jobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		jobs[i], err = client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return run, workflows, jobs, nil
}

// describeFailure gathers what an agent needs to act on a failed job: its
// failed steps and failed tests. Both are best effort — the job has already
// failed, and a detail that cannot be fetched should not stop the report.
func describeFailure(ctx context.Context, opts watchOptions, wf apiclient.WorkflowV3, j apiclient.WorkflowJobV3) JobFailure {
	f := JobFailure{
		ID:          j.ID,
		Name:        j.Name,
		WorkflowID:  wf.ID,
		Workflow:    wf.Name,
		URL:         opts.jobURL(wf.ID, j.ID),
		FailedSteps: []FailedStep{},
		FailedTests: []FailedTest{},
	}
	if detail, err := opts.client.GetJobV3(ctx, j.ID); err == nil {
		for _, exec := range detail.Executions {
			for _, step := range exec.Steps {
				if step.Outcome == "failed" {
					f.FailedSteps = append(f.FailedSteps, FailedStep{
						Execution: exec.Index, Num: step.Num, Name: step.Name, ExitCode: step.ExitCode,
					})
				}
			}
		}
	}
	_ = opts.client.StreamJobTests(ctx, j.ID, func(t apiclient.TestResult) {
		if t.Result != "failure" && t.Result != "error" {
			return
		}
		f.FailedTestCount++
		if len(f.FailedTests) < MaxFailedTests {
			f.FailedTests = append(f.FailedTests, FailedTest{
				Classname: t.Classname, Name: t.Name, Message: truncate(t.Message, maxTestMessage),
			})
		}
	})
	return f
}

// runOutcome reduces an ended run to succeeded, failed or canceled. The V3 runs
// API reports current_outcome and may leave outcome empty, so either is read. A
// run that carries its own errors failed whatever its outcome says: a
// dynamic-config run whose continued config was rejected is reported as
// succeeded by the API.
func runOutcome(run *apiclient.RunV3) string {
	outcome := run.Outcome
	if outcome == "" {
		outcome = run.CurrentOutcome
	}
	switch {
	case len(run.Errors) > 0:
		return OutcomeFailed
	case outcome == OutcomeSucceeded:
		return OutcomeSucceeded
	case outcome == OutcomeCanceled:
		return OutcomeCanceled
	default:
		return OutcomeFailed
	}
}

func runErrors(run *apiclient.RunV3) []RunError {
	out := make([]RunError, 0, len(run.Errors))
	for _, e := range run.Errors {
		out = append(out, RunError{Type: e.Type, Message: strings.TrimSpace(e.Message)})
	}
	return out
}

func progressKey(p Progress) string {
	var b strings.Builder
	for _, wf := range p.Workflows {
		_, _ = fmt.Fprintf(&b, "%s=%s:%d/%d;", wf.Name, wf.Status, wf.Done, wf.Jobs)
	}
	return b.String()
}

// truncate shortens s to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
