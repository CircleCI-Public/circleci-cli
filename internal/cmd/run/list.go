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
	"github.com/google/uuid"
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
		Short:   "List recent runs for a project",
		Long: heredoc.Doc(`
			List recent runs for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Use --branch to filter results
			to a single branch, or --current-branch (-B) to automatically use
			the branch you have checked out.

			The markdown table includes the commit subject; the JSON adds the full
			commit and repository detail.

			JSON fields: id, phase, outcome, current_outcome, branch, tag, revision,
			             repository_url, commit.subject/url/author_name/author_login,
			             created_at
		`),
		Example: heredoc.Doc(`
			# List recent runs for the current project
			$ circleci run list

			# Filter to the branch you have checked out
			$ circleci run list --current-branch

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
			if currentBranch && branch == "" {
				info, err := gitremote.Detect()
				if err != nil {
					return cmdutil.GitDetectErr(err, "Or specify the branch explicitly: circleci run list --branch <name>")
				}
				branch = info.Branch
			}
			return runList(ctx, client, projectSlug, branch, limit, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch")
	cmd.Flags().BoolVarP(&currentBranch, "current-branch", "B", false, "Filter by the currently checked-out branch")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of runs to show [default: 10]")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runListEntry struct {
	ID             uuid.UUID     `json:"id"`
	Phase          string        `json:"phase"`
	Outcome        string        `json:"outcome,omitempty"`
	CurrentOutcome string        `json:"current_outcome,omitempty"`
	Branch         string        `json:"branch,omitempty"`
	Tag            string        `json:"tag,omitempty"`
	Revision       string        `json:"revision,omitempty"`
	RepositoryURL  string        `json:"repository_url,omitempty"`
	Commit         *commitOutput `json:"commit,omitempty"`
	CreatedAt      string        `json:"created_at"`
}

func runList(ctx context.Context, client *apiclient.Client, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci run list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	now := time.Now().UTC()
	runs, err := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
		ProjectIDs: []string{proj.ID.String()},
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
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Tag:            r.Tag,
		Revision:       rev,
		RepositoryURL:  r.RepositoryURL,
		Commit:         commitOutputFrom(r.Commit),
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
}

func printList(ctx context.Context, entries []runListEntry) {
	table := mdtable.New("Ref", "Revision", "Subject", "ID", "Created", "Status")
	for _, e := range entries {
		table.Row(refDisplay(e.Branch, e.Tag), orDash(e.Revision), orDash(entrySubject(e)), "`"+e.ID.String()+"`", e.CreatedAt, apiclient.PhaseOutcomeStatus(e.Phase, e.Outcome, e.CurrentOutcome))
	}
	iostream.PrintMarkdown(ctx, "# Runs\n"+table.Render())
}

// entrySubject is the commit subject shown in the Subject column, capped tight
// (runListSubjectMax) so the row never wraps, or "" when the run resolved no
// commit.
func entrySubject(e runListEntry) string {
	if e.Commit == nil {
		return ""
	}
	return subjectDisplay(e.Commit.Subject, runListSubjectMax)
}

// refDisplay renders the git ref for a run: the branch, or the tag (marked
// with 🏷) for runs triggered by a tag rather than a branch. A run that
// resolved neither — e.g. one that never ran because its config could not be
// fetched — shows "-" so the column is not blank.
func refDisplay(branch, tag string) string {
	if branch == "" && tag != "" {
		return "🏷 " + tag
	}
	return orDash(branch)
}

// orDash returns s, or "-" when s is empty, so table cells never render blank.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
