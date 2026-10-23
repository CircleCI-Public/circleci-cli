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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// maxTestFetches bounds how many failed jobs' test results are read at once.
const maxTestFetches = 8

// testReadBudget caps the time spent reading failed jobs' test results, for all
// of them together, so the hint never slows the report by more than this. A var
// so tests can shorten it.
var testReadBudget = 10 * time.Second

// reportJob is a job the failure report printed failed steps for.
type reportJob struct {
	name          string
	id            uuid.UUID
	hasFailedTest bool
}

// runFailureReport prints condensed output for every failed step in the run,
// organised as workflow → job → execution → step headers, followed by next
// steps. It is the output path for --failure-report and never enters the TUI.
//
// Output is written to stdout so it can be piped directly into an agent's
// context window. Empty output (no failed steps, or run not in a failed state)
// is valid and exits 0.
func runFailureReport(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, r.ID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil // no workflows yet — nothing to report
		}
		return apiErr(err, r.ID.String())
	}

	var out strings.Builder

	// The run's own errors first: a config the platform would not compile has no
	// failed step to report, so without these the report for a dynamic-config run
	// whose continued config was rejected would be empty.
	if len(r.Errors) > 0 {
		out.WriteString("## run errors\n\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&out, "- %s: %s\n", e.Type, strings.TrimSpace(e.Message))
		}
		out.WriteString("\n")
	}

	if len(r.Warnings) > 0 {
		out.WriteString("## run warnings\n\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&out, "- \u26a0 %s\n", strings.TrimSpace(w.Message))
			if w.Description != "" {
				fmt.Fprintf(&out, "  %s\n", w.Description)
			}
			if w.URL != "" {
				fmt.Fprintf(&out, "  \u2192 %s\n", w.URL)
			}
		}
		out.WriteString("\n")
	}

	var failedWorkflows []apiclient.WorkflowV3
	var failedJobs []reportJob
	for _, wf := range workflows {
		wfWritten := false

		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID.String())
		}

		for _, j := range jobs {
			jobDetail, err := client.GetJobV3(ctx, j.ID)
			if err != nil {
				if httpcl.HasStatusCode(err, http.StatusNotFound) {
					continue
				}
				return apiErr(err, j.ID.String())
			}

			// Collect executions that contain at least one failed step.
			type failedExec struct {
				exec  apiclient.JobV3Execution
				steps []apiclient.JobV3Step // only the failed steps
			}
			var failed []failedExec
			for _, exec := range jobDetail.Executions {
				var failedSteps []apiclient.JobV3Step
				for _, step := range exec.Steps {
					if step.Outcome == "failed" {
						failedSteps = append(failedSteps, step)
					}
				}
				if len(failedSteps) > 0 {
					failed = append(failed, failedExec{exec: exec, steps: failedSteps})
				}
			}
			if len(failed) == 0 {
				continue
			}

			// Lazy-write the workflow header the first time we have output for it.
			if !wfWritten {
				fmt.Fprintf(&out, "## workflow: %s\n\n", wf.Name)
				wfWritten = true
				failedWorkflows = append(failedWorkflows, wf)
			}
			failedJobs = append(failedJobs, reportJob{name: j.Name, id: j.ID})

			totalExecs := len(jobDetail.Executions)
			failedCount := len(failed)

			for _, fe := range failed {
				// Job header: include execution index and failure count only for
				// parallel jobs (more than one execution total).
				if totalExecs > 1 {
					fmt.Fprintf(&out, "### job: %s (execution %d, %d of %d failed)\n\n",
						j.Name, fe.exec.Index, failedCount, totalExecs)
				} else {
					fmt.Fprintf(&out, "### job: %s\n\n", j.Name)
				}

				for _, step := range fe.steps {
					if step.ExitCode != nil {
						fmt.Fprintf(&out, "#### step %d: %s [exit: %d]\n\n", step.Num, step.Name, *step.ExitCode)
					} else {
						fmt.Fprintf(&out, "#### step %d: %s\n\n", step.Num, step.Name)
					}

					condensed, err := client.GetJobStdoutCondensed(ctx, j.ID, fe.exec.Index, step.Num)
					if err != nil {
						if httpcl.HasStatusCode(err, http.StatusNotFound) {
							condensed = nil
						} else {
							return apiErr(err, fmt.Sprintf("step %d of job %s", step.Num, j.ID))
						}
					}

					if len(bytes.TrimSpace(condensed)) == 0 {
						out.WriteString("(no output)\n\n")
					} else {
						out.Write(condensed)
						if !bytes.HasSuffix(condensed, []byte("\n")) {
							out.WriteString("\n")
						}
						out.WriteString("\n")
					}
				}
			}
		}
	}

	writeNextSteps(ctx, client, &out, r, failedWorkflows, failedJobs)

	if out.Len() > 0 {
		iostream.Print(ctx, out.String())
	}
	return nil
}

