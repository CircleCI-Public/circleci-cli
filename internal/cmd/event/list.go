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

package event

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
		projectSlug   string
		branch        string
		currentBranch bool
		limit         int
		jsonOut       bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent events for a project",
		Long: heredoc.Doc(`
			List recent events for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Use --branch to filter results
			to a single branch, or --current-branch (-B) to automatically use
			the branch you have checked out.

			JSON fields: id, phase, outcome, current_outcome, branch, revision, created_at
		`),
		Example: heredoc.Doc(`
			# List recent events for the current project
			$ circleci event list

			# Filter to the branch you have checked out
			$ circleci event list --current-branch

			# Filter to a specific branch
			$ circleci event list --branch main

			# List events for an explicit project
			$ circleci event list --project gh/org/repo

			# Show more results
			$ circleci event list --limit 25

			# Output as JSON for scripting
			$ circleci event list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			if currentBranch && branch == "" {
				info, err := gitremote.Detect()
				if err != nil {
					return cmdutil.GitDetectErr(err, "Or specify the branch explicitly: circleci event list --branch <name>")
				}
				branch = info.Branch
			}
			return eventList(ctx, client, projectSlug, branch, limit, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch")
	cmd.Flags().BoolVarP(&currentBranch, "current-branch", "B", false, "Filter by the currently checked-out branch")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of events to show [default: 10]")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type eventListEntry struct {
	ID             string `json:"id"`
	Phase          string `json:"phase"`
	Outcome        string `json:"outcome,omitempty"`
	CurrentOutcome string `json:"current_outcome,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Revision       string `json:"revision,omitempty"`
	CreatedAt      string `json:"created_at"`
}

func eventList(ctx context.Context, client *apiclient.Client, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci event list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectInfo(ctx, projectSlug)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	now := time.Now().UTC()
	events, err := client.SearchEvents(ctx, apiclient.EventSearchParams{
		ProjectIDs: []string{proj.ID},
		From:       now.AddDate(0, 0, -90),
		To:         now,
		Filter:     apiclient.BuildEventFilter(branch, ""),
		Limit:      limit,
	})
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entries := make([]eventListEntry, len(events))
	for i, r := range events {
		entries[i] = toListEntry(&r)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(events) == 0 {
		iostream.ErrPrintln(ctx, "No events found.")
		return nil
	}

	printList(ctx, entries)
	return nil
}

func toListEntry(r *apiclient.Event) eventListEntry {
	rev := r.Revision
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return eventListEntry{
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Revision:       rev,
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
}

func printList(ctx context.Context, entries []eventListEntry) {
	table := mdtable.New("Branch", "Revision", "ID", "Created", "Status")
	for _, e := range entries {
		table.Row(e.Branch, e.Revision, "`"+e.ID+"`", e.CreatedAt, apiclient.PhaseOutcomeStatus(e.Phase, e.Outcome, e.CurrentOutcome))
	}
	iostream.PrintMarkdown(ctx, "# Events\n"+table.Render())
}
