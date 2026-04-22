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

package workflow

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newCancelCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cancel <workflow-id>",
		Short: "Cancel a running workflow",
		Long: heredoc.Doc(`
			Cancel a running CircleCI workflow.

			Any in-progress jobs will be stopped. Jobs that have already
			completed are not affected.

			Workflow IDs are shown in the output of 'circleci pipeline get'.

			In a terminal, you will be prompted to confirm before cancelling.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Cancel a running workflow (with confirmation)
			$ circleci workflow cancel 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Cancel without confirmation
			$ circleci workflow cancel 5034460f-c7c4-4c43-9457-de07e2029e7b --force

			# Find a running workflow ID from the latest pipeline and cancel it
			$ circleci pipeline get --json | jq -r '.workflows[] | select(.status=="running") | .id' | xargs circleci workflow cancel --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runCancel(ctx, streams, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

func runCancel(ctx context.Context, streams iostream.Streams, id string, force bool) error {
	if !force {
		if streams.IsInteractive() {
			prompt := fmt.Sprintf("Cancel workflow %s? In-progress jobs will be stopped.", id)
			if !streams.Confirm(prompt) {
				return clierrors.New("workflow.cancel_aborted", "Cancellation aborted",
					"Workflow cancellation was not confirmed.").
					WithExitCode(clierrors.ExitCancelled)
			}
		} else {
			return clierrors.New("workflow.cancel_requires_force", "Cancellation requires --force",
				fmt.Sprintf("Cancelling workflow %s will stop all in-progress jobs.", id)).
				WithSuggestions("Pass --force (-f) to confirm cancellation in non-interactive mode").
				WithExitCode(clierrors.ExitCancelled)
		}
	}

	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	if err := client.CancelWorkflow(ctx, id); err != nil {
		return apiErr(err, id)
	}

	streams.Printf("%s Cancelled workflow %s\n", streams.Symbol("✓", "OK:"), id)
	return nil
}
