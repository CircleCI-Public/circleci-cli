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
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create <project-name> --org <vcs/org-slug>",
		Short: "Create a new project",
		Long: heredoc.Doc(`
			Create a new CircleCI project in the given organization.

			The --org flag takes a slug in the form vcs/org (e.g. gh/myorg or
			circleci/9YytKzouJxzu4TjCRFqAoD). The org slug is found in the
			CircleCI web app under Organization Settings > Organization slug.

			JSON fields: id, slug, name, organization_name, organization_slug,
			             organization_id, vcs_provider, vcs_default_branch, vcs_url
		`),
		Example: heredoc.Doc(`
			# Create a GitHub-hosted project
			$ circleci project create my-new-repo --org gh/myorg

			# Create a CircleCI-native project (standalone pipelines)
			$ circleci project create new-service --org circleci/9YytKzouJxzu4TjCRFqAoD

			# Create a project and output as JSON for scripting
			$ circleci project create my-new-repo --org gh/myorg --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgSlug == "" {
				return clierrors.New("args.missing_org", "Missing required flag",
					"--org is required to specify the target organization.").
					WithSuggestions(
						"Pass --org <vcs/org-slug> (e.g. --org gh/myorg)",
						"Find your org slug in CircleCI under Organization Settings > Organization slug",
					).
					WithExitCode(clierrors.ExitBadArguments)
			}
			vcs, org, err := parseOrgSlug(orgSlug)
			if err != nil {
				return clierrors.New("args.invalid_org", "Invalid --org value",
					fmt.Sprintf("%q is not a valid org slug.", orgSlug)).
					WithSuggestions("Use the form vcs/org (e.g. gh/myorg or circleci/9YytKzouJxzu4TjCRFqAoD)").
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runProjectCreate(ctx, client, vcs, org, args[0], jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "organization slug (e.g. gh/myorg)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

// parseOrgSlug splits "gh/myorg" → ("gh", "myorg").
func parseOrgSlug(slug string) (vcs, org string, err error) {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid org slug")
	}
	return parts[0], parts[1], nil
}

type createProjectOutput struct {
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

func runProjectCreate(ctx context.Context, client *apiclient.Client, vcs, org, name string, jsonOut bool) error {
	proj, err := client.CreateProject(ctx, vcs, org, name)
	if err != nil {
		return cmdutil.APIErr(err, fmt.Sprintf("%s/%s/%s", vcs, org, name),
			"project.create_failed", "Could not create project %q.")
	}

	out := createProjectOutput{
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

	printCreatedProject(ctx, out)
	return nil
}

func printCreatedProject(ctx context.Context, p createProjectOutput) {
	var md strings.Builder
	md.WriteString("# Project Created\n")
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", p.Name)
	_, _ = fmt.Fprintf(&md, "- Slug: %s\n", p.Slug)
	_, _ = fmt.Fprintf(&md, "- Project ID: `%s`\n", p.ID)
	_, _ = fmt.Fprintf(&md, "- Organization: %s\n", p.OrganizationName)
	_, _ = fmt.Fprintf(&md, "- Organization ID: `%s`\n", p.OrganizationID)
	if p.VCSProvider != "" {
		md.WriteString("\n## VCS\n")
		_, _ = fmt.Fprintf(&md, "- Provider: %s\n", p.VCSProvider)
		if p.VCSDefaultBranch != "" {
			_, _ = fmt.Fprintf(&md, "- Default Branch: %s\n", p.VCSDefaultBranch)
		}
		if p.VCSURL != "" {
			_, _ = fmt.Fprintf(&md, "- URL: %s\n", p.VCSURL)
		}
	}
	md.WriteString("\n")
	iostream.PrintMarkdown(ctx, md.String())
}
