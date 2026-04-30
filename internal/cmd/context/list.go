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

package context

import (
	"context"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/mdtable"
)

func newListCmd() *cobra.Command {
	var (
		orgSlug string
		name    string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List contexts for an organization",
		Long: heredoc.Doc(`
			List all contexts for a CircleCI organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org. Provide the org slug in the form
			gh/myorg or bitbucket/myorg.

			JSON fields: id, name, created_at
		`),
		Example: heredoc.Doc(`
			# List contexts for the org inferred from git remote
			$ circleci context list

			# List contexts containing the given name
			$ circleci context list --name substring

			# List contexts for a specific organization
			$ circleci context list --org gh/myorg

			# Output as JSON for scripting
			$ circleci context list --json

			# Get just context names
			$ circleci context list --json --jq '.[].name'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runList(ctx, client, orgSlug, name, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); defaults to git remote")
	cmd.Flags().StringVar(&name, "name", "", "Find contexts by name (partial match)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type contextListEntry struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func runList(ctx context.Context, client *apiclient.Client, orgSlug string, name string, jsonOut bool) error {
	if orgSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the organization: circleci context list --org gh/myorg")
		}
		orgSlug = orgFromSlug(info.Slug)
	}

	contexts, err := client.ListContexts(ctx, orgSlug, name)
	if err != nil {
		return apiErr(err, orgSlug)
	}

	entries := make([]contextListEntry, len(contexts))
	for i, c := range contexts {
		entries[i] = contextListEntry{
			ID:        c.ID,
			Name:      c.Name,
			CreatedAt: c.CreatedAt,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(contexts) == 0 {
		iostream.ErrPrintln(ctx, "No contexts found.")
		return nil
	}

	tbl := mdtable.New("Name", "ID", "Created")
	for _, e := range entries {
		tbl.Row(e.Name, "`"+e.ID.String()+"`", e.CreatedAt.Format(time.RFC3339))
	}
	iostream.PrintMarkdown(ctx, "# Contexts\n"+tbl.Render())
	return nil
}

// orgFromSlug extracts the org portion of a project slug.
// "gh/myorg/myrepo" → "gh/myorg"
func orgFromSlug(projectSlug string) string {
	parts := strings.SplitN(projectSlug, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return projectSlug
}
