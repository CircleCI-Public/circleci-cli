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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/jq"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

// validResults are the outcomes accepted by --filter result=<value>.
var validResults = map[string]bool{"success": true, "failure": true, "skipped": true}

// validSortKeys are the columns accepted by --sort.
var validSortKeys = map[string]bool{"name": true, "classname": true, "result": true, "run_time": true}

func newListCmd() *cobra.Command {
	var (
		filters []string
		all     bool
		sortKey string
		limit   int
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list <job-id>",
		Aliases: []string{"ls"},
		Short:   "List test results for a job",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				job-id is the UUID of the job whose test results to list. Job
				UUIDs are shown in "circleci job get" and "circleci run get --json".
			`),
		},
		Long: heredoc.Doc(`
			Show the test results recorded for a CircleCI job.

			By default only failed tests are shown. Pass --all to show every
			result, or --filter result=value to select specific outcomes.

			--filter takes key=value and may be repeated. Repeating the same key
			matches any of its values (OR); different keys must all match (AND).
			Supported keys:
			  result      exact match: success, failure or skipped
			  name        case-insensitive substring match on the test name
			  classname   case-insensitive substring match on the suite/classname

			The result selection (failed-only default, --all, or result= filters)
			decides which outcomes are shown; name and classname filters narrow
			within that selection. --all cannot be combined with a result= filter.

			Use --sort to order results by name, classname, result or run_time
			(ascending), and --limit to cap the number of rows.

			JSON fields: classname, name, result, run_time, message

			--json emits one JSON object per line (JSONL). Combine it with --jq to
			aggregate across records: the expression runs once per record (so
			'.name' prints one name per line), and jq's inputs builtin pulls the
			rest of the stream, e.g. '[.,inputs] | length' or
			'[.,inputs] | group_by(.result) | map({(.[0].result): length})'.
		`),
		Example: heredoc.Doc(`
			# List failed tests for a job (the default)
			$ circleci testresult list 8e50c384-0083-43d0-bc8f-93f0db589d6b

			# Show every result: passing, failed and skipped
			$ circleci testresult list 8e50c384-0083-43d0-bc8f-93f0db589d6b --all

			# Show skipped tests instead
			$ circleci testresult list 8e50c384-0083-43d0-bc8f-93f0db589d6b --filter result=skipped

			# Failed tests in a specific suite, slowest last (sorted by run_time)
			$ circleci testresult list <job-id> --filter result=failure --filter classname=api --sort run_time

			# Every result for tests whose name matches "login"
			$ circleci testresult list <job-id> --all --filter name=login

			# Limit output and emit JSON for scripting
			$ circleci testresult list <job-id> --limit 20 --json

			# Count failed tests by aggregating the JSONL stream with jq
			$ circleci testresult list <job-id> --json --jq '[.,inputs] | length'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runList(ctx, client, args[0], filters, all, sortKey, limit, jsonOut)
		},
	}

	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Filter tests by key=value (result, name, classname); repeatable")
	cmd.Flags().BoolVar(&all, "all", false, "Show all results (passing, failed and skipped), not just failures")
	cmd.Flags().StringVar(&sortKey, "sort", "", "Sort by name, classname, result or run_time")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results to show (0 = no limit)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

func runList(ctx context.Context, client *apiclient.Client, idStr string, filters []string, all bool, sortKey string, limit int, jsonOut bool) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return badArg("args.invalid_job_id", "Invalid job ID", "Expected a job UUID, got: "+idStr).
			WithSuggestions("Find job UUIDs with: circleci job get")
	}

	keep, err := parseFilters(filters, all)
	if err != nil {
		return err
	}
	if err := validateSort(sortKey); err != nil {
		return err
	}
	if limit < 0 {
		return badArg("args.invalid_limit", "Invalid limit", "--limit must be zero or greater")
	}

	// --json emits JSONL (one record per line). Without a --jq filter records
	// stream straight through the decode callback, so nothing is buffered; with
	// a filter the stream is collected so jq can aggregate across records.
	// Sorting would require holding the whole response in memory regardless, and
	// jq can order the stream itself, so --sort is incompatible with --json.
	if jsonOut {
		if sortKey != "" {
			return badArg("args.conflicting_flags", "Cannot sort streamed JSON",
				"--sort cannot be combined with --json; JSON is streamed in the order the API returns it").
				WithSuggestions("Drop --sort, or sort the JSONL stream downstream (e.g. with jq)")
		}
		return streamJSON(ctx, client, id, keep, limit)
	}

	// Table output buffers the matching records so it can sort and size columns
	// before rendering. Collection still happens through the streaming callback.
	var results []apiclient.TestResult
	err = client.StreamJobTests(ctx, id, func(tr apiclient.TestResult) {
		if keep(tr) {
			results = append(results, tr)
		}
	})
	if err != nil {
		return apiErr(err, id.String())
	}

	sortTests(results, sortKey)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		iostream.Print(ctx, "No matching test results found.\n")
		return nil
	}

	table := mdtable.New("Result", "Name", "Classname", "Time (s)")
	for _, tr := range results {
		table.Row(tr.Result, tr.Name, tr.Classname, formatRunTime(tr.RunTime))
	}
	iostream.PrintMarkdown(ctx, "# Test results\n"+table.Render())
	return nil
}

