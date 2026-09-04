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
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// runFailureReport2 is the output path for the hidden, experimental
// --failure-report-2 flag. It is a throwaway client-side mockup of a possible
// future GET /api/v3/runs/:id/failure-report endpoint — that endpoint does not
// exist. Everything here is assembled by composing existing, public v3 API
// calls (GetRunV3, GetRunWorkflowsV3, GetWorkflowJobsV3, GetJobV3,
// StreamJobTests); nothing proxies a new route. The purpose is to let someone
// eyeball the target response shape against real run data before anyone
// commits to building the real endpoint.
//
// Confidence varies sharply by kind:
//   - failed_job, unauthorized_job: solid — WorkflowJobV3.Outcome already
//     reports "failed"/"unauthorized" as documented values.
//   - failed_test: solid — StreamJobTests decodes the same JSONL the real
//     test-results endpoint serves; only failing cases are kept.
//   - invalid_config, missing_config, trigger_failure: best-effort only. See
//     mapRunErrorKind for why.
func runFailureReport2(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, r.ID)
	if err != nil {
		if !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return apiErr(err, r.ID.String())
		}
		workflows = nil // no workflows yet — nothing job-scoped to report
	}

	var pairs []wfJob
	for _, wf := range workflows {
		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID.String())
		}
		for _, j := range jobs {
			pairs = append(pairs, wfJob{wf, j})
		}
	}

	items := runErrorItems(r)
	for _, p := range dedupLatestJobs(pairs) {
		jobItems, err := jobFailureItems(ctx, client, p.wf, p.job)
		if err != nil {
			return err
		}
		items = append(items, jobItems...)
	}

	return iostream.PrintJSON(ctx, failureReportOutput{Data: items})
}

// wfJob pairs a job with the workflow it belongs to, so a flat traversal
// order (workflow → job) can be preserved through the dedup step below.
type wfJob struct {
	wf  apiclient.WorkflowV3
	job apiclient.WorkflowJobV3
}

// dedupLatestJobs applies the documented spec's dedup rule: job-scoped items
// keep only the latest attempt per (workflow name, job name). GetWorkflowJobsV3
// returns a retried job's attempts as separate entries in the order the API
// created them, so the last occurrence for a given key is taken as the latest
// attempt — no attempt number is exposed to compare instead. The result
// preserves the input order of each kept entry.
func dedupLatestJobs(pairs []wfJob) []wfJob {
	latestIdx := make(map[[2]string]int, len(pairs))
	for i, p := range pairs {
		latestIdx[[2]string{p.wf.Name, p.job.Name}] = i
	}
	out := make([]wfJob, 0, len(latestIdx))
	for i, p := range pairs {
		if latestIdx[[2]string{p.wf.Name, p.job.Name}] == i {
			out = append(out, p)
		}
	}
	return out
}

// failureReportOutput is the top-level synthetic response shape: a flat list
// of failure items, mirroring the documented (unimplemented)
// GET /api/v3/runs/:id/failure-report response.
type failureReportOutput struct {
	Data []failureReportItem `json:"data"`
}

// failureReportItem and its nested types are declared locally rather than
// reusing any apiclient response type: this shape is not returned by any real
// endpoint, so mutating a real apiclient struct to fit it would risk the two
// drifting into each other by accident.
type failureReportItem struct {
	ID         uuid.UUID               `json:"id"`
	Attributes failureReportAttributes `json:"attributes"`
	References failureReportReferences `json:"references,omitempty"`
}

type failureReportAttributes struct {
	// Kind is one of invalid_config, missing_config, trigger_failure,
	// unauthorized_job, failed_job, failed_test.
	Kind      string `json:"kind"`
	Message   string `json:"message,omitempty"`
	File      string `json:"file,omitempty"`
	Classname string `json:"classname,omitempty"`
	Name      string `json:"name,omitempty"`

	// RawType carries the unmapped, verbatim RunError.Type string for a
	// run-level item (invalid_config/missing_config/trigger_failure). It has
	// no place in the documented target shape and exists only in this
	// experimental path, so whoever reviews the mockup output against real
	// run data can sanity-check mapRunErrorKind's guesses.
	RawType string `json:"debug_raw_type,omitempty"`
}

type failureReportReferences struct {
	Workflow *failureReportRef `json:"workflow,omitempty"`
	Job      *failureReportRef `json:"job,omitempty"`
}

type failureReportRef struct {
	ID         uuid.UUID             `json:"id"`
	Attributes failureReportRefAttrs `json:"attributes,omitempty"`
}

type failureReportRefAttrs struct {
	Name string `json:"name,omitempty"`
}

