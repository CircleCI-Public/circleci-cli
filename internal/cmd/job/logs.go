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

package job

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/logs"
)

func newLogsCmd() *cobra.Command {
	var (
		projectSlug string
		step        string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "logs <job-number>",
		Short: "Fetch logs for a job",
		Long: heredoc.Doc(`
			Fetch the log output for a specific job number.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			To infer the job from the latest pipeline, use 'circleci logs'
			with --last-failed or --last-job.

			JSON fields: step, status, exit_code, output
		`),
		Example: heredoc.Doc(`
			# Fetch logs for job number 123
			$ circleci job logs 123

			# Filter to a specific step
			$ circleci job logs 123 --step "Run tests"

			# Specify the project explicitly
			$ circleci job logs 123 --project gh/myorg/myrepo

			# Output as JSON
			$ circleci job logs 123 --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-number"); cliErr != nil {
				return cliErr
			}
			streams := iostream.FromCmd(cmd)
			var jobNumber int64
			if _, err := fmt.Sscanf(args[0], "%d", &jobNumber); err != nil {
				return clierrors.New("args.invalid_job_number", "Invalid job number",
					fmt.Sprintf("%q is not a valid job number.", args[0])).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return runJobLogs(ctx, client, streams, jobNumber, projectSlug, step, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&step, "step", "", "Filter output to a single step by name")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func runJobLogs(ctx context.Context, client *apiclient.Client, streams iostream.Streams, jobNumber int64, projectSlug, step string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify the project: circleci job logs <number> --project gh/org/repo",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}
	stepLogs, err := logs.ForJob(ctx, client, projectSlug, jobNumber, step)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("logs.not_found", "Job not found",
				fmt.Sprintf("No job #%d found in project %q.", jobNumber, projectSlug)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return clierrors.New("api.error", "CircleCI API error", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
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
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(stepLogs)
	}

	for i, sl := range stepLogs {
		if i > 0 {
			streams.Println()
		}
		if sl.Status == "failed" {
			streams.Printf("=== %s (failed) ===\n", sl.Name)
		} else {
			streams.Printf("=== %s ===\n", sl.Name)
		}
		streams.Print(sl.Output)
	}
	return nil
}
