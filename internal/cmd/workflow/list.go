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
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newListCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		limit       int
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list [<run-id>]",
		Aliases: []string{"ls"},
		Short:   "List workflows for a run or recent runs",
		Long: heredoc.Doc(`
			List workflows for a CircleCI run.

			With no argument, lists workflows for recent runs in the current
			project, grouped by run. Use --branch to filter to a specific branch
			and --limit to control how many runs are shown.

			Pass a run UUID or run number to list workflows for a single
			run. Run numbers are shown in 'circleci run list'; UUIDs
			are shown in 'circleci run list --json'.

			When passing a run number, the project is inferred from the
			current git repository unless overridden with --project.

			JSON fields (single run):  id, name, status
			JSON fields (recent runs): run_id, id, name, status
		`),
		Example: heredoc.Doc(`
			# List workflows for recent runs in the current project
			$ circleci workflow list

			# Filter to a specific branch
			$ circleci workflow list --branch main

			# List workflows by run UUID
			$ circleci workflow list 9e0c9d52-3b7e-4cd6-b5f7-bfc5e4a07e81

			# List workflows by run number
			$ circleci workflow list 75

			# List workflows for a run in a specific project
			$ circleci workflow list 75 --project gh/myorg/myrepo

			# Output as JSON
			$ circleci workflow list --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return runListRecent(ctx, client, projectSlug, branch, limit, jsonOut)
			}
			return runList(ctx, client, args[0], projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch (recent-runs mode)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of recent runs to show (recent-runs mode)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type workflowListOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type workflowRecentOutput struct {
	RunID  string `json:"run_id"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func runList(ctx context.Context, client *apiclient.Client, arg, projectSlug string, jsonOut bool) error {
	runID, err := resolveRunID(ctx, client, arg, projectSlug)
	if err != nil {
		return err
	}

	workflows, err := client.GetRunWorkflowsV3(ctx, runID)
	if err != nil {
		return apiErr(err, runID)
	}

	var out []workflowListOutput
	for _, wf := range workflows {
		out = append(out, workflowListOutput{
			ID:     wf.ID,
			Name:   wf.Name,
			Status: wf.Status,
		})
	}

	if jsonOut {
		if out == nil {
			out = []workflowListOutput{}
		}
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.Printf(ctx, "No workflows found for run %s.\n", arg)
		return nil
	}

	table := mdtable.New("ID", "Name", "Status")
	for _, wf := range out {
		table.Row(wf.ID, wf.Name, wf.Status)
	}
	iostream.PrintMarkdown(ctx, "# Workflows\n"+table.Render())
	return nil
}

func runListRecent(ctx context.Context, client *apiclient.Client, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, gitErr := gitremote.Detect()
		if gitErr != nil {
			return cmdutil.GitDetectErr(gitErr, "Or specify --project explicitly")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectInfo(ctx, projectSlug)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	now := time.Now().UTC()
	runs, err := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
		ProjectIDs: []string{proj.ID},
		From:       now.AddDate(0, 0, -90),
		To:         now,
		Filter:     apiclient.BuildRunFilter(branch, ""),
		Limit:      limit,
	})
	if err != nil {
		return apiErr(err, projectSlug)
	}

	if jsonOut {
		var out []workflowRecentOutput
		for _, r := range runs {
			workflows, wErr := client.GetRunWorkflowsV3(ctx, r.ID)
			if wErr != nil {
				return apiErr(wErr, r.ID)
			}
			for _, wf := range workflows {
				out = append(out, workflowRecentOutput{
					RunID:  r.ID,
					ID:     wf.ID,
					Name:   wf.Name,
					Status: wf.Status,
				})
			}
		}
		if out == nil {
			out = []workflowRecentOutput{}
		}
		return iostream.PrintJSON(ctx, out)
	}

	if len(runs) == 0 {
		iostream.Printf(ctx, "No runs found for project %s.\n", projectSlug)
		return nil
	}

	var md strings.Builder
	md.WriteString("# Recent runs\n")

	for _, r := range runs {
		revision := r.Revision
		if len(revision) > 7 {
			revision = revision[:7]
		}
		_, _ = fmt.Fprintf(&md, "## Run %s\n", r.ID)
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", revision)

		workflows, wErr := client.GetRunWorkflowsV3(ctx, r.ID)
		if wErr != nil {
			return apiErr(wErr, r.ID)
		}

		if len(workflows) == 0 {
			_, _ = fmt.Fprintf(&md, "- Workflows: none\n")
			continue
		}
		_, _ = fmt.Fprintf(&md, "### Workflows\n")
		table := mdtable.New("ID", "Name", "Status")
		for _, wf := range workflows {
			table.Row(wf.ID, wf.Name, wf.Status)
		}
		md.WriteString(table.Render())
		md.WriteString("\n")
	}
	iostream.PrintMarkdown(ctx, md.String())
	return nil
}

// resolveRunID returns a run UUID from either a UUID string or a
// run number (requires project slug resolution from git if not provided).
func resolveRunID(ctx context.Context, client *apiclient.Client, arg, projectSlug string) (string, error) {
	if strings.Contains(arg, "-") {
		return arg, nil
	}

	number, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return "", clierrors.New("args.invalid_run_id", "Invalid run ID",
			"Expected a run UUID or run number, got: "+arg).
			WithSuggestions("Use 'circleci run list' to find run IDs and numbers").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if projectSlug == "" {
		info, gitErr := gitremote.Detect()
		if gitErr != nil {
			return "", cmdutil.GitDetectErr(gitErr, "Or specify --project explicitly")
		}
		projectSlug = info.Slug
	}

	r, rErr := client.GetPipelineByNumber(ctx, projectSlug, number)
	if rErr != nil {
		return "", apiErr(rErr, arg)
	}
	return r.ID, nil
}
