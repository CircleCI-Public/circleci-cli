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

package namespace

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		orgID   string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create <name> --org-id <uuid>",
		Short: "Create a namespace",
		Long: heredoc.Doc(`
			Create a CircleCI orb namespace for an organization.

			Each organization may claim one namespace. Namespace names must
			be globally unique across CircleCI. All orbs published in a
			namespace are world-readable.

			JSON fields: id, name
		`),
		Example: heredoc.Doc(`
			# Create a namespace for an organization
			$ circleci namespace create myorg --org-id 00000000-0000-0000-0000-000000000001

			# Create a namespace and output JSON
			$ circleci namespace create myorg --org-id 00000000-0000-0000-0000-000000000001 --json

			# Capture just the namespace ID
			$ circleci namespace create myorg --org-id 00000000-0000-0000-0000-000000000001 --json --jq '.id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "name"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runCreate(ctx, client, args[0], orgID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "organization UUID to claim the namespace for (required)")
	_ = cmd.MarkFlagRequired("org-id")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runCreate(ctx context.Context, client *apiclient.Client, name, orgID string, jsonOut bool) error {
	ns, err := client.CreateNamespace(ctx, name, orgID)
	if err != nil {
		return apiErr(err, name)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, namespaceOutput{ID: ns.ID, Name: ns.Name})
	}

	iostream.Printf(ctx, "%s Created namespace %q (%s)\n", iostream.SymbolOK(ctx), ns.Name, ns.ID)
	iostream.Printf(ctx, "Orbs published in this namespace are world-readable.\n")
	return nil
}
