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

// Package logs implements the "circleci logs" command.
package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/logs"
)

// NewLogsCmd returns the top-level "circleci logs" command.
func NewLogsCmd() *cobra.Command {
	var (
		lastFailed  bool
		lastJob     bool
		step        string
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "logs [<job-number>]",
		Short: "Fetch job logs",
		Long: heredoc.Doc(`
			Fetch the log output for a CircleCI job.

			Provide a job number to fetch logs for a specific job. Use --last-failed
			to fetch logs from the most recent failed job on the current branch, or
			--last-job for the most recent completed job regardless of status.

			Exactly one of <job-number>, --last-failed, or --last-job is required.

			Use --step to narrow output to a single named step. Without it, all
			steps are shown with a header between each.

			JSON fields: step, status, exit_code, output
		`),
		Example: heredoc.Doc(`
			# Fetch logs for a specific job number
			$ circleci logs 123

			# Fetch logs from the latest failed job on the current branch
			$ circleci logs --last-failed

			# Fetch logs from the latest failed job on main
			$ circleci logs --last-failed --branch main

			# Fetch logs from the last completed job on the current branch
			$ circleci logs --last-job

			# Show only the output of a specific step
			$ circleci logs 123 --step "Run tests"

			# Output as JSON
			$ circleci logs 123 --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return run(ctx, client, args, lastFailed, lastJob, step, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&lastFailed, "last-failed", false, "Fetch logs from the latest failed job on the current branch")
	cmd.Flags().BoolVar(&lastJob, "last-job", false, "Fetch logs from the last completed job on the current branch")
	cmd.Flags().StringVar(&step, "step", "", "Filter output to a single step by name")
	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch for job inference (default: current branch)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func run(ctx context.Context, client *apiclient.Client, args []string, lastFailed, lastJob bool, step, projectSlug, branch string, jsonOut bool) error {
	// Validate: exactly one mode must be chosen.
	modes := 0
	if len(args) == 1 {
		modes++
	}
	if lastFailed {
		modes++
	}
	if lastJob {
		modes++
	}
	if modes == 0 {
		return clierrors.New("args.missing", "No job specified",
			"Specify a job number or use --last-failed or --last-job.").
			WithSuggestions(
				"Run 'circleci pipeline get' to see job numbers for the latest pipeline",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	if modes > 1 {
		return clierrors.New("args.conflict", "Conflicting arguments",
			"Provide exactly one of: a job number, --last-failed, or --last-job.").
			WithExitCode(clierrors.ExitBadArguments)
	}

	var (
		jobNumber   int64
		fetchLogsSp *iostream.Spin
	)

	switch {
	case len(args) == 1:
		if _, err := fmt.Sscanf(args[0], "%d", &jobNumber); err != nil {
			return clierrors.New("args.invalid_job_number", "Invalid job number",
				fmt.Sprintf("%q is not a valid job number.", args[0])).
				WithExitCode(clierrors.ExitBadArguments)
		}
		if projectSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return gitDetectErr(err)
			}
			projectSlug = info.Slug
		}

	case lastFailed, lastJob:
		effectiveBranch := branch
		if projectSlug == "" || effectiveBranch == "" {
			info, gitErr := gitremote.Detect()
			if gitErr != nil {
				return gitDetectErr(gitErr)
			}
			if projectSlug == "" {
				projectSlug = info.Slug
			}
			if effectiveBranch == "" {
				effectiveBranch = info.Branch
			}
		}

		sp1 := iostream.Spinner(ctx, true, fmt.Sprintf("Fetching latest pipeline for %s on branch %s", projectSlug, effectiveBranch))
		pipeline, err := client.GetLatestPipeline(ctx, projectSlug, effectiveBranch)
		sp1.Stop()
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}

		if lastFailed {
			jobNumber, projectSlug, err = logs.LastFailed(ctx, client, pipeline.ID)
		} else {
			jobNumber, projectSlug, err = logs.LastJob(ctx, client, pipeline.ID)
		}
		if err != nil {
			var noneFound *logs.ErrNoneFound
			if errors.As(err, &noneFound) {
				return clierrors.New("logs.none_found", "No matching job",
					noneFound.Reason).
					WithSuggestions(
						"Run 'circleci pipeline get' to inspect the pipeline's workflows and jobs",
					).
					WithExitCode(clierrors.ExitNotFound)
			}
			return apiErr(err, pipeline.ID)
		}

		fetchLogsSp = iostream.Spinner(ctx, true, fmt.Sprintf("Fetching logs for job #%d", jobNumber))
	}

	stepLogs, err := logs.ForJob(ctx, client, projectSlug, jobNumber, step)
	fetchLogsSp.Stop()
	if err != nil {
		return apiErr(err, fmt.Sprintf("job #%d", jobNumber))
	}

	if len(stepLogs) == 0 {
		return clierrors.New("logs.no_output", "No log output found",
			fmt.Sprintf("Job #%d returned no step output.", jobNumber)).
			WithSuggestions(
				"The job may still be running, or output may have expired",
				"Verify the job number with: circleci pipeline get",
			).
			WithExitCode(clierrors.ExitNotFound)
	}

	if jsonOut {
		enc := json.NewEncoder(iostream.Out(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(stepLogs)
	}

	printLogs(ctx, stepLogs)
	return nil
}

func gitDetectErr(err error) *clierrors.CLIError {
	return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
		WithSuggestions(
			"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
			"Or specify the project with --project gh/org/repo",
		).
		WithExitCode(clierrors.ExitBadArguments)
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject, "logs.not_found", "No resource found for %q.")
}

func printLogs(ctx context.Context, stepLogs []logs.StepLog) {
	for i, sl := range stepLogs {
		if i > 0 {
			iostream.Println(ctx)
		}
		if sl.Status == "failed" {
			iostream.Printf(ctx, "=== %s (failed) ===\n", sl.Name)
		} else {
			iostream.Printf(ctx, "=== %s ===\n", sl.Name)
		}
		iostream.Print(ctx, sl.Output)
	}
}
