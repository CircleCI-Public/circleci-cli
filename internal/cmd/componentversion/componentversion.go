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

// Package componentversion implements the "circleci component-version" command group.
package componentversion

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

type componentVersionEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ComponentID string `json:"component_id"`
	CreatedAt   string `json:"created_at"`
}

// NewComponentVersionCmd returns the top-level "circleci component-version" command group.
func NewComponentVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "component-version <command>",
		GroupID: "management",
		Short:   "Manage deploy component versions",
		Long: heredoc.Doc(`
			List and inspect CircleCI deploy component versions.

			Deploy component versions represent specific released versions
			of a component across environments.

			Also available as: circleci deploy version <command>
		`),
		RunE: cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	cmd.AddCommand(newListCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	var (
		envID   string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list <component-id>",
		Aliases: []string{"ls"},
		Short:   "List versions of a deploy component",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<component-id>%[1]s is the UUID of the deploy component whose
				versions you want to list. Component IDs are shown in the output of
				%[1]scircleci deploy component list%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			List versions of a CircleCI deploy component.

			Optionally filter by deploy environment with --environment.

			JSON fields: id, name, component_id, created_at
		`),
		Example: heredoc.Doc(`
			# List versions of a component
			$ circleci component-version list a0000000-0000-4000-8000-000000c00001

			# Filter by environment
			$ circleci component-version list a0000000-0000-4000-8000-000000c00001 --environment a0000000-0000-4000-8000-000000e00001

			# Output as JSON
			$ circleci component-version list a0000000-0000-4000-8000-000000c00001 --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runList(ctx, client, args[0], envID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&envID, "environment", "", "Filter by deploy environment ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runList(ctx context.Context, client *apiclient.Client, componentID, envID string, jsonOut bool) error {
	versions, err := client.ListComponentVersions(ctx, componentID, envID, 20)
	if err != nil {
		return cmdutil.APIErr(err, componentID,
			"deploy.component.not_found", "No deploy component found for %q.",
			"Check the component ID and try again")
	}

	entries := make([]componentVersionEntry, len(versions))
	for i, v := range versions {
		entries[i] = componentVersionEntry{
			ID:          v.ID,
			Name:        v.Attributes.Name,
			ComponentID: v.References.Component.ID,
			CreatedAt:   v.Attributes.CreatedAt.Format("2006-01-02 15:04 UTC"),
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No versions found.")
		return nil
	}

	table := mdtable.New("ID", "Version", "Created")
	for _, e := range entries {
		table.Row(e.ID, e.Name, e.CreatedAt)
	}
	iostream.PrintMarkdown(ctx, "# Component Versions\n"+table.Render())
	return nil
}
