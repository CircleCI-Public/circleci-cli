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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
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
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent runs for a project",
		Long: heredoc.Doc(`
			List recent runs for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Use --branch to filter results
			to a single branch.

			JSON fields: id, status, branch, revision, created_at
		`),
		Example: heredoc.Doc(`
			# List recent runs for the current project
			$ circleci run list

			# Filter to a specific branch
			$ circleci run list --branch main

			# List runs for an explicit project
			$ circleci run list --project gh/org/repo

			# Show more results
			$ circleci run list --limit 25

			# Output as JSON for scripting
			$ circleci run list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runList(ctx, client, projectSlug, branch, limit, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of runs to show [default: 10]")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runListEntry struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Branch    string `json:"branch,omitempty"`
	Revision  string `json:"revision,omitempty"`
	CreatedAt string `json:"created_at"`
}

func runList(ctx context.Context, client *apiclient.Client, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci run list --project gh/org/repo")
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

	entries := make([]runListEntry, len(runs))
	for i, r := range runs {
		entries[i] = toListEntry(&r)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(runs) == 0 {
		iostream.ErrPrintln(ctx, "No runs found.")
		return nil
	}

	printList(ctx, entries)
	return nil
}

func toListEntry(r *apiclient.RunV3) runListEntry {
	rev := r.Revision
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return runListEntry{
		ID:        r.ID,
		Status:    r.Status,
		Branch:    r.Branch,
		Revision:  rev,
		CreatedAt: r.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
}

func printList(ctx context.Context, entries []runListEntry) {
	table := mdtable.New("Branch", "Revision", "ID", "Created", "Status")
	for _, e := range entries {
		table.Row(e.Branch, e.Revision, "`"+e.ID+"`", e.CreatedAt, e.Status)
	}
	iostream.PrintMarkdown(ctx, "# Runs\n"+table.Render())
}
