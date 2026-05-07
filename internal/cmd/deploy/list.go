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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/mdtable"
)

func newListCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent deploys",
		Long: heredoc.Doc(`
			List deploys for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Each deploy shows the component,
			version, status, type, and when it was created.

			JSON fields: id, component_name, version, type, status, is_rollback,
			             pipeline_id, workflow_id, created_at, ended_at
		`),
		Example: heredoc.Doc(`
			# List the 10 most recent deploys (auto-detect project from git remote)
			$ circleci deploy list

			# List for a specific project
			$ circleci deploy list --project gh/myorg/myrepo

			# Output as JSON for scripting
			$ circleci deploy list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runList(ctx, client, projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type deployEntry struct {
	ID            string `json:"id"`
	ComponentName string `json:"component_name"`
	Version       string `json:"version"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	IsRollback    bool   `json:"is_rollback"`
	PipelineID    string `json:"pipeline_id,omitempty"`
	WorkflowID    string `json:"workflow_id,omitempty"`
	CreatedAt     string `json:"created_at"`
	EndedAt       string `json:"ended_at,omitempty"`
}

func runList(ctx context.Context, client *apiclient.Client, projectSlug string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci deploy list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectInfo(ctx, projectSlug)
	if err != nil {
		return cmdutil.APIErr(err, projectSlug,
			"project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this checkout to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}

	deploys, err := client.ListDeploys(ctx, proj.ID, proj.OrganizationID, 10)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entries := make([]deployEntry, len(deploys))
	for i, d := range deploys {
		version := ""
		if d.TargetVersion != nil {
			version = d.TargetVersion.Name
		}
		endedAt := ""
		if !d.EndedAt.IsZero() {
			endedAt = d.EndedAt.Format("2006-01-02 15:04 UTC")
		}
		entries[i] = deployEntry{
			ID:            d.ID,
			ComponentName: d.ComponentName,
			Version:       version,
			Type:          d.Type,
			Status:        d.Status,
			IsRollback:    d.PlanIsRollback,
			PipelineID:    d.PipelineID,
			WorkflowID:    d.WorkflowID,
			CreatedAt:     d.CreatedAt.Format("2006-01-02 15:04 UTC"),
			EndedAt:       endedAt,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No deploys found.")
		return nil
	}

	printList(ctx, entries)
	return nil
}

func printList(ctx context.Context, entries []deployEntry) {
	table := mdtable.New("Component", "Version", "Type", "Status", "Created")
	for _, e := range entries {
		table.Row(e.ComponentName, e.Version, strings.ToLower(e.Type), strings.ToLower(e.Status), e.CreatedAt)
	}
	iostream.PrintMarkdown(ctx, "# Deploys\n"+table.Render())
}
