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

package deploy

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newEnvironmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environment <command>",
		Short: "Manage deploy environments",
		Long: heredoc.Doc(`
			List and inspect CircleCI deploy environments.

			Deploy environments represent targets such as production or staging
			where components are deployed.
		`),
		RunE: cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	cmd.AddCommand(newEnvironmentListCmd())
	cmd.AddCommand(newEnvironmentGetCmd())

	return cmd
}

// --- environment list ---

type environmentEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	OrgID string `json:"org_id"`
}

func newEnvironmentListCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List deploy environments",
		Long: heredoc.Doc(`
			List deploy environments for a CircleCI organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org.

			JSON fields: id, name, org_id
		`),
		Example: heredoc.Doc(`
			# List environments for the current git remote's org
			$ circleci deploy environment list

			# List environments for a specific org
			$ circleci deploy environment list --org gh/myorg

			# Output as JSON
			$ circleci deploy environment list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runEnvironmentList(ctx, client, orgSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runEnvironmentList(ctx context.Context, client *apiclient.Client, orgSlug string, jsonOut bool) error {
	orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci deploy environment list")
	if err != nil {
		return err
	}

	environments, err := client.ListEnvironments(ctx, orgID.String(), 20)
	if err != nil {
		return cmdutil.APIErr(err, orgSlug,
			"deploy.environments.not_found", "No deploy environments found for %q.",
			"Check that CircleCI Deploys is configured for this organization")
	}

	entries := make([]environmentEntry, len(environments))
	for i, e := range environments {
		entries[i] = environmentEntry{
			ID:    e.ID,
			Name:  e.Attributes.Name,
			OrgID: e.References.Organization.ID,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No deploy environments found.")
		return nil
	}

	table := mdtable.New("ID", "Name")
	for _, e := range entries {
		table.Row(e.ID, e.Name)
	}
	iostream.PrintMarkdown(ctx, "# Deploy Environments\n"+table.Render())
	return nil
}

// --- environment get ---

func newEnvironmentGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <environment-id>",
		Short: "Get a deploy environment",
		Long: heredoc.Doc(`
			Get details about a CircleCI deploy environment by ID.

			JSON fields: id, name, org_id
		`),
		Example: heredoc.Doc(`
			# Get a deploy environment by ID
			$ circleci deploy environment get a0000000-0000-4000-8000-000000e00001

			# Output as JSON
			$ circleci deploy environment get a0000000-0000-4000-8000-000000e00001 --json

			# Filter JSON output with jq
			$ circleci deploy environment get a0000000-0000-4000-8000-000000e00001 --json --jq '.name'
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runEnvironmentGet(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runEnvironmentGet(ctx context.Context, client *apiclient.Client, envID string, jsonOut bool) error {
	environment, err := client.GetEnvironment(ctx, envID)
	if err != nil {
		return cmdutil.APIErr(err, envID,
			"deploy.environment.not_found", "No deploy environment found for %q.",
			"Check the environment ID and try again")
	}

	entry := environmentEntry{
		ID:    environment.ID,
		Name:  environment.Attributes.Name,
		OrgID: environment.References.Organization.ID,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entry)
	}

	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Deploy Environment\n\n**ID:** %s\n**Name:** %s\n**Org ID:** %s\n",
		entry.ID, entry.Name, entry.OrgID))
	return nil
}
