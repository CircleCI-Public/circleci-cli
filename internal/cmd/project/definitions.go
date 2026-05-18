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

package project

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newDefinitionsCmd() *cobra.Command {
	var (
		projectSlug string
		projectID   string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "definitions",
		Short: "List pipeline definitions for a project",
		Long: heredoc.Doc(`
			List all pipeline definitions for a CircleCI project.

			The project is resolved from --project-id if provided; otherwise the
			project slug (--project or git remote) is used to look up the project UUID.

			JSON fields: id, name, description, created_at,
			             config_source.provider, config_source.file_path,
			             config_source.repo.external_id, config_source.repo.full_name,
			             checkout_source.provider,
			             checkout_source.repo.external_id, checkout_source.repo.full_name
		`),
		Example: heredoc.Doc(`
			# List pipeline definitions for the current repository's project
			$ circleci project definitions

			# List pipeline definitions for a specific project
			$ circleci project definitions --project gh/myorg/myrepo

			# Output as JSON for scripting
			$ circleci project definitions --json

			# Filter by config provider
			$ circleci project definitions --json --jq '.[] | select(.config_source.provider == "github_app")'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runDefinitions(ctx, client, projectSlug, projectID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project UUID (overrides --project)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type definitionOutput struct {
	ID             string                              `json:"id"`
	Name           string                              `json:"name"`
	Description    string                              `json:"description,omitempty"`
	CreatedAt      string                              `json:"created_at"`
	ConfigSource   *apiclient.PipelineDefinitionSource `json:"config_source,omitempty"`
	CheckoutSource *apiclient.PipelineDefinitionSource `json:"checkout_source,omitempty"`
}

func runDefinitions(ctx context.Context, client *apiclient.Client, projectSlug, projectID string, jsonOut bool) error {
	resolvedProjectID, err := cmdutil.ResolveProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	defs, err := client.ListPipelineDefinitions(ctx, resolvedProjectID)
	if err != nil {
		return cmdutil.APIErr(err, resolvedProjectID,
			"pipeline_definition.list_failed",
			"Failed to list pipeline definitions for project %q.",
			"Check that the project ID is correct",
			"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations")
	}

	out := make([]definitionOutput, len(defs))
	for i, d := range defs {
		out[i] = definitionOutput{
			ID:             d.ID,
			Name:           d.Name,
			Description:    d.Description,
			CreatedAt:      d.CreatedAt.Format(time.RFC3339),
			ConfigSource:   d.ConfigSource,
			CheckoutSource: d.CheckoutSource,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.ErrPrintln(ctx, "No pipeline definitions found.")
		return nil
	}

	tbl := mdtable.New("ID", "Name", "Config Provider", "Config File", "Checkout Provider")
	for _, d := range out {
		configProvider, configFile := "", ""
		if d.ConfigSource != nil {
			configProvider = d.ConfigSource.Provider
			configFile = d.ConfigSource.FilePath
		}
		checkoutProvider := ""
		if d.CheckoutSource != nil {
			checkoutProvider = d.CheckoutSource.Provider
		}
		tbl.Row(d.ID, d.Name, configProvider, configFile, checkoutProvider)
	}
	heading := "# Pipeline Definitions"
	if len(out) > 3 {
		heading = fmt.Sprintf("# Pipeline Definitions (%d)", len(out))
	}
	iostream.PrintMarkdown(ctx, heading+"\n"+tbl.Render())
	return nil
}
