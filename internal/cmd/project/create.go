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
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/org"
)

func newCreateCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create [project-name] --org <vcs/org-slug>",
		Short: "Create a new project",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				[project-name] is the name for the new project and is optional.
				When omitted, the current git repository's name is used: in a
				terminal you are prompted with it as the default, and in
				non-interactive mode it is used automatically.
			`),
		},
		Long: heredoc.Doc(`
			Create a new CircleCI project in the given organization.

			The --org flag takes a slug in the form vcs/org (e.g. gh/myorg or
			circleci/9YytKzouJxzu4TjCRFqAoD). The org slug is found in the
			CircleCI web app under Organization Settings > Organization slug.

			The project name argument is optional in a terminal: if omitted, you
			will be prompted for it and the current repository name is offered
			as the default. In non-interactive mode the current repository name
			is used automatically when no argument is given.

			The --org flag is also optional in a terminal: if omitted, you will
			be prompted to pick from the organizations your account belongs to.
			In non-interactive mode it is required.

			JSON fields: id, slug, name, organization_name, organization_slug,
			             organization_id, vcs_provider, vcs_default_branch, vcs_url
		`),
		Example: heredoc.Doc(`
			# Create a project (prompted for name if run interactively)
			$ circleci project create --org gh/myorg

			# Create a GitHub-hosted project
			$ circleci project create my-new-repo --org gh/myorg

			# Create a CircleCI-native project (standalone pipelines)
			$ circleci project create new-service --org circleci/9YytKzouJxzu4TjCRFqAoD

			# Create a project and output as JSON for scripting
			$ circleci project create my-new-repo --org gh/myorg --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			if name == "" {
				defaultName := gitremote.DetectRepoName()
				if iostream.IsInteractive(ctx) {
					val, err := iostream.PromptText(ctx, "Project name", defaultName)
					if err != nil {
						return clierrors.New("project.create_cancelled", "Cancelled",
							"Prompt was cancelled.").
							WithExitCode(clierrors.ExitCancelled)
					}
					if val == "" {
						val = defaultName
					}
					if val == "" {
						return clierrors.New("project.create_cancelled", "Cancelled",
							"No project name entered.").
							WithExitCode(clierrors.ExitCancelled)
					}
					name = val
				} else {
					if defaultName == "" {
						return clierrors.New("args.missing_name", "Missing project name",
							"Provide a project name as an argument.").
							WithSuggestions(
								"Pass a project name: circleci project create <name> --org <vcs/org-slug>",
							).
							WithExitCode(clierrors.ExitBadArguments)
					}
					name = defaultName
				}
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			if orgSlug == "" {
				if !iostream.IsInteractive(ctx) {
					return clierrors.New("args.missing_org", "Missing required flag",
						"--org is required to specify the target organization.").
						WithSuggestions(
							"Pass --org <vcs/org-slug> (e.g. --org gh/myorg)",
							"Find your org slug in CircleCI under Organization Settings > Organization slug",
						).
						WithExitCode(clierrors.ExitBadArguments)
				}
				selected, err := org.Select(ctx, client)
				if err != nil {
					return err
				}
				orgSlug = selected
			}

			vcs, orgName, err := org.ParseSlug(orgSlug)
			if err != nil {
				return clierrors.New("args.invalid_org", "Invalid --org value",
					fmt.Sprintf("%q is not a valid org slug.", orgSlug)).
					WithSuggestions("Use the form vcs/org (e.g. gh/myorg or circleci/9YytKzouJxzu4TjCRFqAoD)").
					WithExitCode(clierrors.ExitBadArguments)
			}
			appURL, err := cmdutil.AppURL(ctx)
			if err != nil {
				return err
			}
			return runProjectCreate(ctx, client, vcs, orgName, name, appURL, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "organization slug (e.g. gh/myorg)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
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

func runProjectCreate(ctx context.Context, client *apiclient.Client, vcs, org, name, appURL string, jsonOut bool) error {
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

	pipelinesURL, _ := cmdutil.PipelinesURL(appURL, out.Slug)
	printCreatedProject(ctx, out, pipelinesURL)
	return nil
}

func printCreatedProject(ctx context.Context, p createProjectOutput, pipelinesURL string) {
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
	if pipelinesURL != "" {
		_, _ = fmt.Fprintf(&md, "\nPipelines: %s\n", pipelinesURL)
	}
	md.WriteString("\n")
	iostream.PrintMarkdown(ctx, md.String())
}
