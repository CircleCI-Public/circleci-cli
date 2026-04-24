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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newRerunCmd() *cobra.Command {
	var fromFailed bool

	cmd := &cobra.Command{
		Use:   "rerun <workflow-id>",
		Short: "Rerun a workflow",
		Long: heredoc.Doc(`
			Rerun a CircleCI workflow.

			By default all jobs in the workflow are rerun from scratch. Use
			--from-failed to rerun only the jobs that failed, leaving successful
			jobs untouched.

			Workflow IDs are shown in the output of 'circleci pipeline get'.
		`),
		Example: heredoc.Doc(`
			# Rerun all jobs in a workflow from scratch
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Rerun only the failed jobs
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b --from-failed

			# Find a workflow ID from the latest pipeline
			$ circleci pipeline get --json | jq -r '.workflows[].id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runRerun(ctx, client, args[0], fromFailed)
		},
	}

	cmd.Flags().BoolVar(&fromFailed, "from-failed", false, "Rerun only failed jobs")
	return cmd
}

func runRerun(ctx context.Context, client *apiclient.Client, id string, fromFailed bool) error {
	if err := client.RerunWorkflow(ctx, id, fromFailed); err != nil {
		return apiErr(err, id)
	}

	if fromFailed {
		iostream.Printf(ctx, "Rerunning failed jobs in workflow %s\n", id)
	} else {
		iostream.Printf(ctx, "Rerunning workflow %s from scratch\n", id)
	}
	return nil
}
