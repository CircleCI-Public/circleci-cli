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

package workflow

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

const sshDocsURL = "https://circleci.com/docs/guides/execute/ssh-access-jobs/"

func newRerunCmd() *cobra.Command {
	var (
		opts    rerunOpts
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "rerun <workflow-id>",
		Short: "Rerun a workflow",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<workflow-id>%[1]s is the UUID of the workflow to rerun. Workflow IDs are
				shown in the output of %[1]scircleci run get%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			A rerun creates a new workflow; its ID is reported so you can follow it.

			JSON fields: workflow_id, rerun_from, from_failed, ssh, jobs[].id/name
		`),
		Example: heredoc.Doc(`
			# Rerun all jobs in a workflow from scratch
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Rerun only the failed jobs
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b --from-failed

			# Rerun one job with SSH access, by name
			$ circleci workflow rerun <workflow-id> --ssh --job run-tests

			# Rerun two jobs by job ID (see: circleci workflow get <workflow-id>)
			$ circleci workflow rerun <workflow-id> --job <job-id> --job <job-id>
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			// Validate before loading a client: a bad flag combination should
			// fail the same way whether or not the user is logged in.
			if err := opts.validate(args[0]); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runRerun(ctx, client, args[0], opts, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&opts.fromFailed, "from-failed", false, "Rerun only failed jobs")
	cmd.Flags().StringArrayVar(&opts.jobs, "job", nil, "Rerun only this job, by name or ID (repeatable)")
	cmd.Flags().BoolVar(&opts.ssh, "ssh", false, "Enable SSH access to the rerun jobs (requires --job)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	return cmd
}

type rerunOpts struct {
	fromFailed bool
	ssh        bool
	jobs       []string
}

// validate rejects flag combinations the API would refuse, before any request.
// The relationships are checked by value rather than with cobra's flag groups:
// MCP calls set every flag explicitly, and cobra counts an explicit false or
// empty value as "set".
func (o rerunOpts) validate(workflowID string) error {
	switch {
	case o.ssh && o.fromFailed:
		return clierrors.New("workflow.rerun.invalid_args", "Invalid arguments",
			"--ssh and --from-failed can't be used together: SSH is enabled on the jobs named with --job, and --from-failed picks the jobs itself.").
			WithSuggestions(fmt.Sprintf("Name the failed jobs instead: circleci workflow rerun %s --ssh --job <name>", workflowID)).
			WithExitCode(clierrors.ExitBadArguments)
	case len(o.jobs) > 0 && o.fromFailed:
		return clierrors.New("workflow.rerun.invalid_args", "Invalid arguments",
			"--job and --from-failed can't be used together: each picks which jobs to rerun.").
			WithExitCode(clierrors.ExitBadArguments)
	case o.ssh && len(o.jobs) == 0:
		return clierrors.New("workflow.rerun.ssh_requires_job", "SSH rerun needs a job",
			"--ssh needs at least one --job: SSH is enabled on the jobs you name, not on a whole workflow.").
			WithSuggestions(
				fmt.Sprintf("List the workflow's jobs with: circleci workflow get %s", workflowID),
				fmt.Sprintf("Then rerun one with: circleci workflow rerun %s --ssh --job <name>", workflowID),
			).
			WithRef(sshDocsURL).
			WithExitCode(clierrors.ExitBadArguments)
	}
	for _, j := range o.jobs {
		if strings.TrimSpace(j) == "" {
			return clierrors.New("workflow.rerun.invalid_args", "Invalid arguments",
				"--job needs a job name or job ID (got an empty value).").
				WithExitCode(clierrors.ExitBadArguments)
		}
	}
	return nil
}

// rerunJSONOutput is the --json shape. rerun_from is echoed back because the new
// workflow ID alone does not say what it came from.
type rerunJSONOutput struct {
	WorkflowID string         `json:"workflow_id"`
	RerunFrom  string         `json:"rerun_from"`
	FromFailed bool           `json:"from_failed"`
	SSH        bool           `json:"ssh"`
	Jobs       []rerunJobJSON `json:"jobs,omitempty"`
}

// rerunJobJSON identifies a job the rerun was limited to, in the *old* workflow.
// The rerun creates new jobs with new IDs; these are the ones that were named.
type rerunJobJSON struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func runRerun(ctx context.Context, client *apiclient.Client, id string, opts rerunOpts, jsonOut bool) error {
	var jobs []apiclient.WorkflowJobV3
	if len(opts.jobs) > 0 {
		wfID, err := uuid.Parse(id)
		if err != nil {
			return clierrors.New("workflow.rerun.invalid_id", "Invalid workflow ID",
				fmt.Sprintf("%q is not a workflow ID: --job needs the workflow's UUID to look its jobs up.", id)).
				WithSuggestions("Find workflow IDs with: circleci run get").
				WithExitCode(clierrors.ExitBadArguments)
		}
		all, err := client.GetWorkflowJobsV3(ctx, wfID)
		if err != nil {
			return apiErr(err, id)
		}
		jobs, err = resolveJobs(id, opts.jobs, all)
		if err != nil {
			return err
		}
		if opts.ssh {
			if err := checkSSHAllowed(ctx, client, jobs[0].ProjectID); err != nil {
				return err
			}
		}
	}

	jobIDs := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		jobIDs[i] = j.ID
	}
	newID, err := client.RerunWorkflow(ctx, id, apiclient.RerunOptions{
		FromFailed: opts.fromFailed,
		EnableSSH:  opts.ssh,
		Jobs:       jobIDs,
	})
	if err != nil {
		return apiErr(err, id)
	}

	if jsonOut {
		out := rerunJSONOutput{
			WorkflowID: newID,
			RerunFrom:  id,
			FromFailed: opts.fromFailed,
			SSH:        opts.ssh,
		}
		for _, j := range jobs {
			out.Jobs = append(out.Jobs, rerunJobJSON{ID: j.ID, Name: j.Name})
		}
		return iostream.PrintJSON(ctx, out)
	}

	// Lead with the new workflow's ID rather than the one that was passed in: the
	// old one is spent, and the new one is what `workflow get` or `run watch`
	// needs next.
	switch {
	case opts.fromFailed:
		iostream.Printf(ctx, "Rerunning failed jobs from %s as workflow %s\n", id, newID)
	case len(jobs) > 0:
		with := ""
		if opts.ssh {
			with = " with SSH"
		}
		iostream.Printf(ctx, "Rerunning %s%s from %s as workflow %s\n", jobNames(jobs), with, id, newID)
	default:
		iostream.Printf(ctx, "Rerunning %s from scratch as workflow %s\n", id, newID)
	}

	if opts.ssh {
		// The SSH command only exists once the job has started, and appears in
		// the job's output. Waiting for it and printing it is not done yet, so
		// point at where it will be.
		iostream.ErrPrintf(ctx, "Once the job starts, its output shows the ssh command to connect.\n")
		iostream.ErrPrintf(ctx, "Follow it with: circleci workflow get %s\n", newID)
	}
	return nil
}

// resolveJobs maps each --job value to a job in the workflow. A value that
// parses as a UUID is matched against job IDs, anything else against job names.
// IDs are checked against the workflow too, so a job from another workflow
// fails here with a clear message rather than at the API. Repeated values
// resolve to one job.
func resolveJobs(workflowID string, values []string, jobs []apiclient.WorkflowJobV3) ([]apiclient.WorkflowJobV3, error) {
	var out []apiclient.WorkflowJobV3
	seen := map[uuid.UUID]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		i := slices.IndexFunc(jobs, func(j apiclient.WorkflowJobV3) bool {
			if id, err := uuid.Parse(v); err == nil {
				return j.ID == id
			}
			return j.Name == v
		})
		if i < 0 {
			return nil, jobNotFound(workflowID, v, jobs)
		}
		if !seen[jobs[i].ID] {
			seen[jobs[i].ID] = true
			out = append(out, jobs[i])
		}
	}
	return out, nil
}

