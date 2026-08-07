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
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

type deploySettingsEntry struct {
	ID                         string `json:"id"`
	ProjectID                  string `json:"project_id"`
	AutoCancelRedundantDeploys bool   `json:"auto_cancel_redundant_deploys"`
}

func newSettingsCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Get deploy settings for a project",
		Long: heredoc.Doc(`
			Get deploy settings for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			JSON fields: id, project_id, auto_cancel_redundant_deploys
		`),
		Example: heredoc.Doc(`
			# Get deploy settings for the current git remote's project
			$ circleci deploy settings

			# Get settings for a specific project
			$ circleci deploy settings --project gh/myorg/myrepo

			# Output as JSON
			$ circleci deploy settings --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runDeploySettings(ctx, client, projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runDeploySettings(ctx context.Context, client *apiclient.Client, projectSlug string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci deploy settings --project gh/org/repo")
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

	settings, err := client.GetDeploySettings(ctx, proj.ID.String())
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entry := deploySettingsEntry{
		ID:                         settings.ID,
		ProjectID:                  settings.References.Project.ID,
		AutoCancelRedundantDeploys: settings.Attributes.AutoCancelRedundantDeploys,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entry)
	}

	autoCancelStr := "disabled"
	if entry.AutoCancelRedundantDeploys {
		autoCancelStr = "enabled"
	}

	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Deploy Settings\n\n**Project ID:** %s\n**Auto-cancel redundant deploys:** %s\n",
		entry.ProjectID, autoCancelStr))
	return nil
}
