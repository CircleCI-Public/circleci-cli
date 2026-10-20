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
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	runpkg "github.com/CircleCI-Public/circleci-cli/internal/run"
)

func newApproveCmd() *cobra.Command {
	var (
		projectSlug string
		job         string
		force       bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "approve <run-number-or-id>",
		Short: "Approve an approval job that is on hold",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<run-number-or-id>%[1]s is a run UUID, or a run number from %[1]scircleci run list%[1]s
				looked up in the git remote's project (or %[1]s--project%[1]s).
			`, "`"),
			// An approval can't be undone, and the jobs it releases are often deploys.
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Approve an approval job on hold, so the jobs that require it start. With
			several on hold, name one with --job or pick it from a list in a terminal.

			JSON fields: run_id, workflow_id, workflow_name, job_id, job_name
		`),
		Example: heredoc.Doc(`
			# Approve the approval job on hold in run 75 (with confirmation)
			$ circleci run approve 75

			# Approve a specific approval job by name, without confirmation
			$ circleci run approve 75 --job hold-deploy --force

			# Approve the hold on the latest run on main, by run UUID
			$ circleci run list --branch main --json --jq '.[0].id' | xargs circleci run approve --force
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
			return runApprove(ctx, client, args[0], projectSlug, job, force, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used when approving by number")
	cmd.Flags().StringVar(&job, "job", "", "name or UUID of the approval job to approve")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type approveJSONOutput struct {
	RunID        uuid.UUID `json:"run_id"`
	WorkflowID   uuid.UUID `json:"workflow_id"`
	WorkflowName string    `json:"workflow_name"`
	JobID        uuid.UUID `json:"job_id"`
	JobName      string    `json:"job_name"`
}

func runApprove(ctx context.Context, client *apiclient.Client, arg, projectSlug, job string, force, jsonOut bool) error {
	runID, displayName, err := resolveRunRef(ctx, client, "approve", arg, projectSlug)
	if err != nil {
		return err
	}

	pending, err := runpkg.PendingApprovals(ctx, client, runID)
	if err != nil {
		return apiErr(err, displayName)
	}

	approval, picked, err := chooseApproval(ctx, pending, job, displayName)
	if err != nil {
		return err
	}

	// Picking an approval from the list is itself the confirmation.
	if !picked {
		if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
			fmt.Sprintf("Approve %s in run %s? Jobs that require it will start.", approval.Label(), displayName),
			approveAbortedErr(),
			clierrors.New("run.approve_requires_force", "Approval requires --force",
				fmt.Sprintf("Approving %s in run %s will start the jobs that require it.", approval.Label(), displayName)).
				WithExitCode(clierrors.ExitCancelled),
		); err != nil {
			return err
		}
	}

	if err := runpkg.Approve(ctx, client, approval); err != nil {
		return cmdutil.APIErr(err, approval.Label(),
			"run.approval_not_found", "Approval job %s was not found.",
			"It may have just been approved or cancelled. Check: circleci run get "+arg)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, approveJSONOutput{
			RunID:        runID,
			WorkflowID:   approval.WorkflowID,
			WorkflowName: approval.WorkflowName,
			JobID:        approval.JobID,
			JobName:      approval.JobName,
		})
	}
	iostream.Printf(ctx, "%s Approved %s in run %s\n", iostream.SymbolOK(ctx), approval.Label(), displayName)
	return nil
}

// chooseApproval picks the approval to approve from those on hold in a run:
// the one --job names, the only one there is, or — with several and a terminal
// — the one the user picks. picked reports that the user chose it from a list.
func chooseApproval(ctx context.Context, pending []runpkg.PendingApproval, job, displayName string) (approval runpkg.PendingApproval, picked bool, err error) {
	if len(pending) == 0 {
		return approval, false, clierrors.New("run.no_pending_approval", "No approval on hold",
			fmt.Sprintf("Run %s has no approval jobs waiting to be approved.", displayName)).
			WithSuggestions(
				"An approval job can only be approved once the jobs it requires have finished",
				"It may already have been approved, or the run may have ended",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}

	candidates := pending
	if job != "" {
		candidates = runpkg.MatchApprovals(pending, job)
		if len(candidates) == 0 {
			return approval, false, clierrors.New("run.approval_not_found", "Approval job not found",
				fmt.Sprintf("Run %s has no approval job %q on hold. On hold: %s.", displayName, job, labels(pending))).
				WithSuggestions("Pass one of the approval jobs on hold to --job").
				WithExitCode(clierrors.ExitNotFound)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], false, nil
	}

	if !iostream.IsInteractive(ctx) {
		return approval, false, clierrors.New("run.approval_ambiguous", "Several approvals on hold",
			fmt.Sprintf("Run %s has %d matching approval jobs on hold: %s.", displayName, len(candidates), labels(candidates))).
			WithSuggestions("Pass the job's name, or its UUID when names repeat, to --job").
			WithExitCode(clierrors.ExitBadArguments)
	}

	options := make([]string, len(candidates))
	for i, a := range candidates {
		options[i] = a.Label()
	}
	idx, err := iostream.PromptSelect(ctx, "Which approval job should be approved?", options)
	if err != nil {
		return approval, false, err
	}
	if idx < 0 {
		return approval, false, approveAbortedErr()
	}
	return candidates[idx], true, nil
}

func approveAbortedErr() *clierrors.CLIError {
	return clierrors.New("run.approve_aborted", "Approval aborted",
		"The approval was not confirmed.").
		WithExitCode(clierrors.ExitCancelled)
}

// labels lists approvals for an error message, with each job's UUID so that
// approvals sharing a name can still be told apart for --job.
func labels(approvals []runpkg.PendingApproval) string {
	parts := make([]string, len(approvals))
	for i, a := range approvals {
		parts[i] = fmt.Sprintf("%s [%s]", a.Label(), a.JobID)
	}
	return strings.Join(parts, ", ")
}
