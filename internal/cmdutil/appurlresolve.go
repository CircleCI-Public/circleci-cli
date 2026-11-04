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

package cmdutil

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// ResolveRunURL turns a CircleCI web URL into the UUID of the run it belongs
// to, with a short name for the run to show in messages. It walks up from
// whatever the URL names: a run id is returned as is, a workflow is fetched for
// its run, a pipeline number is looked up in its project, and a job is fetched
// for its pipeline. Errors from the API are returned unwrapped so the caller
// can map them with APIErr.
func ResolveRunURL(ctx context.Context, client *apiclient.Client, raw string) (uuid.UUID, string, error) {
	ref, err := ParseAppURL(raw)
	if err != nil {
		return uuid.Nil, "", invalidURLErr(raw, err, "run")
	}
	switch {
	case ref.RunID != uuid.Nil:
		return ref.RunID, ref.RunID.String(), nil
	case ref.PipelineNumber != 0:
		p, err := client.GetPipelineByNumber(ctx, ref.ProjectSlug, ref.PipelineNumber)
		if err != nil {
			return uuid.Nil, "", err
		}
		return p.ID, fmt.Sprintf("#%d", p.Number), nil
	case ref.WorkflowID != uuid.Nil:
		wf, err := client.GetWorkflowV3(ctx, ref.WorkflowID)
		if err != nil {
			return uuid.Nil, "", err
		}
		return wf.RunID, wf.RunID.String(), nil
	case ref.JobID != uuid.Nil:
		j, err := client.GetJobV3(ctx, ref.JobID)
		if err != nil {
			return uuid.Nil, "", err
		}
		return j.PipelineID, j.PipelineID.String(), nil
	case ref.JobNumber != 0:
		j, err := client.GetJobByNumber(ctx, ref.ProjectSlug, ref.JobNumber)
		if err != nil {
			return uuid.Nil, "", err
		}
		return j.Pipeline.ID, j.Pipeline.ID.String(), nil
	}
	return uuid.Nil, "", invalidURLErr(raw, fmt.Errorf("the URL names no run, workflow or job"), "run")
}

// ResolveJobRef turns a <job-id> argument into a job UUID. A UUID passes
// through; a CircleCI web URL is resolved to the job it names. A job page URL
// carries the job UUID directly. A URL with only a job number is looked up in
// its project, then matched by name in its workflow's job list, because the
// number endpoint does not return the UUID.
func ResolveJobRef(ctx context.Context, client *apiclient.Client, arg string) (uuid.UUID, error) {
	if !LooksLikeURL(arg) {
		id, err := uuid.Parse(arg)
		if err != nil {
			return uuid.Nil, clierrors.New("args.invalid_job_id", "Invalid job ID",
				fmt.Sprintf("%q is not a valid job UUID or CircleCI job URL.", arg)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		return id, nil
	}
	ref, err := ParseAppURL(arg)
	if err != nil {
		return uuid.Nil, invalidURLErr(arg, err, "job")
	}
	if ref.JobID != uuid.Nil {
		return ref.JobID, nil
	}
	if ref.JobNumber == 0 {
		return uuid.Nil, invalidURLErr(arg, fmt.Errorf("the URL names a run or workflow, not a job"), "job")
	}

	subject := fmt.Sprintf("%s #%d", ref.ProjectSlug, ref.JobNumber)
	j, err := client.GetJobByNumber(ctx, ref.ProjectSlug, ref.JobNumber)
	if err != nil {
		return uuid.Nil, APIErr(err, subject, "job.not_found", "No job found for %q.")
	}
	workflowID := ref.WorkflowID
	if workflowID == uuid.Nil {
		workflowID = j.LatestWorkflow.ID
	}
	jobs, err := client.GetWorkflowJobsV3(ctx, workflowID)
	if err != nil {
		return uuid.Nil, APIErr(err, subject, "job.not_found", "No job found for %q.")
	}
	var matches []uuid.UUID
	for _, wj := range jobs {
		if wj.Name == j.Name {
			matches = append(matches, wj.ID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return uuid.Nil, clierrors.New("job.not_found", "Job not found",
			fmt.Sprintf("Job %q (#%d) is not in workflow %s.", j.Name, j.Number, workflowID)).
			WithSuggestions("List the workflow's jobs: circleci workflow get " + workflowID.String()).
			WithExitCode(clierrors.ExitNotFound)
	default:
		return uuid.Nil, clierrors.New("job.ambiguous", "Ambiguous job",
			fmt.Sprintf("Workflow %s has %d jobs named %q; the job number alone cannot tell them apart.", workflowID, len(matches), j.Name)).
			WithSuggestions("Pick the job UUID from: circleci workflow get " + workflowID.String()).
			WithExitCode(clierrors.ExitBadArguments)
	}
}

func invalidURLErr(raw string, cause error, kind string) *clierrors.CLIError {
	return clierrors.New("args.invalid_url", "Invalid CircleCI URL",
		fmt.Sprintf("%q is not a CircleCI %s URL: %v.", raw, kind, cause)).
		WithSuggestions("Pass a UUID, or copy the URL from the CircleCI app's pipeline, workflow or job page").
		WithExitCode(clierrors.ExitBadArguments)
}
