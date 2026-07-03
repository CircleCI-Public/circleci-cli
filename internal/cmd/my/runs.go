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

package my

import (
	"context"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newRunsCmd() *cobra.Command {
	var (
		limit   int
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "runs",
		Aliases: []string{"run"},
		Short:   "List your recent runs grouped by project",
		Long: heredoc.Doc(`
			List recent runs you triggered, across every project you have access to.

			This is the personal counterpart to "circleci run list": rather than a
			single project, it shows your runs everywhere, in the order the API
			returns them, with the project of each run in its own column.

			JSON: an array of runs, each { project, project_id, id, phase, outcome,
			current_outcome, branch, tag, revision, created_at }, where project is
			the run's "org/repo" repository (when known) and project_id its UUID.
		`),
		Example: heredoc.Doc(`
			# List your recent runs across all projects
			$ circleci my runs

			# Show more results
			$ circleci my runs --limit 50

			# Output as JSON for scripting
			$ circleci my runs --json

			# Pull out just the run IDs with jq
			$ circleci my runs --json --jq '.[].id'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runMyRuns(ctx, client, limit, jsonOut)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of runs to fetch [default: 20]")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runEntry struct {
	Project        string    `json:"project,omitempty"`
	ProjectID      string    `json:"project_id,omitempty"`
	ID             uuid.UUID `json:"id"`
	Phase          string    `json:"phase"`
	Outcome        string    `json:"outcome,omitempty"`
	CurrentOutcome string    `json:"current_outcome,omitempty"`
	Branch         string    `json:"branch,omitempty"`
	Tag            string    `json:"tag,omitempty"`
	Revision       string    `json:"revision,omitempty"`
	CreatedAt      string    `json:"created_at"`
}

func runMyRuns(ctx context.Context, client *apiclient.Client, limit int, jsonOut bool) error {
	runs, err := client.ListMyRunsV3(ctx, limit, "")
	if err != nil {
		return cmdutil.APIErr(err, "your runs",
			"my.runs_failed", "Could not list your runs.")
	}

	// Each entry carries the run's "org/repo" repository slug and its project
	// UUID. JSON exposes both (project + project_id); the table shows the slug.
	entries := make([]runEntry, len(runs))
	for i := range runs {
		entries[i] = toEntry(&runs[i],
			cmdutil.RepoSlug(runs[i].RepositoryURL),
			projectIDString(runs[i].ProjectID))
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No runs found.")
		return nil
	}

	printRuns(ctx, entries)
	return nil
}

// projectIDString renders a run's project UUID for JSON, or "" for the nil UUID
// (so the omitempty project_id field is dropped rather than showing all-zeroes).
func projectIDString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func toEntry(r *apiclient.RunV3, project, projectID string) runEntry {
	rev := r.Revision
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return runEntry{
		Project:        project,
		ProjectID:      projectID,
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Tag:            r.Tag,
		Revision:       rev,
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
}

// printRuns renders every run in API order as one table with a Project column;
// runs are not grouped or reordered.
func printRuns(ctx context.Context, entries []runEntry) {
	var b strings.Builder
	b.WriteString("# My runs\n\n")
	table := mdtable.New("Project", "Ref", "Revision", "ID", "Created", "Status")
	for _, e := range entries {
		project := e.Project
		if project == "" {
			project = "(unknown)"
		}
		table.Row(
			project,
			refDisplay(e.Branch, e.Tag),
			e.Revision,
			"`"+e.ID.String()+"`",
			e.CreatedAt,
			apiclient.PhaseOutcomeStatus(e.Phase, e.Outcome, e.CurrentOutcome),
		)
	}
	b.WriteString(table.Render())
	iostream.PrintMarkdown(ctx, b.String())
}

// refDisplay renders the git ref for a run: the branch, or the tag (marked
// with 🏷) for runs triggered by a tag rather than a branch.
func refDisplay(branch, tag string) string {
	if branch == "" && tag != "" {
		return "🏷 " + tag
	}
	return branch
}
