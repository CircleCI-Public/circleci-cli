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

package pipeline

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
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
		Short:   "List recent pipelines for a project",
		Long: heredoc.Doc(`
			List recent pipelines for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Use --branch to filter results
			to a single branch.

			JSON fields: id, number, state, project_slug, branch, revision,
			             created_at, trigger
		`),
		Example: heredoc.Doc(`
			# List recent pipelines for the current project
			$ circleci pipeline list

			# Filter to a specific branch
			$ circleci pipeline list --branch main

			# List pipelines for an explicit project
			$ circleci pipeline list --project gh/org/repo

			# Show more results
			$ circleci pipeline list --limit 25

			# Output as JSON for scripting
			$ circleci pipeline list --json
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
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of pipelines to show [default: 10]")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

type pipelineListEntry struct {
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
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci pipeline list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	pipelines, err := client.ListPipelines(ctx, projectSlug, branch, limit)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entries := make([]pipelineListEntry, len(pipelines))
	for i, p := range pipelines {
		entries[i] = pipelineToListEntry(&p)
	}

	if jsonOut {
		return cmdutil.WriteJSON(iostream.Out(ctx), entries)
	}

	if len(pipelines) == 0 {
		iostream.ErrPrintln(ctx, "No pipelines found.")
		return nil
	}

	printList(ctx, entries)
	return nil
}

func pipelineToListEntry(p *apiclient.Pipeline) pipelineListEntry {
	e := pipelineListEntry{
		ID:          p.ID,
		Number:      p.Number,
		State:       p.State,
		ProjectSlug: p.ProjectSlug,
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
	e.Trigger.Type = p.Trigger.Type
	e.Trigger.Actor = p.Trigger.Actor.Login
	if p.VCS != nil {
		e.Branch = p.VCS.Branch
		e.Revision = p.VCS.Revision
		if len(e.Revision) > 7 {
			e.Revision = e.Revision[:7]
		}
	}
	return e
}

func printList(ctx context.Context, entries []pipelineListEntry) {
	for _, e := range entries {
		state := ""
		if e.State == "errored" {
			state = "  [errored]"
		}
		iostream.Printf(ctx, "#%-4d  %-20s  %s  %s  %s%s\n",
			e.Number, e.Branch, e.Revision, e.ID, e.CreatedAt, state)
	}
}
