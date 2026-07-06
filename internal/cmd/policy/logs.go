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
	"encoding/json"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newLogsCmd() *cobra.Command {
	var (
		org          string
		policyCtx    string
		after        string
		before       string
		status       string
		branch       string
		projectID    string
		outputFile   string
		policyBundle bool
		jsonOut      bool
	)

	cmd := &cobra.Command{
		Use:   "logs [decision-id]",
		Short: "Get policy decision logs",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<decision-id>%[1]s is optional and retrieves a single log entry.
				When omitted, all logs are returned (paginated automatically).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Retrieve policy decision logs for an owner.

			Without a decision ID, returns all logs (paginated automatically).
			Pass a decision ID to retrieve a single log entry. Use --policy-bundle
			to retrieve only the policy bundle snapshot for a given decision.

			Logs can be filtered by status, branch, project, and time range.
			Use --out to write results to a file.

			JSON fields: id, status, created_at, org_id, project_id, branch,
			             build_number, policies, decision, metadata
		`),
		Example: heredoc.Doc(`
			# Get all decision logs
			$ circleci policy logs --org gh/acme

			# Get a specific decision log
			$ circleci policy logs abc123 --org gh/acme

			# Filter by status and branch
			$ circleci policy logs --org gh/acme --status HARD_FAIL --branch main

			# Write output to a file
			$ circleci policy logs --org gh/acme --out logs.json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			decisionID := ""
			if len(args) == 1 {
				decisionID = args[0]
			}
			req := apiclient.DecisionLogsRequest{
				Status:    status,
				Branch:    branch,
				ProjectID: projectID,
			}
			if after != "" {
				t, err := parseTime(after)
				if err != nil {
					return clierrors.New("policy.logs_invalid_after", "Invalid --after value", err.Error()).
						WithSuggestions("Use RFC3339 format: 2006-01-02T15:04:05Z").
						WithExitCode(clierrors.ExitBadArguments)
				}
				req.After = &t
			}
			if before != "" {
				t, err := parseTime(before)
				if err != nil {
					return clierrors.New("policy.logs_invalid_before", "Invalid --before value", err.Error()).
						WithSuggestions("Use RFC3339 format: 2006-01-02T15:04:05Z").
						WithExitCode(clierrors.ExitBadArguments)
				}
				req.Before = &t
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci policy logs")
			if err != nil {
				return err
			}
			return runLogs(ctx, client, orgID.String(), policyCtx, decisionID, outputFile, policyBundle, jsonOut, req)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Required: true})
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmd.Flags().StringVar(&after, "after", "", "Return logs created after this time (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&before, "before", "", "Return logs created before this time (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by decision status (PASS, SOFT_FAIL, HARD_FAIL, ERROR)")
	cmd.Flags().StringVar(&branch, "branch", "", "Filter by branch name")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project ID")
	cmd.Flags().StringVar(&outputFile, "out", "", "Write output to this file instead of stdout")
	cmd.Flags().BoolVar(&policyBundle, "policy-bundle", false, "Retrieve the policy bundle snapshot for the given decision ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runLogs(ctx context.Context, client *apiclient.Client, ownerID, policyCtx, decisionID, outputFile string, policyBundle, jsonOut bool, req apiclient.DecisionLogsRequest) error {
	var (
		result any
		err    error
	)

	if decisionID != "" {
		result, err = client.GetDecisionLog(ctx, ownerID, policyCtx, decisionID, policyBundle)
		if err != nil {
			return policyAPIErr(err, decisionID)
		}
	} else {
		spin := iostream.Spinner(ctx, !jsonOut, "Fetching decision logs...")
		defer spin.Stop()

		var all []json.RawMessage
		for {
			batch, err := client.GetDecisionLogs(ctx, ownerID, policyCtx, req)
			if err != nil {
				return policyAPIErr(err, ownerID)
			}
			if len(batch) == 0 {
				break
			}
			all = append(all, batch...)
			req.Offset = len(all)
		}
		spin.Stop()
		result = all
	}

	if outputFile != "" {
		f, err := os.Create(outputFile) //nolint:gosec // outputFile is user-supplied
		if err != nil {
			return clierrors.New("policy.logs_write_failed", "Could not open output file", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		defer func() { _ = f.Close() }()
		iostream.ErrPrintf(ctx, "Writing logs to %s...\n", outputFile)
		return cmdutil.WriteJSON(f, result)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, result)
	}
	return cmdutil.WriteJSON(iostream.Out(ctx), result)
}

var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2006/01/02",
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range timeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, clierrors.New("policy.invalid_time", "Invalid time format", s+": unrecognised time format").
		WithSuggestions("Use RFC3339 format: 2006-01-02T15:04:05Z, or date only: 2006-01-02").
		WithExitCode(clierrors.ExitBadArguments)
}
