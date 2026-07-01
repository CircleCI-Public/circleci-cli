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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/termrender"
)

func newGetCmd() *cobra.Command {
	var (
		filters []string
		jsonOut bool
		plain   bool
	)

	cmd := &cobra.Command{
		Use:   "get <job-id> <name>",
		Short: "Get a single test result by name",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<job-id> is the UUID of the job whose test results to search. Job
				UUIDs are shown in "circleci job get" and "circleci run get --json".

				<name> is the exact test name to look up. If more than one test
				shares that name, use --filter classname=<value> to disambiguate.
			`),
		},
		Long: heredoc.Doc(`
			Get a single test result from a job by its exact name.

			The name must match exactly. When several tests share a name — for
			example the same test running under different suites — the lookup is
			ambiguous and fails; pass --filter classname=<value> to narrow to one.
			classname is matched as a case-insensitive substring and may be
			repeated to accept any of several suites.

			To browse or filter across all of a job's tests, use
			'circleci testresult list'.

			By default the message is replayed through a virtual terminal and
			embedded in the output. Pass --plain to print only the message exactly
			as the test runner recorded it (ANSI and all), which is handy for
			piping a failure's output to another tool.

			JSON fields: classname, name, result, run_time, message
		`),
		Example: heredoc.Doc(`
			# Get a test by name
			$ circleci testresult get 8e50c384-0083-43d0-bc8f-93f0db589d6b TestLogin

			# Disambiguate when the name appears in multiple suites
			$ circleci testresult get <job-id> TestLogin --filter classname=api

			# Print only the raw test message
			$ circleci testresult get <job-id> TestLogin --plain

			# Output as JSON
			$ circleci testresult get <job-id> TestLogin --json
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id", "name"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args[0], args[1], filters, jsonOut, plain)
		},
	}

	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Disambiguate by classname=<value> when a name is shared; repeatable")
	cmd.Flags().BoolVar(&plain, "plain", false, "Print only the raw test message, verbatim and unformatted")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

func runGet(ctx context.Context, client *apiclient.Client, idStr, name string, filters []string, jsonOut, plain bool) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return badArg("args.invalid_job_id", "Invalid job ID", "Expected a job UUID, got: "+idStr).
			WithSuggestions("Find job UUIDs with: circleci job get")
	}

	if plain && jsonOut {
		return badArg("args.conflicting_flags", "Conflicting output flags",
			"--plain and --json cannot be combined; choose one output format")
	}

	classnames, err := parseClassnameFilter(filters)
	if err != nil {
		return err
	}

	// Collect every test with the exact name, narrowed by the classname
	// disambiguator, so we can tell "not found" from "ambiguous".
	var matches []apiclient.TestResult
	err = client.StreamJobTests(ctx, id, func(tr apiclient.TestResult) {
		if tr.Name != name {
			return
		}
		if len(classnames) > 0 && !containsSubstr(classnames, strings.ToLower(tr.Classname)) {
			return
		}
		matches = append(matches, tr)
	})
	if err != nil {
		return apiErr(err, id.String())
	}

	switch len(matches) {
	case 0:
		return testNotFound(idStr, name, len(classnames) > 0)
	case 1:
		// Exactly one — fall through.
	default:
		return ambiguousTest(idStr, name, matches)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, matches[0])
	}
	if plain {
		// Only the message, exactly as recorded. A trailing newline is ensured
		// so the shell prompt starts on its own line, but nothing else is added.
		msg := matches[0].Message
		iostream.Print(ctx, msg)
		if !strings.HasSuffix(msg, "\n") {
			iostream.Print(ctx, "\n")
		}
		return nil
	}
	printTest(ctx, matches[0])
	return nil
}

// parseClassnameFilter reads the --filter flags, which for "test get" may only
// carry the classname disambiguator. Values are lower-cased for a
// case-insensitive substring match.
func parseClassnameFilter(filters []string) ([]string, error) {
	var classnames []string
	for _, raw := range filters {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, badArg("args.invalid_filter", "Invalid filter",
				fmt.Sprintf("Expected key=value, got: %q", raw)).
				WithSuggestions("The only filter accepted here is classname, e.g. --filter classname=api")
		}
		if strings.TrimSpace(key) != "classname" {
			return nil, badArg("args.invalid_filter", "Invalid filter key",
				fmt.Sprintf("test get only supports the classname filter; got %q", key)).
				WithSuggestions(`Use "circleci testresult list" to filter by result or name`)
		}
		classnames = append(classnames, strings.ToLower(val))
	}
	return classnames, nil
}

func printTest(ctx context.Context, tr apiclient.TestResult) {
	var md strings.Builder
	md.WriteString("# Test\n")
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", tr.Name)
	_, _ = fmt.Fprintf(&md, "- Classname: %s\n", tr.Classname)
	_, _ = fmt.Fprintf(&md, "- Result: %s\n", tr.Result)
	_, _ = fmt.Fprintf(&md, "- Time: %ss\n", formatRunTime(tr.RunTime))
	if msg := renderMessage(tr.Message); msg != "" {
		// A test message is captured terminal output and can contain backticks
		// (e.g. a printed code block), so size the fence to outlast any run. The
		// "text" info string matches "job output list" and stops renderers from
		// guessing a language for the log.
		fence := cmdutil.CodeFence(msg)
		md.WriteString("\n## Message\n\n")
		md.WriteString(fence + "text\n")
		md.WriteString(msg + "\n")
		md.WriteString(fence + "\n")
	}
	iostream.PrintMarkdown(ctx, md.String())
}

// renderMessage replays the captured terminal output stored in a test's message
// through termrender, collapsing in-place redraws (progress bars, TUI frames)
// and discarding ANSI styling, so the result is plain text safe to embed in a
// markdown code block. Only the human view renders the message; JSON output
// keeps it raw. On a render error the raw message is used as a fallback.
func renderMessage(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	var buf strings.Builder
	if err := termrender.Render(&buf, strings.NewReader(message)); err != nil {
		return strings.TrimRight(message, "\n")
	}
	return strings.TrimRight(buf.String(), "\n")
}

func testNotFound(jobID, name string, filtered bool) error {
	msg := fmt.Sprintf("No test named %q was found in job %s.", name, jobID)
	if filtered {
		msg = fmt.Sprintf("No test named %q with a matching classname was found in job %s.", name, jobID)
	}
	return clierrors.New("test.not_found", "Test not found", msg).
		WithSuggestions(fmt.Sprintf("List the job's tests with: circleci testresult list %s --all", jobID)).
		WithExitCode(clierrors.ExitNotFound)
}

func ambiguousTest(jobID, name string, matches []apiclient.TestResult) error {
	// One suggestion per distinct classname, in the order first seen, so the
	// user can copy a fully-formed disambiguating command.
	seen := map[string]bool{}
	var suggestions []string
	for _, m := range matches {
		if seen[m.Classname] {
			continue
		}
		seen[m.Classname] = true
		suggestions = append(suggestions,
			fmt.Sprintf("circleci testresult get %s %q --filter classname=%s", jobID, name, m.Classname))
	}
	return badArg("test.ambiguous", "Ambiguous test name",
		fmt.Sprintf("%d tests are named %q; add --filter classname=<value> to select one.", len(matches), name)).
		WithSuggestions(suggestions...)
}
