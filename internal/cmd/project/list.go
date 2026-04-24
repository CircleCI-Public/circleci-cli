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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List followed projects",
		Long: heredoc.Doc(`
			List all CircleCI projects followed by the authenticated user.

			Projects are identified by a slug in the form vcs/org/repo
			(e.g. gh/myorg/myrepo). Use 'circleci project follow' to start
			following a new project.

			JSON fields: slug, name, vcs_type, username, reponame
		`),
		Example: heredoc.Doc(`
			# List all followed projects
			$ circleci project list

			# Output as JSON for scripting
			$ circleci project list --json

			# Filter by org with jq
			$ circleci project list --json | jq '.[] | select(.username == "myorg")'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return runProjectList(ctx, client, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

type projectOutput struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	VCSType  string `json:"vcs_type"`
	Username string `json:"username"`
	RepoName string `json:"reponame"`
}

func runProjectList(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return cmdutil.APIErr(err, "projects", "project.list_failed", "Could not list projects: %s")
	}

	out := make([]projectOutput, len(projects))
	for i, p := range projects {
		slug := p.Slug
		if slug == "" {
			// Build slug from parts when the v1.1 API omits the top-level slug field.
			var vcs string
			switch p.VCSType {
			case "github":
				vcs = "gh"
			case "bitbucket":
				vcs = "bb"
			default:
				vcs = strings.ToLower(p.VCSType)
			}
			slug = vcs + "/" + p.Username + "/" + p.RepoName
		}
		out[i] = projectOutput{
			Slug:     slug,
			Name:     p.Name,
			VCSType:  p.VCSType,
			Username: p.Username,
			RepoName: p.RepoName,
		}
	}

	if jsonOut {
		enc := json.NewEncoder(iostream.Out(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(out) == 0 {
		iostream.ErrPrintln(ctx, "No followed projects found.")
		return nil
	}

	var md strings.Builder
	md.WriteString("# Projects\n")
	for _, p := range out {
		_, _ = fmt.Fprintf(&md, "- %s\n", p.Slug)
	}
	iostream.PrintMarkdown(ctx, md.String())
	return nil
}
