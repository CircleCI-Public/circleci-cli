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
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/jobssh"
)

const sshDocsURL = "https://circleci.com/docs/guides/execute/ssh-access-jobs/"

// sshPollInterval is how often an SSH rerun is polled for its ssh command. The
// user is waiting at the terminal, so this is shorter than run watch's.
const sshPollInterval = 3 * time.Second

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
			Reports the new workflow's ID. With --ssh, waits for each job's ssh command.

			JSON fields: workflow_id, rerun_from, from_failed, ssh, jobs[].id/name/
			rerun_job_id/ssh_commands[].execution/command/user/host/port
		`),
		Example: heredoc.Doc(`
			# Rerun all jobs in a workflow from scratch
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Rerun only the failed jobs
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b --from-failed

			# Rerun one job with SSH and print the ssh command once it starts
			$ circleci workflow rerun <workflow-id> --ssh --job run-tests
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
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "With --ssh, return without waiting for the ssh command")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 10*time.Minute, "With --ssh, how long to wait for the ssh command")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	return cmd
}

type rerunOpts struct {
	fromFailed bool
	ssh        bool
	noWait     bool
	timeout    time.Duration
	jobs       []string
}

// waitForSSH reports whether the rerun waits for the ssh commands.
func (o rerunOpts) waitForSSH() bool { return o.ssh && !o.noWait }

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
	if o.waitForSSH() && o.timeout <= 0 {
		return clierrors.New("workflow.rerun.invalid_args", "Invalid arguments",
			fmt.Sprintf("--timeout must be a positive duration such as 10m (got %s).", o.timeout)).
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

// rerunJobJSON identifies a job the rerun was limited to. ID is the job in the
// *old* workflow, the one that was named; the rerun creates a new job with a new
// ID, reported as RerunJobID once the rerun has been waited on.
type rerunJobJSON struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	RerunJobID  *uuid.UUID       `json:"rerun_job_id,omitempty"`
	SSHCommands []sshCommandJSON `json:"ssh_commands,omitempty"`
}

// sshCommandJSON is how to connect to one execution of a rerun job. User and
// Port are omitted when the command has none.
type sshCommandJSON struct {
	Execution int    `json:"execution"`
	Command   string `json:"command"`
	User      string `json:"user,omitempty"`
	Host      string `json:"host"`
	Port      int    `json:"port,omitempty"`
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
		// A failed wait still prints what it found before returning its
		// error (which goes to stderr): on a timeout with two jobs, the one
		// that connected is as usable as it is in human mode.
		var (
			conns   []jobssh.Conn
			waitErr error
		)
		if opts.waitForSSH() {
			conns, waitErr = waitForSSH(ctx, newID, client, jobs, opts, false)
		}
		out := rerunJSONOutput{
			WorkflowID: newID,
			RerunFrom:  id,
			FromFailed: opts.fromFailed,
			SSH:        opts.ssh,
		}
		for _, j := range jobs {
			out.Jobs = append(out.Jobs, rerunJob(j, conns))
		}
		if err := iostream.PrintJSON(ctx, out); err != nil {
			return err
		}
		return waitErr
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

	switch {
	case opts.waitForSSH():
		if _, err := waitForSSH(ctx, newID, client, jobs, opts, true); err != nil {
			return err
		}
		iostream.ErrPrintf(ctx, "Connect with the SSH key added to CircleCI or your VCS provider. See: %s\n", sshDocsURL)
	case opts.ssh:
		iostream.ErrPrintf(ctx, "Once the job starts, its output shows the ssh command to connect.\n")
		iostream.ErrPrintf(ctx, "Follow it with: circleci workflow get %s\n", newID)
	}
	return nil
}

// waitForSSH waits for the rerun jobs' ssh commands. With human set, it shows
// progress on stderr and prints each command on stdout as it is found.
func waitForSSH(ctx context.Context, newID string, client *apiclient.Client, jobs []apiclient.WorkflowJobV3, opts rerunOpts, human bool) ([]jobssh.Conn, error) {
	wfID, err := uuid.Parse(newID)
	if err != nil {
		return nil, clierrors.New("workflow.rerun.invalid_id", "Invalid workflow ID",
			fmt.Sprintf("The rerun reported %q, which is not a workflow ID, so its ssh command can't be looked up.", newID)).
			WithExitCode(clierrors.ExitAPIError)
	}
	names := make([]string, len(jobs))
	for i, j := range jobs {
		names[i] = j.Name
	}
	spin := &iostream.Spin{}
	defer func() { spin.Stop() }()
	waitOpts := jobssh.Options{Timeout: opts.timeout, Interval: sshPollInterval}
	if human {
		waitOpts.OnStatus = func(msg string) {
			spin.Stop()
			spin = iostream.Spinner(ctx, true, msg)
		}
		waitOpts.OnConn = func(c jobssh.Conn) {
			spin.Stop()
			label := c.Job
			if c.Executions > 1 {
				label = fmt.Sprintf("%s (execution %d)", c.Job, c.Execution)
			}
			iostream.Printf(ctx, "%s: %s\n", label, c.Command)
		}
	}

	conns, err := jobssh.Wait(ctx, client, wfID, names, waitOpts)
	if err == nil {
		return conns, nil
	}
	var timeout *jobssh.TimeoutError
	var noSSH *jobssh.NoSSHError
	switch {
	case errors.As(err, &timeout):
		return conns, clierrors.New("workflow.rerun.ssh_timeout", "Timed out waiting for SSH",
			fmt.Sprintf("No ssh command for %s within %s. The rerun is still running.", strings.Join(timeout.Pending, ", "), timeout.Timeout)).
			WithSuggestions(
				fmt.Sprintf("Check whether the job is still queued with: circleci workflow get %s", newID),
				"Wait longer next time with --timeout, e.g. --timeout 30m",
			).
			WithExitCode(clierrors.ExitTimeout)
	case errors.As(err, &noSSH):
		return conns, clierrors.New("workflow.rerun.ssh_unavailable", "No SSH access",
			fmt.Sprintf("Job %s ended without printing an ssh command, so there is nothing to connect to.", noSSH.Job)).
			WithSuggestions(fmt.Sprintf("See how the job ended with: circleci job get %s", noSSH.JobID)).
			WithRef(sshDocsURL).
			WithExitCode(clierrors.ExitGeneralError)
	case ctx.Err() != nil:
		return conns, clierrors.New("workflow.rerun.interrupted", "Stopped waiting",
			"Stopped waiting for the ssh command. The rerun is still running.").
			WithSuggestions(fmt.Sprintf("Follow it with: circleci workflow get %s", newID)).
			WithExitCode(clierrors.ExitCancelled)
	}
	return conns, apiErr(err, newID)
}

// rerunJob is the --json entry for a job the rerun was limited to, with the
// commands found for its rerun, if any.
func rerunJob(j apiclient.WorkflowJobV3, conns []jobssh.Conn) rerunJobJSON {
	out := rerunJobJSON{ID: j.ID, Name: j.Name}
	for _, c := range conns {
		if c.Job != j.Name {
			continue
		}
		id := c.JobID
		out.RerunJobID = &id
		out.SSHCommands = append(out.SSHCommands, sshCommandJSON{
			Execution: c.Execution,
			Command:   c.Command,
			User:      c.User,
			Host:      c.Host,
			Port:      c.Port,
		})
	}
	return out
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
