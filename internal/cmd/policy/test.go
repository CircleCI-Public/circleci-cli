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

package policy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
	"github.com/CircleCI-Public/circle-policy-agent/cpa/tester"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newTestCmd() *cobra.Command {
	var (
		org     string
		run     string
		all     bool
		explain bool
		jsonOut bool
		junit   bool
	)

	cmd := &cobra.Command{
		Use:   "test <path>",
		Short: "Run policy tests",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is a directory of policies and tests. Append %[1]s/...%[1]s
				to discover tests recursively in every subdirectory.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Discover and run policy tests locally.

			Tests live in *_test.yaml / *_test.yml files alongside your .rego
			policies; each test key must start with "test_". Every test compares its
			expected "decision" against the decision the policy engine produces for
			the given "input", and native OPA unit tests are run too.

			A test's input is compiled first only when it sets "compile: true" or
			provides "pipeline_parameters"; otherwise the raw input is evaluated.
			Compilation uses the CircleCI compile endpoint, so --org (or a git remote)
			and a token are required only when a test compiles.

			The command exits non-zero if any test fails.

			By default results print as a human-readable table showing only
			failures; pass --all to include passing tests. Use --json for a JSON
			array of results (scriptable, --jq-aware) or --junit for JUnit XML.

			Use --explain to print each test's full evaluation context (input,
			decision, and evaluation), which implies --all.

			JSON fields: Passed, Group, Name, Elapsed, ElapsedMS, Err, Ctx
		`),
		Example: heredoc.Doc(`
			# Run every test under ./policies and its subdirectories
			$ circleci policy test ./policies/...

			# Run tests in a single directory, showing passing tests too
			$ circleci policy test ./policies --all

			# Run only tests whose name matches a regexp
			$ circleci policy test ./policies/... --run 'test_enforce_.*'

			# Emit JUnit XML for CI
			$ circleci policy test ./policies/... --junit
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var include *regexp.Regexp
			if run != "" {
				var rerr error
				include, rerr = regexp.Compile(run)
				if rerr != nil {
					return clierrors.New("policy.invalid_run", "Invalid --run regexp", rerr.Error()).
						WithSuggestions("Pass a valid regular expression, e.g. --run 'test_.*'").
						WithExitCode(clierrors.ExitBadArguments)
				}
			}

			// The API client is only needed when a test compiles its input, so it is
			// created lazily on first use: `policy test` runs offline for tests that
			// supply a raw input.
			var (
				client    *apiclient.Client
				ownerID   string
				setupErr  error
				setupOnce sync.Once
			)
			compile := func(data []byte, pipelineValues map[string]any) ([]byte, error) {
				setupOnce.Do(func() {
					client, setupErr = cmdutil.LoadClient(ctx)
					if setupErr != nil {
						return
					}
					ownerID, setupErr = resolveOwnerID(ctx, client, org, "circleci policy test")
				})
				if setupErr != nil {
					return nil, setupErr
				}

				parameters, _ := pipelineValues["parameters"].(map[string]any)
				resp, err := client.CompileConfig(ctx, string(data), ownerID, false, configcmd.LocalPipelineValues(parameters), parameters)
				if err != nil {
					return nil, err
				}
				if len(resp.Errors) > 0 {
					msgs := make([]error, len(resp.Errors))
					for i, e := range resp.Errors {
						msgs[i] = errors.New(e.Message)
					}
					return nil, errors.Join(msgs...)
				}
				if !resp.Valid {
					return nil, errors.New("config compilation failed")
				}
				return []byte(resp.OutputYAML), nil
			}

			runner, err := tester.NewRunner(tester.RunnerOptions{
				Path:    args[0],
				Include: include,
				Compile: compile,
			})
			if err != nil {
				return clierrors.New("policy.test_setup_failed", "Could not set up policy test runner", err.Error()).
					WithSuggestions("Check that the path exists and contains policies and tests").
					WithExitCode(clierrors.ExitBadArguments)
			}

			testFailed := clierrors.New("policy.test_failed", "Policy tests failed",
				"One or more policy tests did not pass.").
				WithExitCode(clierrors.ExitValidationFail)

			// --junit takes precedence and uses the tester's XML handler (JUnit is
			// not a human/JSON format). Otherwise JSON goes through the shared
			// output helpers (color- and --jq-aware) and the default human output
			// is a Markdown table rendered like every other command.
			switch {
			case junit:
				handlerOpts := tester.ResultHandlerOptions{Verbose: all, Debug: explain, Dst: iostream.Out(ctx)}
				if !runner.RunAndHandleResults(tester.MakeJUnitResultHandler(handlerOpts)) {
					return testFailed
				}
			case jsonOut:
				results, ok := collectResults(runner, explain)
				if err := iostream.PrintJSON(ctx, results); err != nil {
					return err
				}
				if !ok {
					return testFailed
				}
			default:
				if !renderResultsMarkdown(ctx, runner, all, explain) {
					return testFailed
				}
			}
			return nil
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "for private orb resolution when a test compiles", DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&run, "run", "", "Only run tests whose name matches this regexp")
	cmd.Flags().BoolVar(&all, "all", false, "Show all tests, not just failures")
	cmd.Flags().BoolVar(&explain, "explain", false, "Print each test's full evaluation context (implies --all)")
	cmd.Flags().BoolVar(&junit, "junit", false, "Output results as JUnit XML")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// collectResults drains the runner's result stream, returning the results and