// writeNextSteps closes a report that found failures (run errors or failed
// steps) with what to do about them. A report without any stays as it is, so
// a run with nothing wrong still prints nothing.
func writeNextSteps(ctx context.Context, client *apiclient.Client, out *strings.Builder,
	r *apiclient.RunV3, failedWorkflows []apiclient.WorkflowV3, failedJobs []reportJob) {
	if len(r.Errors) == 0 && len(failedJobs) == 0 {
		return
	}
	markFailedTests(ctx, client, failedJobs)
	out.WriteString(failureReportNextSteps(r, failedWorkflows, failedJobs))
}

// markFailedTests sets hasFailedTest on each job whose test results include a
// failure. The reads run concurrently under testReadBudget. A read that errors
// or runs out of time leaves the job unmarked, which only drops its hint: the
// report itself never fails because of it.
func markFailedTests(ctx context.Context, client *apiclient.Client, jobs []reportJob) {
	ctx, cancel := context.WithTimeout(ctx, testReadBudget)
	defer cancel()

	var g errgroup.Group
	g.SetLimit(maxTestFetches)
	for i := range jobs {
		g.Go(func() error {
			_ = client.StreamJobTests(ctx, jobs[i].id, func(tr apiclient.TestResult) {
				if tr.Result == "failure" {
					jobs[i].hasFailedTest = true
				}
			})
			return nil
		})
	}
	_ = g.Wait()
}

// failureReportNextSteps is the report's closing section: the commands to run
// about the failures above. Config errors get a validate line, failed jobs with
// failed tests a testresult line, failed workflows a transient-rerun line, and
// every report ends with how to push the fix and watch it.
func failureReportNextSteps(r *apiclient.RunV3, failedWorkflows []apiclient.WorkflowV3, failedJobs []reportJob) string {
	var lines []string
	for _, e := range r.Errors {
		if e.Type == "config" {
			lines = append(lines, "Validate the config locally: circleci config validate .circleci/config.yml")
			break
		}
	}
	var testJobs []reportJob
	for _, j := range failedJobs {
		if j.hasFailedTest {
			testJobs = append(testJobs, j)
		}
	}
	lines = append(lines, cappedLines(testJobs, "job with failed tests", "jobs with failed tests", r.ID, func(j reportJob) string {
		return fmt.Sprintf("Failed tests in job %q: circleci testresult list %s", j.name, j.id)
	})...)
	lines = append(lines, cappedLines(failedWorkflows, "failed workflow", "failed workflows", r.ID, func(wf apiclient.WorkflowV3) string {
		return fmt.Sprintf("If %q failed transiently (network timeout, lost executor), rerun its failed jobs: %s",
			wf.Name, rerunFromFailedCommand(wf.ID))
	})...)
	lines = append(lines, "Push the fix and watch it: "+pushAndWatchCommand)

	var b strings.Builder
	b.WriteString("## next steps\n\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "- %s\n", l)
	}
	return b.String()
}
