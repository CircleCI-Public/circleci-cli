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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List organizations you belong to",
		Long: heredoc.Doc(`
			List all CircleCI organizations the authenticated user is a member of.

			Only a VCS-backed org has a slug, so orgs are identified here by ID.
			Most commands' --org flag takes an org ID as well as a slug.

			JSON fields: id, name, vcs_type
		`),
		Example: heredoc.Doc(`
			# List all your organizations
			$ circleci org list

			# Output as JSON for scripting
			$ circleci org list --json

			# Extract just the IDs
			$ circleci org list --json --jq '.[].id'
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
	Name    string `json:"name"`
	VCSType string `json:"vcs_type"`
}

func runOrgList(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	orgs, err := client.ListOrgs(ctx)
	if err != nil {
		return cmdutil.APIErr(err, "organizations", "org.list_failed", "Could not list organizations: %s")
	}

	out := make([]orgListOutput, len(orgs))
	for i, o := range orgs {
		out[i] = orgListOutput{
			ID:      o.ID.String(),
			Name:    o.Name,
			VCSType: o.VCS,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.ErrPrintln(ctx, "No organizations found.")
		return nil
	}

	tbl := mdtable.New("Organization ID", "Name", "VCS")
	for _, o := range out {
		// A standalone CircleCI org has no VCS provider, so the column would
		// otherwise be blank.
		vcs := o.VCSType
		if vcs == "" {
			vcs = "-"
		}
		tbl.Row("`"+o.ID+"`", o.Name, vcs)
	}
	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Organizations\n%s", tbl.Render()))
	return nil
}
