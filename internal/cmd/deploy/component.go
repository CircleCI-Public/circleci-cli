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

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

func newComponentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component <command>",
		Short: "Manage deploy components",
		Long: heredoc.Doc(`
			List and inspect CircleCI deploy components.

			Deploy components represent the deployable units of a project,
			such as a service, application, or library.
		`),
		RunE: cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	cmd.AddCommand(newComponentListCmd())
	cmd.AddCommand(newComponentGetCmd())

	return cmd
}

// --- component list ---

type componentEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
}

func newComponentListCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List deploy components",
		Long: heredoc.Doc(`
			List deploy components for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			JSON fields: id, name, project_id
		`),
		Example: heredoc.Doc(`
			# List components for the current git remote's project
			$ circleci deploy component list

			# List components for a specific project
			$ circleci deploy component list --project gh/myorg/myrepo

			# Output as JSON
			$ circleci deploy component list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runComponentList(ctx, client, projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runComponentList(ctx context.Context, client *apiclient.Client, projectSlug string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci deploy component list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return cmdutil.APIErr(err, projectSlug,
			"project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}

	components, err := client.ListComponents(ctx, proj.OrgID.String(), proj.ID.String(), 20)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entries := make([]componentEntry, len(components))
	for i, c := range components {
		entries[i] = componentEntry{
			ID:        c.ID,
			Name:      c.Attributes.Name,
			ProjectID: c.References.Project.ID,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No deploy components found.")
		return nil
	}

	table := mdtable.New("ID", "Name")
	for _, e := range entries {
		table.Row(e.ID, e.Name)
	}
	iostream.PrintMarkdown(ctx, "# Deploy Components\n"+table.Render())
	return nil
}

// --- component get ---

func newComponentGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <component-id>",
		Short: "Get a deploy component",
		Long: heredoc.Doc(`
			Get details about a CircleCI deploy component by ID.

			JSON fields: id, name, project_id
		`),
		Example: heredoc.Doc(`
			# Get a deploy component by ID
			$ circleci deploy component get a0000000-0000-4000-8000-000000c00001

			# Output as JSON
			$ circleci deploy component get a0000000-0000-4000-8000-000000c00001 --json

			# Filter JSON output with jq
			$ circleci deploy component get a0000000-0000-4000-8000-000000c00001 --json --jq '.name'
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runComponentGet(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runComponentGet(ctx context.Context, client *apiclient.Client, componentID string, jsonOut bool) error {
	component, err := client.GetComponent(ctx, componentID)
	if err != nil {
		return cmdutil.APIErr(err, componentID,
			"deploy.component.not_found", "No deploy component found for %q.",
			"Check the component ID and try again")
	}

	entry := componentEntry{
		ID:        component.ID,
		Name:      component.Attributes.Name,
		ProjectID: component.References.Project.ID,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entry)
	}

	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Deploy Component\n\n**ID:** %s\n**Name:** %s\n**Project ID:** %s\n",
		entry.ID, entry.Name, entry.ProjectID))
	return nil
}
