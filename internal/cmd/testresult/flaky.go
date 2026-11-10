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

package testresult

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// maxFlakePages bounds how many pages one call fetches, so a project with a
// long flake history cannot make the command hang on serial requests.
const maxFlakePages = 50

// validFlakySortKeys are the keys accepted by "testresult flaky --sort".
var validFlakySortKeys = map[string]bool{"times_flaked": true, "last_flaked": true, "name": true}

// flakyTest is one flaky test: the API's per-occurrence records grouped by
// test, counted, and described by the most recent occurrence.
type flakyTest struct {
	TestName     string    `json:"test_name"`
	Classname    string    `json:"classname"`
	File         string    `json:"file"`
	TimesFlaked  int       `json:"times_flaked"`
	LastFlakedAt time.Time `json:"last_flaked_at"`
	JobName      string    `json:"job_name"`
	WorkflowName string    `json:"workflow_name"`
	JobID        uuid.UUID `json:"job_id"`
	WorkflowID   uuid.UUID `json:"workflow_id"`
	RunID        uuid.UUID `json:"run_id"`
}

func newFlakyCmd() *cobra.Command {
	var (
		projectSlug string
		sortKey     string
		limit       int
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "flaky",
		Short: "List flaky tests detected across a project's runs",
		Long: heredoc.Doc(`
			List the tests that failed and then passed within the same pipeline,
			one row per test, most-flaked first. One call answers "which tests are
			flaky?" — no need to walk runs and jobs. Job and workflow details are
			those of each test's most recent flake.

			JSON fields: test_name, classname, file, times_flaked, last_flaked_at,
			job_name, workflow_name, job_id, workflow_id, run_id — one object per
			line (JSONL).
		`),
		Example: heredoc.Doc(`
			# Flaky tests for the project in the current directory
			$ circleci testresult flaky

			# The five worst offenders in another project
			$ circleci testresult flaky --project gh/myorg/myrepo --limit 5

			# Names of tests that flaked more than five times
			$ circleci testresult flaky --json --jq 'select(.times_flaked > 5) | .test_name'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := validateFlakyFlags(sortKey, limit); err != nil {
				return err
			}
			slug, err := cmdutil.ResolveProjectSlug(projectSlug)
			if err != nil {
				return err
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runFlaky(ctx, client, slug, sortKey, limit, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&sortKey, "sort", "times_flaked", "Sort by times_flaked or last_flaked (descending), or name")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of tests to show (0 = no limit)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

func validateFlakyFlags(sortKey string, limit int) error {
	if !validFlakySortKeys[sortKey] {
		return badArg("args.invalid_sort", "Invalid sort key",
			fmt.Sprintf("sort must be one of times_flaked, last_flaked, name; got %q", sortKey))
	}
	if limit < 0 {
		return badArg("args.invalid_limit", "Invalid limit", "--limit must be zero or greater")
	}
	return nil
}

func runFlaky(ctx context.Context, client *apiclient.Client, slug, sortKey string, limit int, jsonOut bool) error {
	notFound := func(err error) error {
		return cmdutil.APIErr(err, slug,
			"project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Or specify the project with --project gh/org/repo")
	}
	sp := iostream.Spinner(ctx, !jsonOut, "Fetching flaky tests for "+slug)
	proj, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		sp.Stop()
		return notFound(err)
	}
	search, err := client.SearchTestFlakes(ctx, proj.ID, maxFlakePages)
	sp.Stop()
	if err != nil {
		// The project exists, so a 404 here means the host has no test-flake
		// data to serve, not that the project is missing.
		return cmdutil.APIErr(err, slug,
			"testresult.flakes_unavailable", "Flaky-test data is not available for %q.",
			"The test-flakes API is experimental and may not be enabled on this CircleCI host",
			"Check CIRCLE_HOST if you are not using circleci.com")
	}

	all := groupFlakes(search.Flakes)
	sortFlakyTests(all, sortKey)
	tests := all
	if limit > 0 && len(tests) > limit {
		tests = tests[:limit]
	}

	if jsonOut {
		// The records are already in hand, so the only errors here are a bad
		// --jq expression, which the top-level handler reports as user input.
		err := iostream.PrintJSONStream(ctx, func(emit func(any) error) error {
			for _, ft := range tests {
				// A write failure (e.g. a closed pipe from "head") ends the
				// stream quietly.
				if emit(ft) != nil {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		printIncompleteNote(ctx, search)
		return nil
	}

	if len(tests) == 0 {
		iostream.Printf(ctx, "No flaky tests detected for %s.\n", slug)
		printIncompleteNote(ctx, search)
		return nil
	}

	table := mdtable.New("Test", "Classname", "Times flaked", "Job", "Last flaked")
	for _, ft := range tests {
		table.Row(ft.TestName, ft.Classname, strconv.Itoa(ft.TimesFlaked), ft.JobName,
			cmdutil.DisplayTime(ft.LastFlakedAt))
	}
	iostream.PrintMarkdown(ctx, "# Flaky tests\n"+table.Render())
	printIncompleteNote(ctx, search)

	if len(all) > len(tests) {
		iostream.ErrPrintf(ctx, "Showing %d of %d flaky tests.\n", len(tests), len(all))
	}
	return nil
}

// printIncompleteNote tells the user, on stderr, when the search stopped before
// the last page, since the counts shown are then a lower bound.
func printIncompleteNote(ctx context.Context, search apiclient.TestFlakeSearch) {
	switch search.Stopped {
	case apiclient.TestFlakesPageCap:
		iostream.ErrPrintf(ctx, "Results are incomplete: stopped after %d pages (%d flake occurrences). "+
			"The API orders occurrences by file, so later tests may be missing and counts may be low.\n",
			maxFlakePages, len(search.Flakes))
	case apiclient.TestFlakesRepeatedCursor:
		iostream.ErrPrintf(ctx, "Results are incomplete: the API returned the same page cursor twice, "+
			"so paging stopped after %d flake occurrences.\n", len(search.Flakes))
	case apiclient.TestFlakesComplete:
	}
}

// groupFlakes folds flake occurrences into one flakyTest per test, keyed by
// file, classname and name: the count is the number of occurrences, and the
// job and workflow come from the most recent one.
func groupFlakes(flakes []apiclient.TestFlake) []flakyTest {
	type key struct{ file, classname, name string }
	byTest := map[key]*flakyTest{}
	var order []key
	for _, f := range flakes {
		k := key{f.File, f.Classname, f.Name}
		ft, ok := byTest[k]
		if !ok {
			ft = &flakyTest{TestName: f.Name, Classname: f.Classname, File: f.File}
			byTest[k] = ft
			order = append(order, k)
		}
		ft.TimesFlaked++
		if ft.TimesFlaked == 1 || f.JobStartedAt.After(ft.LastFlakedAt) {
			ft.LastFlakedAt = f.JobStartedAt
			ft.JobName = f.JobName
			ft.WorkflowName = f.WorkflowName
			ft.JobID = f.JobID
			ft.WorkflowID = f.WorkflowID
			ft.RunID = f.RunID
		}
	}
	tests := make([]flakyTest, 0, len(order))
	for _, k := range order {
		tests = append(tests, *byTest[k])
	}
	return tests
}

// sortFlakyTests orders tests in place. times_flaked and last_flaked sort
// descending (worst or newest first); name sorts ascending. Ties fall back to
// test name so the output is stable regardless of API order.
func sortFlakyTests(tests []flakyTest, key string) {
	byName := func(a, b flakyTest) bool {
		if a.TestName != b.TestName {
			return a.TestName < b.TestName
		}
		return a.Classname < b.Classname
	}
	sort.SliceStable(tests, func(i, j int) bool {
		a, b := tests[i], tests[j]
		switch key {
		case "last_flaked":
			if !a.LastFlakedAt.Equal(b.LastFlakedAt) {
				return a.LastFlakedAt.After(b.LastFlakedAt)
			}
		case "times_flaked":
			if a.TimesFlaked != b.TimesFlaked {
				return a.TimesFlaked > b.TimesFlaked
			}
		}
		return byName(a, b)
	})
}
