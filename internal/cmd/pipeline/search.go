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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/mdtable"
)

func newSearchCmd() *cobra.Command {
	var (
		projectSlug string
		filter      string
		after       string
		before      string
		limit       int
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search pipelines using filter expressions",
		Long: heredoc.Doc(`
			Search pipelines using the CircleCI pipeline search API.

			The project is inferred from the current git repository's remote unless
			overridden with --project. For simple branch filtering use "circleci
			pipeline list"; this command is for the filter-expression API.

			By default, results are limited to pipelines created in the last
			two weeks. Use --after and --before to broaden or narrow the window.

			Filter expressions use field names like pipeline.git.branch,
			pipeline.git.tag, pipeline.git.revision, pipeline.number, and
			actor.id. See the CircleCI API docs for the full list of fields.

			Operators: ==, != (equality); starts-with (string prefix);
			           <, <=, >, >= (numeric); and, or, not (logical)

			actor.id takes a user UUID, not a login. Find a user's UUID via
			"circleci api /me".

			JSON fields: id, number, state, status, branch, revision,
			             workflows_summary (map of status → count),
			             created_at (RFC3339)
		`),
		Example: heredoc.Doc(`
			# Search pipelines on the main branch
			$ circleci pipeline search --filter 'pipeline.git.branch == "main"'

			# Search pipelines on branches starting with "feature/"
			$ circleci pipeline search --filter 'pipeline.git.branch starts-with "feature/"'

			# Combine fields with "and"
			$ circleci pipeline search --filter 'pipeline.git.branch == "main" and actor.id == "104c584e-50cb-4f72-a43a-a38a7b0b6a7b"'

			# Search within a date range and output as JSON
			$ circleci pipeline search --after 2024-01-01T00:00:00Z --before 2024-02-01T00:00:00Z --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return clierrors.New("search.invalid_limit", "Invalid limit",
					fmt.Sprintf("--limit must be a positive integer, got %d", limit),
				).WithExitCode(clierrors.ExitBadArguments)
			}

			var afterTime, beforeTime *time.Time
			if after != "" {
				t, err := time.Parse(time.RFC3339, after)
				if err != nil {
					return clierrors.New("search.invalid_date", "Invalid date",
						fmt.Sprintf("--after %q is not a valid RFC3339 timestamp", after),
					).WithExitCode(clierrors.ExitBadArguments)
				}
				afterTime = &t
			}
			if before != "" {
				t, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return clierrors.New("search.invalid_date", "Invalid date",
						fmt.Sprintf("--before %q is not a valid RFC3339 timestamp", before),
					).WithExitCode(clierrors.ExitBadArguments)
				}
				beforeTime = &t
			}

			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return runSearch(ctx, client, searchParams{
				projectSlug: projectSlug,
				filter:      filter,
				afterTime:   afterTime,
				beforeTime:  beforeTime,
				limit:       limit,
				jsonOut:     jsonOut,
			})
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&filter, "filter", "", `Filter expression (e.g. 'pipeline.git.branch == "main"')`)
	cmd.Flags().StringVar(&after, "after", "", "Only pipelines created after this time (RFC3339)")
	cmd.Flags().StringVar(&before, "before", "", "Only pipelines created before this time (RFC3339)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type searchParams struct {
	projectSlug string
	filter      string
	afterTime   *time.Time
	beforeTime  *time.Time
	limit       int
	jsonOut     bool
}

func runSearch(ctx context.Context, client *apiclient.Client, p searchParams) error {
	slug := p.projectSlug
	if slug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci pipeline search --project gh/org/repo")
		}
		slug = info.Slug
	}

	sp := iostream.Spinner(ctx, !p.jsonOut, "Searching pipelines...")

	projectInfo, err := client.GetProjectInfo(ctx, slug)
	if err != nil {
		sp.Stop()
		return cmdutil.APIErr(err, slug, "project.not_found", "No project found for %q.",
			"Check the project slug and try again")
	}

	scope := &apiclient.SearchScope{
		ProjectIDs:    []string{projectInfo.ID},
		CreatedAfter:  p.afterTime,
		CreatedBefore: p.beforeTime,
	}

	req := apiclient.SearchPipelinesRequest{
		Scope:  scope,
		Filter: p.filter,
	}

	pipelines, err := client.SearchPipelines(ctx, req, p.limit)
	sp.Stop()
	if err != nil {
		return apiErr(err, slug)
	}

	entries := make([]searchResultEntry, len(pipelines))
	for i, pip := range pipelines {
		entries[i] = toSearchResultEntry(&pip)
	}

	if p.jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No pipelines found.")
		return nil
	}

	printSearchResults(ctx, entries)
	return nil
}

type searchResultEntry struct {
	ID               string         `json:"id"`
	Number           int64          `json:"number"`
	State            string         `json:"state,omitempty"`
	Status           string         `json:"status,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Revision         string         `json:"revision,omitempty"`
	WorkflowsSummary map[string]int `json:"workflows_summary,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

func toSearchResultEntry(p *apiclient.SearchPipeline) searchResultEntry {
	e := searchResultEntry{
		ID:        p.ID,
		Number:    p.Number,
		State:     p.State,
		Status:    p.Status,
		CreatedAt: p.CreatedAt.UTC(),
	}
	if p.VCS != nil {
		e.Branch = p.VCS.Branch
		e.Revision = p.VCS.Revision
	}
	if p.WorkflowsSummary != nil {
		e.WorkflowsSummary = p.WorkflowsSummary.CountByStatus
	}
	return e
}

func formatWorkflowCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k, n := range counts {
		if n > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%d %s", counts[k], k)
	}
	return strings.Join(parts, ", ")
}

func printSearchResults(ctx context.Context, entries []searchResultEntry) {
	table := mdtable.New("#", "Branch", "Revision", "Status", "Workflows", "Created", "Pipeline")
	for _, e := range entries {
		rev := e.Revision
		if len(rev) > 7 {
			rev = rev[:7]
		}
		table.Row(
			strconv.Itoa(int(e.Number)),
			e.Branch,
			rev,
			e.Status,
			formatWorkflowCounts(e.WorkflowsSummary),
			e.CreatedAt.Format("2006-01-02 15:04 UTC"),
			"`"+e.ID+"`",
		)
	}
	iostream.PrintMarkdown(ctx, "# Pipeline Search Results\n"+table.Render())
}
