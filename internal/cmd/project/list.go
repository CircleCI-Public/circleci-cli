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

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newListCmd() *cobra.Command {
	var (
		following bool
		orgID     string
		name      string
		slug      string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List projects",
		Long: heredoc.Doc(`
			List the CircleCI projects visible to the authenticated user. With no
			filters, lists the projects you follow.

			JSON fields: id, name, org_id, org_name
		`),
		Example: heredoc.Doc(`
			# List the projects you follow
			$ circleci project list

			# List every project in an organization
			$ circleci project list --org-id 8f7c5a1e-0000-4000-8000-000000000001

			# Resolve a single project by slug
			$ circleci project list --slug gh/myorg/myrepo

			# Output as JSON for scripting
			$ circleci project list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			filter, err := projectListFilter(following, orgID, name, slug)
			if err != nil {
				return err
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			return runProjectList(ctx, client, filter, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&following, "following", false, "only projects you follow (default when no other filter is given)")
	cmd.Flags().StringVar(&orgID, "org-id", "", "only projects in this organization UUID")
	cmd.Flags().StringVar(&name, "name", "", "only projects matching this name")
	cmd.Flags().StringVar(&slug, "slug", "", "resolve one project by slug (e.g. gh/myorg/myrepo)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type projectOutput struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OrgID   uuid.UUID `json:"org_id"`
	OrgName string    `json:"org_name"`
}

// projectListFilter turns the flags into an API filter. The endpoint needs a
// scope — following, an org or a slug — so a bare invocation, or one narrowed
// only by name, falls back to the projects the user follows.
func projectListFilter(following bool, orgID, name, slug string) (apiclient.ProjectFilter, error) {
	if slug != "" {
		// The API rejects filter[slug] alongside any other filter; say so here
		// rather than relaying its 400.
		if following || orgID != "" || name != "" {
			return apiclient.ProjectFilter{}, clierrors.New("project.conflicting_filters", "Conflicting filters",
				"--slug resolves a single project, so it cannot be combined with --following, --org-id or --name.").
				WithSuggestions(
					"Drop the other filters: circleci project list --slug "+slug,
					"Or search instead: circleci project list --name <name>",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		return apiclient.ProjectFilter{Slug: slug}, nil
	}
	if orgID != "" {
		if _, err := uuid.Parse(orgID); err != nil {
			return apiclient.ProjectFilter{}, clierrors.New("project.invalid_org_id", "Invalid organization ID",
				fmt.Sprintf("%q is not a valid organization UUID.", orgID)).
				WithSuggestions(
					"Run: circleci org list --json  # to find the UUID",
					"Or filter by project slug: circleci project list --slug gh/myorg/myrepo",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
	}
	return apiclient.ProjectFilter{
		Following: following || orgID == "",
		OrgID:     orgID,
		Name:      name,
	}, nil
}

func runProjectList(ctx context.Context, client *apiclient.Client, filter apiclient.ProjectFilter, jsonOut bool) error {
	projects, err := client.ListProjects(ctx, filter)
	if err != nil {
		return cmdutil.APIErr(err, "projects", "project.list_failed", "Could not list projects: %s")
	}

	out := make([]projectOutput, len(projects))
	for i, p := range projects {
		out[i] = projectOutput{
			ID:      p.ID,
			Name:    p.Name,
			OrgID:   p.OrgID,
			OrgName: p.OrgName,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.ErrPrintln(ctx, "No projects found.")
		return nil
	}

	tbl := mdtable.New("ID", "Org Name", "Project Name")
	for _, p := range out {
		tbl.Row("`"+p.ID.String()+"`", p.OrgName, p.Name)
	}
	iostream.PrintMarkdown(ctx, "# Projects\n"+tbl.Render())
	return nil
}
