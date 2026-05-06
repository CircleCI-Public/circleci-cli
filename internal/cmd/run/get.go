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
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

const (
	statusCanceled = "canceled"
	statusSuccess  = "success"
)

func newGetCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get [<run-id-or-number>]",
		Short: "Get a run's status",
		Long: heredoc.Doc(`
			Display the status of a CircleCI run and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch.

			Pass a run number (e.g. 75) or UUID to look up a specific run.
			When using a number, the project is inferred from the git remote unless
			overridden with --project.

			JSON fields: id, number, status, project_slug, branch, revision,
			             created_at, updated_at, trigger,
			             workflows[].id/name/status/jobs[].number/name/status/type
		`),
		Example: heredoc.Doc(`
			# Get the latest run for the current branch
			$ circleci run get

			# Get a run by number
			$ circleci run get 75

			# Get a run by UUID
			$ circleci run get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci run get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used when looking up by number")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// runGetOutput is the JSON shape returned by this command.
type runGetOutput struct {
	ID          string           `json:"id"`
	Number      int64            `json:"number"`
	Status      string           `json:"status"`
	ProjectSlug string           `json:"project_slug"`
	Branch      string           `json:"branch,omitempty"`
	Revision    string           `json:"revision,omitempty"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	Trigger     triggerOutput    `json:"trigger"`
	Errors      []errorOutput    `json:"errors,omitempty"`
	Workflows   []workflowOutput `json:"workflows"`
}

type errorOutput struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type triggerOutput struct {
	Type  string `json:"type"`
	Actor string `json:"actor"`
}

type workflowOutput struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Jobs   []jobOutput `json:"jobs"`
}

