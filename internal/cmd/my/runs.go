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
	"net/url"
	"sort"
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
			List recent runs you triggered, grouped by project.

			This is the personal counterpart to "circleci run list": rather than
			a single project, it shows your runs across everywhere you have access,
			grouped by project with the most recent run first.

			JSON: an array of projects, each { repository, runs: [...] }.
			Run fields: id, phase, outcome, current_outcome, branch, tag, revision, created_at
		`),
		Example: heredoc.Doc(`
			# List your recent runs grouped by project
			$ circleci my runs

			# Show more results
			$ circleci my runs --limit 50

			# Output as JSON for scripting
			$ circleci my runs --json

			# Pull out just the run IDs with jq
			$ circleci my runs --json --jq '.[].runs[].id'
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

type projectRuns struct {
	Repository string     `json:"repository"`
	Runs       []runEntry `json:"runs"`
}

type runEntry struct {
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
	runs, err := client.ListMyRunsV3(ctx, limit)
	if err != nil {
		return cmdutil.APIErr(err, "your runs",
			"my.runs_failed", "Could not list your runs.")
	}

	groups := groupByProject(runs)

	if jsonOut {
		return iostream.PrintJSON(ctx, groups)
	}

	if len(groups) == 0 {
		iostream.ErrPrintln(ctx, "No runs found.")
		return nil
	}

	printRuns(ctx, groups)
	return nil
}

// groupByProject sorts runs newest-first, then groups them by repository.
// Group order follows each project's most recent run, and runs within a group
// stay newest-first.
func groupByProject(runs []apiclient.RunV3) []projectRuns {
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	index := map[string]int{}
	var groups []projectRuns
	for i := range runs {
		repo := repoName(runs[i].RepositoryURL)
		gi, ok := index[repo]
		if !ok {
			gi = len(groups)
			index[repo] = gi
			groups = append(groups, projectRuns{Repository: repo})
		}
		groups[gi].Runs = append(groups[gi].Runs, toEntry(&runs[i]))
	}
	return groups
}

func toEntry(r *apiclient.RunV3) runEntry {
	rev := r.Revision
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return runEntry{
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

func printRuns(ctx context.Context, groups []projectRuns) {
	var b strings.Builder
	b.WriteString("# My runs\n")
	for _, g := range groups {
		repo := g.Repository
		if repo == "" {
			repo = "(unknown repository)"
		}
		b.WriteString("\n## " + repo + "\n")
		table := mdtable.New("Ref", "Revision", "ID", "Created", "Status")
		for _, e := range g.Runs {
			table.Row(
				refDisplay(e.Branch, e.Tag),
				e.Revision,
				"`"+e.ID.String()+"`",
				e.CreatedAt,
				apiclient.PhaseOutcomeStatus(e.Phase, e.Outcome, e.CurrentOutcome),
			)
		}
		b.WriteString(table.Render())
	}
	iostream.PrintMarkdown(ctx, b.String())
}

// repoName reduces a repository URL to its "org/repo" form for display,
// e.g. "https://github.com/circleci/foo" -> "circleci/foo". It returns the
// input unchanged if it cannot be parsed.
func repoName(repoURL string) string {
	if repoURL == "" {
		return ""
	}
	u, err := url.Parse(repoURL)
	if err != nil || u.Path == "" {
		return repoURL
	}
	return strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
}

// refDisplay renders the git ref for a run: the branch, or the tag (marked
// with 🏷) for runs triggered by a tag rather than a branch.
func refDisplay(branch, tag string) string {
	if branch == "" && tag != "" {
		return "🏷 " + tag
	}
	return branch
}
