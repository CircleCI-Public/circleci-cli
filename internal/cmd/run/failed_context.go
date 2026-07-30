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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// runLogFailed prints condensed output for every failed step in the run,
// organised as workflow → job → execution → step headers. It is the output path
// for --log-failed and never enters the TUI.
//
// Output is written to stdout so it can be piped directly into an agent's
// context window. Empty output (no failed steps, or run not in a failed state)
// is valid and exits 0.
func runLogFailed(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, r.ID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil // no workflows yet — nothing to report
		}
		return apiErr(err, r.ID.String())
	}

	var out strings.Builder

	for _, wf := range workflows {
		wfWritten := false

		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID.String())
		}

		for _, j := range jobs {
			jobDetail, err := client.GetJobV3(ctx, j.ID)
			if err != nil {
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
			}

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

	if out.Len() > 0 {
		iostream.Print(ctx, out.String())
	}
	return nil
}