func jobNotFound(workflowID, value string, jobs []apiclient.WorkflowJobV3) error {
	kind := "named"
	if _, err := uuid.Parse(value); err == nil {
		kind = "with ID"
	}
	suggestions := []string{fmt.Sprintf("List the workflow's jobs with: circleci workflow get %s", workflowID)}
	if len(jobs) > 0 {
		suggestions = append([]string{"Jobs in this workflow: " + jobNames(jobs)}, suggestions...)
	}
	return clierrors.New("workflow.rerun.job_not_found", "Job not found",
		fmt.Sprintf("Workflow %s has no job %s %q.", workflowID, kind, value)).
		WithSuggestions(suggestions...).
		WithExitCode(clierrors.ExitNotFound)
}

// checkSSHAllowed fails early when the project has SSH reruns turned off, which
// the API would otherwise reject with a less specific error. If the settings
// can't be read (a user may rerun without being allowed to read them), the
// rerun goes ahead and the API decides.
func checkSSHAllowed(ctx context.Context, client *apiclient.Client, projectID uuid.UUID) error {
	if projectID == uuid.Nil {
		return nil
	}
	settings, err := client.GetProjectSettings(ctx, projectID)
	if err != nil || !settings.DisableSSH {
		return nil
	}
	return clierrors.New("workflow.rerun.ssh_disabled", "SSH disabled for project",
		"This project has SSH access to jobs turned off, so the job can't be rerun with SSH.").
		WithSuggestions(
			"Check the setting with: circleci project setting get disable-ssh",
			"A project admin can turn it back on with: circleci project setting set disable-ssh false",
		).
		WithRef(sshDocsURL).
		WithExitCode(clierrors.ExitGeneralError)
}

func jobNames(jobs []apiclient.WorkflowJobV3) string {
	names := make([]string, len(jobs))
	for i, j := range jobs {
		names[i] = j.Name
	}
	return strings.Join(names, ", ")
}
