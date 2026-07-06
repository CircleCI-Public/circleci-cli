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
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newPushCmd() *cobra.Command {
	var (
		org       string
		policyCtx string
		noPrompt  bool
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "push <path>",
		Short: "Push a policy bundle to CircleCI",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the path to a local directory of .rego policy files,
				for example, "./policies". Its contents are uploaded as the policy
				bundle, replacing the existing bundle.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Upload a directory of .rego files as a policy bundle.

			Before applying changes, a diff is shown and confirmation is
			requested. Pass --no-prompt to skip confirmation (useful in CI).

			The policy bundle replaces the existing bundle for the given
			owner and policy context.

			JSON fields: created, deleted, updated (policy names)
		`),
		Example: heredoc.Doc(`
			# Push policies in the ./policies directory
			$ circleci policy push ./policies --org gh/acme

			# Push without confirmation prompt
			$ circleci policy push ./policies --org gh/acme --no-prompt

			# Push to a custom policy context
			$ circleci policy push ./policies --org gh/acme --policy-context config

			# Output the diff as JSON
			$ circleci policy push ./policies --org gh/acme --no-prompt --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci policy push")
			if err != nil {
				return err
			}
			return runPush(ctx, client, args[0], orgID.String(), policyCtx, noPrompt, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Required: true})
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runPush(ctx context.Context, client *apiclient.Client, path, ownerID, policyCtx string, noPrompt, jsonOut bool) error {
	bundle, err := loadPolicyBundle(path)
	if err != nil {
		return err
	}

	if !noPrompt {
		diff, err := client.CreatePolicyBundle(ctx, ownerID, policyCtx, bundle, true)
		if err != nil {
			return policyAPIErr(err, ownerID)
		}
		iostream.ErrPrintln(ctx, "The following changes will be applied:")
		_ = cmdutil.WriteJSON(iostream.Err(ctx), diff)
		iostream.ErrPrintln(ctx, "")

		abortErr := clierrors.New("policy.push_aborted", "Push aborted", "User cancelled the push.").
			WithExitCode(clierrors.ExitCancelled)
		requireForceErr := clierrors.New("policy.push_requires_prompt", "Use --no-prompt in non-interactive mode",
			"Cannot confirm policy push in a non-interactive terminal.").
			WithExitCode(clierrors.ExitBadArguments)
		if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), false, "Do you wish to continue?", abortErr, requireForceErr); err != nil {
			return err
		}
	}

	result, err := client.CreatePolicyBundle(ctx, ownerID, policyCtx, bundle, false)
	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	iostream.ErrPrintln(ctx, "Policy bundle pushed successfully.")

	if jsonOut {
		return iostream.PrintJSON(ctx, result)
	}
	return cmdutil.WriteJSON(iostream.Out(ctx), result)
}
