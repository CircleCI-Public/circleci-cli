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
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/mdtable"
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

			JSON fields: id, number, state, project_slug, branch, revision,
			             created_at, trigger
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
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
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
	ID          string `json:"id"`
	Number      int64  `json:"number"`
	State       string `json:"state"`
	ProjectSlug string `json:"project_slug"`
	Branch      string `json:"branch,omitempty"`
	Revision    string `json:"revision,omitempty"`
	CreatedAt   string `json:"created_at"`
	Trigger     struct {
		Type  string `json:"type"`
		Actor string `json:"actor"`
	} `json:"trigger"`
}

func runList(ctx context.Context, client *apiclient.Client, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci run list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	runs, err := client.ListPipelines(ctx, projectSlug, branch, limit)
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

func toListEntry(r *apiclient.Pipeline) runListEntry {
	e := runListEntry{
		ID:          r.ID,
		Number:      r.Number,
		State:       r.State,
		ProjectSlug: r.ProjectSlug,
		CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
	e.Trigger.Type = r.Trigger.Type
	e.Trigger.Actor = r.Trigger.Actor.Login
	if r.VCS != nil {
		e.Branch = r.VCS.Branch
		e.Revision = r.VCS.Revision
		if len(e.Revision) > 7 {
			e.Revision = e.Revision[:7]
		}
	}
	return e
}

func printList(ctx context.Context, entries []runListEntry) {
	table := mdtable.New("#", "Branch", "Revision", "Run", "Created", "State")
	for _, e := range entries {
		table.Row(strconv.Itoa(int(e.Number)), e.Branch, e.Revision, "`"+e.ID+"`", e.CreatedAt, e.State)
	}
	iostream.PrintMarkdown(ctx, "# Runs\n"+table.Render())
}
