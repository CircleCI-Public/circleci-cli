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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		org     string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create <name> --org <org>",
		Short: "Create a namespace",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				name is the namespace name to create. It must be globally unique
				across CircleCI, e.g. "myorg".
			`),
		},
		Long: heredoc.Doc(`
			Create a CircleCI orb namespace for an organization.

			Each organization may claim one namespace. Namespace names must
			be globally unique across CircleCI. All orbs published in a
			namespace are world-readable.

			JSON fields: id, name
		`),
		Example: heredoc.Doc(`
			# Create a namespace for an organization
			$ circleci namespace create myorg --org gh/acme

			# Create a namespace and output JSON
			$ circleci namespace create myorg --org gh/acme --json

			# Capture just the namespace ID
			$ circleci namespace create myorg --org gh/acme --json --jq '.id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "name"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci namespace create")
			if err != nil {
				return err
			}
			return runCreate(ctx, client, args[0], orgID.String(), jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "to claim the namespace for", Required: true})
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runCreate(ctx context.Context, client *apiclient.Client, name, orgID string, jsonOut bool) error {
	ns, err := client.CreateNamespace(ctx, apiclient.CreateNamespaceRequest{Name: name, OrgID: orgID})
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