// whether every test passed. Ctx (per-test evaluation detail) is dropped unless
// explain is set, matching the tester's own JSON handler.
func collectResults(runner *tester.Runner, explain bool) ([]tester.Result, bool) {
	results := []tester.Result{}
	ok := true
	for r := range runner.Run() {
		if !r.Passed {
			ok = false
		}
		if !explain {
			r.Ctx = nil
		}
		results = append(results, r)
	}
	return results, ok
}

// renderResultsMarkdown drains the runner's result stream and renders the
// outcome as a Markdown document (table + failure diffs + summary) through the
// shared iostream renderer, so human output is styled consistently with the
// rest of the CLI. It returns whether the run succeeded (no failures or errors).
//
// explain implies all: the tester forces verbose when debug context is shown.
func renderResultsMarkdown(ctx context.Context, runner *tester.Runner, all, explain bool) bool {
	if explain {
		all = true
	}

	table := mdtable.New("Result", "Test", "Time (s)")
	var details strings.Builder

	var passed, failed, errorGroups int
	var total time.Duration
	hasRows := false

	elapsed := func(d time.Duration) string { return fmt.Sprintf("%.3f", d.Seconds()) }

	for r := range runner.Run() {
		total += r.Elapsed

		// A result with no Name is a group-level (folder) outcome: a load error,
		// or a "no policies"/"no tests" skip.
		if r.Name == "" {
			switch {
			case errors.Is(r.Err, cpa.ErrNoPolicies):
				table.Row("skip", r.Group+" (no policies)", "")
			case errors.Is(r.Err, tester.ErrNoTests):
				table.Row("skip", r.Group+" (no tests)", "")
			default:
				errorGroups++
				table.Row("ERROR", r.Group, elapsed(r.Elapsed))
				if r.Err != nil {
					fmt.Fprintf(&details, "\n### Error: %s\n\n```\n%s\n```\n", r.Group, r.Err.Error())
				}
			}
			hasRows = true
			continue
		}

		if r.Passed {
			passed++
			if all {
				table.Row("ok", r.Name, elapsed(r.Elapsed))
				hasRows = true
			}
		} else {
			failed++
			table.Row("FAIL", r.Name, elapsed(r.Elapsed))
			hasRows = true
			if r.Err != nil {
				// r.Err for a failed test is the expected-vs-actual unified diff.
				fmt.Fprintf(&details, "\n### FAIL: %s\n\n```diff\n%s\n```\n", r.Name, strings.TrimRight(r.Err.Error(), "\n"))
			}
		}

		if explain && r.Ctx != nil {
			yamlCtx, err := yaml.Marshal(r.Ctx)
			if err == nil {
				fmt.Fprintf(&details, "\n### Debug Test Context: %s\n\n```yaml\n%s```\n", r.Name, string(yamlCtx))
			}
		}
	}

	var doc strings.Builder
	doc.WriteString("# Policy tests\n")
	if hasRows {
		doc.WriteString("\n" + table.Render())
	}
	doc.WriteString(details.String())
	fmt.Fprintf(&doc, "\n**%d/%d tests passed** (%ss)\n", passed, passed+failed, elapsed(total))

	iostream.PrintMarkdown(ctx, doc.String())
	return failed == 0 && errorGroups == 0
}
