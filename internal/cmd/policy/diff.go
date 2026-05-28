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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newDiffCmd() *cobra.Command {
	var (
		ownerID   string
		policyCtx string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "diff <path>",
		Short: "Show diff between local and remote policy bundles",
		Long: heredoc.Doc(`
			Compare a local directory of .rego files against the remote policy
			bundle without making any changes.

			JSON fields: created, deleted, updated (policy names)
		`),
		Example: heredoc.Doc(`
			# Diff policies in ./policies against the remote bundle
			$ circleci policy diff ./policies --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f

			# Diff against a custom policy context
			$ circleci policy diff ./policies --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --policy-context config

			# Output diff as JSON for scripting
			$ circleci policy diff ./policies --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runDiff(ctx, client, args[0], ownerID, policyCtx, jsonOut)
		},
	}

	cmd.Flags().StringVar(&ownerID, "owner-id", "", "Organization UUID (required)")
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	_ = cmd.MarkFlagRequired("owner-id")

	return cmd
}

func runDiff(ctx context.Context, client *apiclient.Client, path, ownerID, policyCtx string, jsonOut bool) error {
	bundle, err := loadPolicyBundle(path)
	if err != nil {
		return err
	}

	diff, err := client.CreatePolicyBundle(ctx, ownerID, policyCtx, bundle, true)
	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, diff)
	}
	return cmdutil.WriteJSON(iostream.Out(ctx), diff)
}
