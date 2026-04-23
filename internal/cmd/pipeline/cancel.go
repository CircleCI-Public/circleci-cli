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
	"errors"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/pipeline"
)

func newCancelCmd() *cobra.Command {
	var projectSlug string
	var force bool

	cmd := &cobra.Command{
		Use:   "cancel <pipeline-number-or-id>",
		Short: "Cancel a pipeline",
		Long: heredoc.Doc(`
			Cancel a running CircleCI pipeline by number or UUID.

			Cancelling a pipeline stops all in-progress workflows and jobs
			within it. Workflows that have already completed are unaffected.

			When using a pipeline number, the project is inferred from the
			git remote unless overridden with --project.

			In a terminal, you will be prompted to confirm before cancelling.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Cancel a pipeline by number (with confirmation)
			$ circleci pipeline cancel 75

			# Cancel a pipeline by UUID without confirmation
			$ circleci pipeline cancel 5034460f-c7c4-4c43-9457-de07e2029e7b --force

			# Cancel the latest pipeline on a branch
			$ circleci pipeline list --branch main --json | jq -r '.[0].id' | xargs circleci pipeline cancel --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "pipeline-number-or-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runPipelineCancel(ctx, client, streams, args[0], projectSlug, force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used when cancelling by number")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runPipelineCancel(ctx context.Context, client *apiclient.Client, streams iostream.Streams, arg, projectSlug string, force bool) error {
	pipelineID := arg
	displayName := arg

	if looksLikeNumber(arg) {
		number, _ := strconv.ParseInt(arg, 10, 64)
		if projectSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
					WithSuggestions(
						"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
						"Or specify the project: circleci pipeline cancel "+arg+" --project gh/org/repo",
					).
					WithExitCode(clierrors.ExitBadArguments)
			}
			projectSlug = info.Slug
		}
		p, err := client.GetPipelineByNumber(ctx, projectSlug, number)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, arg))
		}
		pipelineID = p.ID
		displayName = fmt.Sprintf("#%d", p.Number)
	}

	if !force {
		if streams.IsInteractive() {
			prompt := fmt.Sprintf("Cancel pipeline %s? In-progress jobs will be stopped.", displayName)
			if !streams.Confirm(prompt) {
				return clierrors.New("pipeline.cancel_aborted", "Cancellation aborted",
					"Pipeline cancellation was not confirmed.").
					WithExitCode(clierrors.ExitCancelled)
			}
		} else {
			return clierrors.New("pipeline.cancel_requires_force", "Cancellation requires --force",
				fmt.Sprintf("Cancelling pipeline %s will stop all in-progress jobs.", displayName)).
				WithSuggestions("Pass --force (-f) to confirm cancellation in non-interactive mode").
				WithExitCode(clierrors.ExitCancelled)
		}
	}

	if err := pipeline.Cancel(ctx, client, pipelineID); err != nil {
		var nothingToCancel *pipeline.ErrNothingToCancel
		if errors.As(err, &nothingToCancel) {
			return clierrors.New("pipeline.not_running", "Pipeline is not running",
				fmt.Sprintf("Pipeline %s has no active workflows to cancel.", displayName)).
				WithSuggestions("The pipeline may have already completed or been cancelled.").
				WithExitCode(clierrors.ExitBadArguments)
		}
		return apiErr(err, displayName)
	}

	streams.Printf("%s Cancelled pipeline %s\n", streams.Symbol("✓", "OK:"), displayName)
	return nil
}