type jobOutput struct {
	Number int64  `json:"number,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

func runGet(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string, jsonOut bool) error {
	var (
		err error
		r   *apiclient.Pipeline
	)

	if len(args) == 1 {
		arg := args[0]
		if looksLikeNumber(arg) {
			// Run number: need a project slug to resolve it.
			number, _ := strconv.ParseInt(arg, 10, 64)
			if projectSlug == "" {
				info, err := gitremote.Detect()
				if err != nil {
					return cmdutil.GitDetectErr(err, "Or specify the project: circleci run get "+arg+" --project gh/org/repo")
				}
				projectSlug = info.Slug
			}
			r, err = client.GetPipelineByNumber(ctx, projectSlug, number)
			if err != nil {
				return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, arg))
			}
		} else {
			// UUID
			r, err = client.GetPipeline(ctx, arg)
			if err != nil {
				return apiErr(err, arg)
			}
		}
	} else {
		// No arg: infer from git context.
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide a run number or UUID: circleci run get <number>")
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching latest run for %s on branch %s", info.Slug, effectiveBranch))
		r, err = client.GetLatestPipeline(ctx, info.Slug, effectiveBranch)
		sp.Stop()
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", info.Slug, effectiveBranch))
		}
	}

	workflows, err := client.GetPipelineWorkflows(ctx, r.ID)
	if err != nil {
		return apiErr(err, r.ID)
	}

	wfJobs := make([][]apiclient.WorkflowJob, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID)
		}
		wfJobs[i] = jobs
	}

	out := buildOutput(r, workflows, wfJobs)

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	printRun(ctx, out)
	return nil
}

func buildOutput(r *apiclient.Pipeline, workflows []apiclient.PipelineWorkflowSummary, wfJobs [][]apiclient.WorkflowJob) runGetOutput {
	wflows := make([]workflowOutput, len(workflows))
	for i, w := range workflows {
		jobs := make([]jobOutput, 0, len(wfJobs[i]))
		for _, j := range wfJobs[i] {
			jobs = append(jobs, jobOutput{
				Number: j.JobNumber,
				Name:   j.Name,
				Status: j.Status,
				Type:   j.Type,
			})
		}
		wflows[i] = workflowOutput{ID: w.ID, Name: w.Name, Status: w.Status, Jobs: jobs}
	}

	var branch, revision string
	if r.VCS != nil {
		branch = r.VCS.Branch
		revision = r.VCS.Revision
		if len(revision) > 7 {
			revision = revision[:7]
		}
	}

	errs := make([]errorOutput, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = errorOutput{Type: e.Type, Message: e.Message}
	}

	return runGetOutput{
		ID:          r.ID,
		Number:      r.Number,
		Status:      deriveStatus(r.State, wflows),
		ProjectSlug: r.ProjectSlug,
		Branch:      branch,
		Revision:    revision,
		CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02 15:04:05 UTC"),
		Trigger:     triggerOutput{Type: r.Trigger.Type, Actor: r.Trigger.Actor.Login},
		Errors:      errs,
		Workflows:   wflows,
	}
}

// deriveStatus computes a meaningful overall status from workflow statuses.
// The run-level state from the API reflects creation/setup lifecycle
// (almost always "created") and is not useful as an execution status.
func deriveStatus(runState string, workflows []workflowOutput) string {
	// A run-level error (e.g. config error) takes priority.
	if runState == "errored" {
		return "errored"
	}
	if len(workflows) == 0 {
		return runState
	}
	// Walk workflows in priority order: failure > running > on_hold > canceled > success.
	for _, wf := range workflows {
		switch wf.Status {
		case "failed", "error", "failing":
			return "failed"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "running" {
			return "running"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "on_hold" {
			return "on_hold"
		}
	}
	for _, wf := range workflows {
		if wf.Status == statusCanceled {
			return statusCanceled
		}
	}
	for _, wf := range workflows {
		if wf.Status == statusSuccess {
			return statusSuccess
		}
	}
	return runState
}

// looksLikeNumber returns true if s is a plain positive integer (run number),
// as opposed to a UUID (which contains hyphens).
func looksLikeNumber(s string) bool {
	return !strings.Contains(s, "-") && len(s) > 0
}

func printRun(ctx context.Context, r runGetOutput) {
	var md strings.Builder
	md.WriteString("# Run\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", r.ID)
	_, _ = fmt.Fprintf(&md, "- Number: %d\n", r.Number)
	_, _ = fmt.Fprintf(&md, "- Project: %s\n", r.ProjectSlug)
	if r.Branch != "" {
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
	}
	if r.Revision != "" {
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", r.Revision)
	}
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", r.Status)
	md.WriteString("\n")

	md.WriteString("## Trigger\n")
	_, _ = fmt.Fprintf(&md, "- Created At: %s\n", r.CreatedAt)
	_, _ = fmt.Fprintf(&md, "- By: %s\n", r.Trigger.Actor)
	_, _ = fmt.Fprintf(&md, "- Type: %s\n", r.Trigger.Type)
	md.WriteString("\n")

	if len(r.Errors) > 0 {
		md.WriteString("## Errors\n")
		for _, e := range r.Errors {
			_, _ = fmt.Fprintf(&md, "- [%s] %s\n", e.Type, e.Message)
		}
		md.WriteString("\n")
	}

	if len(r.Workflows) > 0 {
		md.WriteString("## Workflows\n")
		for _, w := range r.Workflows {
			_, _ = fmt.Fprintf(&md, "### %s\n", w.Name)
			_, _ = fmt.Fprintf(&md, "- Status: %s\n", w.Status)
			_, _ = fmt.Fprintf(&md, "- Jobs:\n")
			for _, j := range w.Jobs {
				if j.Type == "approval" {
					_, _ = fmt.Fprintf(&md, "  - %-36s  %s\n", j.Name, j.Status)
				} else {
					_, _ = fmt.Fprintf(&md, "  - %-36s  %s  #%d\n", j.Name, j.Status, j.Number)
				}
			}
		}
		md.WriteString("\n")
	}

	iostream.PrintMarkdown(ctx, md.String())
}
