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

package org

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List organizations you belong to",
		Long: heredoc.Doc(`
			List all CircleCI organizations the authenticated user is a member of.

			JSON fields: id, slug, name, vcs_type
		`),
		Example: heredoc.Doc(`
			# List all your organizations
			$ circleci org list

			# Output as JSON for scripting
			$ circleci org list --json

			# Extract just the slugs
			$ circleci org list --json --jq '.[].slug'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrgList(ctx, client, jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orgListOutput struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	VCSType string `json:"vcs_type"`
}

func runOrgList(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	collabs, err := client.ListCollaborations(ctx)
	if err != nil {
		return cmdutil.APIErr(err, "organizations", "org.list_failed", "Could not list organizations: %s")
	}

	out := make([]orgListOutput, len(collabs))
	for i, c := range collabs {
		out[i] = orgListOutput{
			ID:      c.ID,
			Slug:    c.Slug,
			Name:    c.Name,
			VCSType: c.VCSType,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.ErrPrintln(ctx, "No organizations found.")
		return nil
	}

	tbl := mdtable.New("Slug", "Name", "VCS")
	for _, o := range out {
		vcs := o.VCSType
		if vcs == "" {
			parts := strings.SplitN(o.Slug, "/", 2)
			if len(parts) == 2 {
				vcs = parts[0]
			}
		}
		tbl.Row(o.Slug, o.Name, vcs)
	}
	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Organizations\n%s", tbl.Render()))
	return nil
}
