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

package run

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	runpkg "github.com/CircleCI-Public/circleci-cli/internal/run"
)

func newCancelCmd() *cobra.Command {
	var projectSlug string
	var force bool

	cmd := &cobra.Command{
		Use:   "cancel <run-number-or-id>",
		Short: "Cancel a run",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<run-number-or-id> identifies the run to cancel. It can be:
				- a run UUID (shown in "circleci run list --json")
				- a run number (shown in "circleci run list"); the project is
				  inferred from the git remote unless overridden with --project
			`),
		},
		Long: heredoc.Doc(`
			Cancel a running CircleCI run by number or UUID.

			Cancelling a run stops all in-progress workflows and jobs
			within it. Workflows that have already completed are unaffected.

			When using a run number, the project is inferred from the
			git remote unless overridden with --project.

			In a terminal, you will be prompted to confirm before cancelling.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Cancel a run by number (with confirmation)
			$ circleci run cancel 75

			# Cancel a run by UUID without confirmation
			$ circleci run cancel 5034460f-c7c4-4c43-9457-de07e2029e7b --force

			# Cancel the latest run on a branch
			$ circleci run list --branch main --json --jq '.[0].id' | xargs circleci run cancel --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "run-number-or-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runCancel(ctx, client, args[0], projectSlug, force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used when cancelling by number")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runCancel(ctx context.Context, client *apiclient.Client, arg, projectSlug string, force bool) error {
	runID := arg
	displayName := arg

	if looksLikeNumber(arg) {
		number, _ := strconv.ParseInt(arg, 10, 64)
		if projectSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return cmdutil.GitDetectErr(err, "Or specify the project: circleci run cancel "+arg+" --project gh/org/repo")
			}
			projectSlug = info.Slug
		}
		r, err := client.GetPipelineByNumber(ctx, projectSlug, number)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, arg))
		}
		runID = r.ID
		displayName = fmt.Sprintf("#%d", r.Number)
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Cancel run %s? In-progress jobs will be stopped.", displayName),
		clierrors.New("run.cancel_aborted", "Cancellation aborted",
			"Run cancellation was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("run.cancel_requires_force", "Cancellation requires --force",
			fmt.Sprintf("Cancelling run %s will stop all in-progress jobs.", displayName)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := runpkg.Cancel(ctx, client, runID); err != nil {
		if _, ok := errors.AsType[*runpkg.ErrNothingToCancel](err); ok {
			return clierrors.New("run.not_running", "Run is not running",
				fmt.Sprintf("Run %s has no active workflows to cancel.", displayName)).
				WithSuggestions("The run may have already completed or been cancelled.").
				WithExitCode(clierrors.ExitBadArguments)
		}
		return apiErr(err, displayName)
	}

	iostream.Printf(ctx, "%s Cancelled run %s\n", iostream.SymbolOK(ctx), displayName)
	return nil
}
