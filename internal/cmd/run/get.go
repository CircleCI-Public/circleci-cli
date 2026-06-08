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
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
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
		Use:   "get [<run-id>]",
		Short: "Get a run's status",
		Long: heredoc.Doc(`
			Display the status of a CircleCI run and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch.

			Pass a run UUID to look up a specific run.

			JSON fields: id, status, branch, revision, created_at,
			             errors[].type/message,
			             workflows[].id/name/status/duration/jobs[].id/name/status/type
		`),
		Example: heredoc.Doc(`
			# Get the latest run for the current branch
			$ circleci run get

			# Get a run by UUID
			$ circleci run get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci run get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used for latest-run lookup")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runGetOutput struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	Branch    string           `json:"branch,omitempty"`
	Revision  string           `json:"revision,omitempty"`
	CreatedAt string           `json:"created_at"`
	Errors    []errorOutput    `json:"errors,omitempty"`
	Workflows []workflowOutput `json:"workflows"`
}

type errorOutput struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type workflowOutput struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Status   string      `json:"status"`
	Duration string      `json:"duration,omitempty"`
	Jobs     []jobOutput `json:"jobs"`
}

type jobOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type,omitempty"`
}

func runGet(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string, jsonOut bool) error {
	var (
		r   *apiclient.RunV3
		err error
	)

	if len(args) == 1 {
		r, err = client.GetRunV3(ctx, args[0])
		if err != nil {
			return apiErr(err, args[0])
		}
	} else {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide a run UUID: circleci run get <uuid>")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		proj, err := client.GetProjectInfo(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}

		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching latest run for %s on branch %s", projectSlug, effectiveBranch))
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildRunFilter(effectiveBranch, ""),
			Limit:      1,
		})
		sp.Stop()
		if searchErr != nil {
			return apiErr(searchErr, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		if len(runs) == 0 {
			return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		r = &runs[0]
	}

	// Fetch workflows via V2 using the run ID (which is the pipeline ID).
	workflows, err := client.GetPipelineWorkflows(ctx, r.ID)
	if err != nil {
		return apiErr(err, r.ID)
	}

	wfJobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
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

func buildOutput(r *apiclient.RunV3, workflows []apiclient.PipelineWorkflowSummary, wfJobs [][]apiclient.WorkflowJobV3) runGetOutput {
	wflows := make([]workflowOutput, len(workflows))
	for i, w := range workflows {
		jobs := make([]jobOutput, 0, len(wfJobs[i]))
		for _, j := range wfJobs[i] {
			jobs = append(jobs, jobOutput{
				ID:     j.ID,
				Name:   j.Name,
				Status: j.Status,
				Type:   j.Type,
			})
		}
		var dur string
		if w.StoppedAt != nil {
			dur = formatElapsed(w.StoppedAt.Sub(w.CreatedAt))
		}
		wflows[i] = workflowOutput{ID: w.ID, Name: w.Name, Status: w.Status, Duration: dur, Jobs: jobs}
	}

	revision := r.Revision
	if len(revision) > 7 {
		revision = revision[:7]
	}

	errs := make([]errorOutput, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = errorOutput{Type: e.Type, Message: e.Message}
	}

	return runGetOutput{
		ID:        r.ID,
		Status:    deriveStatus(r.Status, wflows),
		Branch:    r.Branch,
		Revision:  revision,
		CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		Errors:    errs,
		Workflows: wflows,
	}
}

// deriveStatus computes a meaningful overall status from workflow statuses.
func deriveStatus(runStatus string, workflows []workflowOutput) string {
	if len(workflows) == 0 {
		return runStatus
	}
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
	return runStatus
}

func printRun(ctx context.Context, r runGetOutput) {
	var md strings.Builder
	md.WriteString("# Run\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", r.ID)
	if r.Branch != "" {
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
	}
	if r.Revision != "" {
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", r.Revision)
	}
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", r.Status)
	_, _ = fmt.Fprintf(&md, "- Created: %s\n", r.CreatedAt)

	if len(r.Errors) > 0 {
		md.WriteString("\n## Errors\n")
		for _, e := range r.Errors {
			_, _ = fmt.Fprintf(&md, "- **%s**: %s\n", e.Type, e.Message)
		}
	}
	md.WriteString("\n")

	if len(r.Workflows) > 0 {
		md.WriteString("## Workflows\n")
		for _, w := range r.Workflows {
			_, _ = fmt.Fprintf(&md, "### %s\n", w.Name)
			_, _ = fmt.Fprintf(&md, "- Status: %s\n", w.Status)
			if w.Duration != "" {
				_, _ = fmt.Fprintf(&md, "- Duration: %s\n", w.Duration)
			}
			_, _ = fmt.Fprintf(&md, "- Jobs:\n")
			for _, j := range w.Jobs {
				_, _ = fmt.Fprintf(&md, "  - %-36s  %-10s  %-10s  %s\n", j.Name, j.Status, j.Type, j.ID)
			}
		}
		md.WriteString("\n")
	}

	iostream.PrintMarkdown(ctx, md.String())
}
