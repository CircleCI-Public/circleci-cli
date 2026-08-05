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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newRerunCmd() *cobra.Command {
	var (
		fromFailed bool
		jsonOut    bool
	)

	cmd := &cobra.Command{
		Use:   "rerun <workflow-id>",
		Short: "Rerun a workflow",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<workflow-id>%[1]s is the UUID of the workflow to rerun. Workflow IDs are
				shown in the output of %[1]scircleci run get%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			All jobs rerun from scratch unless --from-failed is given, which reruns
			only the jobs that failed. Either way a new workflow is created, and its
			ID is reported so you can follow the run.

			JSON fields: workflow_id, rerun_from, from_failed
		`),
		Example: heredoc.Doc(`
			# Rerun all jobs in a workflow from scratch
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Rerun only the failed jobs
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b --from-failed

			# Find a workflow ID from the latest run
			$ circleci run get --json --jq '.workflows[].id'

			# Rerun and capture the new workflow's ID
			$ circleci workflow rerun <workflow-id> --from-failed --json --jq .workflow_id
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runRerun(ctx, client, args[0], fromFailed, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&fromFailed, "from-failed", false, "Rerun only failed jobs")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	return cmd
}

// rerunJSONOutput is the --json shape. rerun_from is echoed back because the new
// workflow ID alone does not say what it came from.
type rerunJSONOutput struct {
	WorkflowID string `json:"workflow_id"`
	RerunFrom  string `json:"rerun_from"`
	FromFailed bool   `json:"from_failed"`
}

func runRerun(ctx context.Context, client *apiclient.Client, id string, fromFailed, jsonOut bool) error {
	newID, err := client.RerunWorkflow(ctx, id, fromFailed)
	if err != nil {
		return apiErr(err, id)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, rerunJSONOutput{
			WorkflowID: newID,
			RerunFrom:  id,
			FromFailed: fromFailed,
		})
	}

	// Lead with the new workflow's ID rather than the one that was passed in: the
	// old one is spent, and the new one is what `workflow get` or `run watch`
	// needs next.
	if fromFailed {
		iostream.Printf(ctx, "Rerunning failed jobs from %s as workflow %s\n", id, newID)
	} else {
		iostream.Printf(ctx, "Rerunning %s from scratch as workflow %s\n", id, newID)
	}
	return nil
}