// streamJSON emits each matching test result as JSONL, applying the filter and
// limit inline. Values are handed to iostream.PrintJSONStream, which streams
// them unbuffered when no --jq filter is set and otherwise collects them so the
// jq expression can aggregate across records.
func streamJSON(ctx context.Context, client *apiclient.Client, id uuid.UUID, keep func(apiclient.TestResult) bool, limit int) error {
	count := 0
	err := iostream.PrintJSONStream(ctx, func(emit func(any) error) error {
		return client.StreamJobTests(ctx, id, func(tr apiclient.TestResult) {
			if !keep(tr) {
				return
			}
			if limit > 0 && count >= limit {
				return
			}
			count++
			// A write failure (e.g. a closed pipe from a downstream "head") is
			// ignored so the pipeline stays quiet.
			_ = emit(tr)
		})
	})
	if err != nil {
		// A bad --jq expression is a user input error; let the top-level handler
		// report it as such rather than mislabeling it as an API failure.
		if errors.As(err, new(*jq.Error)) {
			return err
		}
		return apiErr(err, id.String())
	}
	return nil
}

// testFilter holds the parsed --filter predicates. Values within a slice are
// OR'd; the three slices are AND'd together.
type testFilter struct {
	results    []string // exact match on TestResult.Result
	names      []string // lower-cased substrings matched against TestResult.Name
	classnames []string // lower-cased substrings matched against TestResult.Classname
}

// parseFilters turns the repeatable --filter flags and --all into a predicate.
//
// Result selection follows a simple precedence: --all shows every outcome,
// explicit result= filters show those outcomes, and otherwise the default is
// failures only. name and classname filters always narrow within the selected
// results. --all and a result= filter are mutually exclusive.
func parseFilters(filters []string, all bool) (func(apiclient.TestResult) bool, error) {
	var f testFilter
	for _, raw := range filters {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, badArg("args.invalid_filter", "Invalid filter",
				fmt.Sprintf("Expected key=value, got: %q", raw)).
				WithSuggestions("Filter keys are result, name and classname, e.g. --filter result=failure")
		}
		switch strings.TrimSpace(key) {
		case "result":
			if !validResults[val] {
				return nil, badArg("args.invalid_filter", "Invalid filter value",
					fmt.Sprintf("result must be one of success, failure, skipped; got %q", val))
			}
			f.results = append(f.results, val)
		case "name":
			f.names = append(f.names, strings.ToLower(val))
		case "classname":
			f.classnames = append(f.classnames, strings.ToLower(val))
		default:
			return nil, badArg("args.invalid_filter", "Invalid filter key",
				fmt.Sprintf("filter key must be one of result, name, classname; got %q", key))
		}
	}

	if all && len(f.results) > 0 {
		return nil, badArg("args.conflicting_filter", "Conflicting filters",
			"--all cannot be combined with a --filter result= value").
			WithSuggestions("Use --all to show every result, or result= filters to pick specific outcomes — not both")
	}

	// Apply the failed-only default only when the caller has not otherwise
	// chosen the result set (via --all or explicit result= filters).
	if !all && len(f.results) == 0 {
		f.results = []string{"failure"}
	}

	return f.matches, nil
}

func (f testFilter) matches(tr apiclient.TestResult) bool {
	if len(f.results) > 0 && !containsExact(f.results, tr.Result) {
		return false
	}
	if len(f.names) > 0 && !containsSubstr(f.names, strings.ToLower(tr.Name)) {
		return false
	}
	if len(f.classnames) > 0 && !containsSubstr(f.classnames, strings.ToLower(tr.Classname)) {
		return false
	}
	return true
}

func containsExact(candidates []string, v string) bool {
	for _, c := range candidates {
		if c == v {
			return true
		}
	}
	return false
}

func containsSubstr(needles []string, haystack string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func validateSort(key string) error {
	if key == "" || validSortKeys[key] {
		return nil
	}
	return badArg("args.invalid_sort", "Invalid sort key",
		fmt.Sprintf("sort must be one of name, classname, result, run_time; got %q", key))
}

// sortTests orders results in place, ascending, by the given key. An empty key
// preserves the order returned by the API.
func sortTests(results []apiclient.TestResult, key string) {
	switch key {
	case "name":
		sort.SliceStable(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	case "classname":
		sort.SliceStable(results, func(i, j int) bool { return results[i].Classname < results[j].Classname })
	case "result":
		sort.SliceStable(results, func(i, j int) bool { return results[i].Result < results[j].Result })
	case "run_time":
		sort.SliceStable(results, func(i, j int) bool { return results[i].RunTime < results[j].RunTime })
	}
}

func formatRunTime(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', 2, 64)
}

func badArg(code, title, msg string) *clierrors.CLIError {
	return clierrors.New(code, title, msg).WithExitCode(clierrors.ExitBadArguments)
}
