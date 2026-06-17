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

func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings <command>",
		Short: "Manage policy enforcement settings",
		Long: heredoc.Doc(`
			Get or update policy enforcement settings for an organization.

			Policy enforcement controls whether pipeline configs are evaluated
			against the policy bundle before running.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "Targeted commands",
		newSettingsGetCmd(),
		newSettingsSetCmd(),
	)

	return cmd
}

func newSettingsGetCmd() *cobra.Command {
	var (
		org       string
		policyCtx string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get policy enforcement settings",
		Long: heredoc.Doc(`
			Retrieve the current policy enforcement settings for an organization.

			JSON fields: enabled
		`),
		Example: heredoc.Doc(`
			# Get policy enforcement settings
			$ circleci policy settings get --org gh/acme

			# Output as JSON
			$ circleci policy settings get --org gh/acme --json

			# Use with jq to extract the enabled field
			$ circleci policy settings get --org gh/acme --json --jq '.enabled'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci policy settings get")
			if err != nil {
				return err
			}
			return runSettingsGet(ctx, client, orgID.String(), policyCtx, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Required: true})
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func newSettingsSetCmd() *cobra.Command {
	var (
		org       string
		policyCtx string
		enabled   bool
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update policy enforcement settings",
		Long: heredoc.Doc(`
			Enable or disable policy enforcement for an organization.

			When enabled, pipeline configs are evaluated against the policy bundle
			before each run. Configs that produce a HARD_FAIL decision are blocked.

			JSON fields: enabled
		`),
		Example: heredoc.Doc(`
			# Enable policy enforcement
			$ circleci policy settings set --org gh/acme --enabled

			# Disable policy enforcement
			$ circleci policy settings set --org gh/acme --enabled=false

			# Output result as JSON
			$ circleci policy settings set --org gh/acme --enabled --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci policy settings set")
			if err != nil {
				return err
			}
			return runSettingsSet(ctx, client, orgID.String(), policyCtx, enabled, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Required: true})
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Enable policy enforcement")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	_ = cmd.MarkFlagRequired("enabled")

	return cmd
}

func runSettingsGet(ctx context.Context, client *apiclient.Client, ownerID, policyCtx string, jsonOut bool) error {
	s, err := client.GetPolicySettings(ctx, ownerID, policyCtx)
	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, s)
	}
	iostream.Printf(ctx, "Enabled: %v\n", s.Enabled)
	return nil
}

func runSettingsSet(ctx context.Context, client *apiclient.Client, ownerID, policyCtx string, enabled, jsonOut bool) error {
	s, err := client.SetPolicySettings(ctx, ownerID, policyCtx, apiclient.DecisionSettings{Enabled: enabled})
	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, s)
	}
	iostream.Printf(ctx, "Policy enforcement %s.\n", enabledLabel(enabled))
	_ = s
	return nil
}

func enabledLabel(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
