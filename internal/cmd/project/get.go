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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func NewGetCmd(use string) *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: "Show project details",
		Long: heredoc.Doc(`
			Display detailed information about a CircleCI project, including
			its UUID, organization ID, and VCS configuration.

			The project is inferred from the current git repository's remote
			unless overridden with --project. The slug must be in the form
			vcs/org/repo (e.g. gh/myorg/myrepo).

			JSON fields: id, slug, name, organization_name, organization_slug,
			             organization_id, vcs_provider, vcs_default_branch, vcs_url
		`),
		Example: heredoc.Doc(`
			# Show details for the current git repository's project
			$ circleci project get

			# Show details for a specific project
			$ circleci project get --project gh/myorg/myrepo

			# Output as JSON for scripting
			$ circleci project get --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runProjectInfo(ctx, client, projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

type projectInfoOutput struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	OrganizationName string `json:"organization_name"`
	OrganizationSlug string `json:"organization_slug"`
	OrganizationID   string `json:"organization_id"`
	VCSProvider      string `json:"vcs_provider,omitempty"`
	VCSDefaultBranch string `json:"vcs_default_branch,omitempty"`
	VCSURL           string `json:"vcs_url,omitempty"`
}

func runProjectInfo(ctx context.Context, client *apiclient.Client, projectSlug string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci project get --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectInfo(ctx, projectSlug)
	if err != nil {
		return cmdutil.APIErr(err, projectSlug, "project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}

	out := projectInfoOutput{
		ID:               proj.ID,
		Slug:             proj.Slug,
		Name:             proj.Name,
		OrganizationName: proj.OrganizationName,
		OrganizationSlug: proj.OrganizationSlug,
		OrganizationID:   proj.OrganizationID,
	}
	if proj.VCSInfo != nil {
		out.VCSProvider = proj.VCSInfo.Provider
		out.VCSDefaultBranch = proj.VCSInfo.DefaultBranch
		out.VCSURL = proj.VCSInfo.VCSURL
	}

	if jsonOut {
		return cmdutil.WriteJSON(iostream.Out(ctx), out)
	}

	printProjectInfo(ctx, out)
	return nil
}

func printProjectInfo(ctx context.Context, p projectInfoOutput) {
	var md strings.Builder
	md.WriteString("# Project\n")
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", p.Name)
	_, _ = fmt.Fprintf(&md, "- Slug: %s\n", p.Slug)
	_, _ = fmt.Fprintf(&md, "- Project ID: `%s`\n", p.ID)
	_, _ = fmt.Fprintf(&md, "- Organization: %s\n", p.OrganizationName)
	_, _ = fmt.Fprintf(&md, "- Organization ID: `%s`\n", p.OrganizationID)
	if p.VCSProvider != "" {
		md.WriteString("\n## VCS\n")
		_, _ = fmt.Fprintf(&md, "- Provider: %s\n", p.VCSProvider)
		_, _ = fmt.Fprintf(&md, "- Default Branch: %s\n", p.VCSDefaultBranch)
		_, _ = fmt.Fprintf(&md, "- URL: %s\n", p.VCSURL)
	}
	md.WriteString("\n")
	iostream.PrintMarkdown(ctx, md.String())
}