// runErrorItems maps a run's own errors to best-effort invalid_config /
// missing_config / trigger_failure items. There is no job or workflow to
// reference here — these errors belong to the run itself (e.g. a
// dynamic-config run whose continued config was rejected, which never
// produces a failed step).
//
// GetRunV3's wire type does not currently surface a "warnings" list alongside
// Errors, so only Errors is used; extending the real RunV3 type to add one is
// out of scope for this mockup.
func runErrorItems(r *apiclient.RunV3) []failureReportItem {
	items := make([]failureReportItem, 0, len(r.Errors))
	for i, e := range r.Errors {
		items = append(items, failureReportItem{
			ID: syntheticID(r.ID.String(), "run-error", strconv.Itoa(i), e.Type),
			Attributes: failureReportAttributes{
				Kind:    mapRunErrorKind(e.Type),
				Message: strings.TrimSpace(e.Message),
				RawType: e.Type,
			},
		})
	}
	return items
}

// mapRunErrorKind guesses a failure-report kind from a RunV3 error's Type
// string.
//
// UNCONFIRMED / GUESSED: query-service treats RunError.Type as an opaque
// passthrough string from an upstream Temporal activity. There is no
// documented enum and no confirmed contract for the values it takes — this is
// a small string-match table built from example values seen in the wild, not
// a source of truth. It will silently misclassify (or fall through to
// invalid_config) the moment the upstream string changes. See RawType above
// for how to sanity-check it against real data; do not port this table into a
// real backend endpoint without confirming the values with query-service.
func mapRunErrorKind(errType string) string {
	t := strings.ToLower(errType)
	switch {
	case strings.Contains(t, "config-fetch"), strings.Contains(t, "fetch"):
		return "missing_config"
	case strings.Contains(t, "trigger"):
		return "trigger_failure"
	default:
		// Also the fallback for an unrecognised type string, since config
		// errors are the most common run-level failure.
		return "invalid_config"
	}
}

// jobFailureItems maps one job to zero or more failure items: an
// unauthorized_job or failed_job item for the job itself, plus a failed_test
// item for every failing test case StreamJobTests reports for it. A job that
// is neither unauthorized nor failed (e.g. still running, or succeeded)
// contributes nothing.
func jobFailureItems(ctx context.Context, client *apiclient.Client, wf apiclient.WorkflowV3, j apiclient.WorkflowJobV3) ([]failureReportItem, error) {
	refs := failureReportReferences{
		Workflow: &failureReportRef{ID: wf.ID, Attributes: failureReportRefAttrs{Name: wf.Name}},
		Job:      &failureReportRef{ID: j.ID, Attributes: failureReportRefAttrs{Name: j.Name}},
	}

	switch j.Outcome {
	case apiclient.StatusUnauthorized:
		return []failureReportItem{{
			ID:         j.ID,
			Attributes: failureReportAttributes{Kind: "unauthorized_job", Name: j.Name},
			References: refs,
		}}, nil

	case "failed":
		items := []failureReportItem{{
			ID:         j.ID,
			Attributes: failureReportAttributes{Kind: "failed_job", Name: j.Name, Message: failingStepMessage(ctx, client, j.ID)},
			References: refs,
		}}

		err := client.StreamJobTests(ctx, j.ID, func(tr apiclient.TestResult) {
			if tr.Result != "failure" {
				return
			}
			items = append(items, failureReportItem{
				ID: syntheticID(j.ID.String(), "test", tr.Classname, tr.Name),
				Attributes: failureReportAttributes{
					Kind:      "failed_test",
					Classname: tr.Classname,
					Name:      tr.Name,
					Message:   strings.TrimSpace(tr.Message),
				},
				References: refs,
			})
		})
		if err != nil && !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, apiErr(err, j.ID.String())
		}
		return items, nil

	default:
		return nil, nil
	}
}

// failingStepMessage fetches job detail and describes its first failed step,
// or "" when the job detail is unavailable (404 — a not-run/skipped job) or
// carries no failed step. Errors other than 404 from GetJobV3 are swallowed
// here rather than aborting the whole report: this is a best-effort message
// field, not required for the failed_job item to be meaningful.
func failingStepMessage(ctx context.Context, client *apiclient.Client, jobID uuid.UUID) string {
	jobDetail, err := client.GetJobV3(ctx, jobID)
	if err != nil || jobDetail == nil {
		return ""
	}
	for _, exec := range jobDetail.Executions {
		for _, step := range exec.Steps {
			if step.Outcome != "failed" {
				continue
			}
			if step.ExitCode != nil {
				return fmt.Sprintf("step %q exited %d", step.Name, *step.ExitCode)
			}
			return fmt.Sprintf("step %q failed", step.Name)
		}
	}
	return ""
}

// syntheticID derives a stable, deterministic UUID from a set of string parts.
// It exists only to give an item an "id" field to satisfy the documented
// shape when there is no underlying resource with a real UUID (a run error,
// or a test case) — it is not a real resource identifier and carries no
// meaning outside this experimental report.
func syntheticID(parts ...string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(parts, "|")))
}
