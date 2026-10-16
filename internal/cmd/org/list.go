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
	orgsvc "github.com/CircleCI-Public/circleci-cli/internal/org"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List organizations you belong to",
		Long: heredoc.Doc(`
			List all CircleCI organizations the authenticated user is a member of.

			The slug is what every other command's --org flag takes; an org ID
			works there too.

			JSON fields: id, name, slug, vcs_type
		`),
		Example: heredoc.Doc(`
			# List all your organizations
			$ circleci org list

			# Output as JSON for scripting
			$ circleci org list --json

			# Extract just the slugs, to pass to --org
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
	Name    string `json:"name"`
	Slug    string `json:"slug,omitempty"`
	VCSType string `json:"vcs_type"`
}

func runOrgList(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	orgs, err := client.ListOrgs(ctx)
	if err != nil {
		return cmdutil.APIErr(err, "organizations", "org.list_failed", "Could not list organizations: %s")
	}

	// The org list itself carries no slug, so it is decorated from the
	// collaborations endpoint. A failed lookup is reported and the listing
	// continues: the slug is supplementary, and dropping the whole listing over
	// it would be a worse answer than an incomplete one.
	slugs, err := orgsvc.SlugsByID(ctx, client)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not resolve organization slugs: %s\n",
			iostream.SymbolWarn(ctx), err)
	}

	out := make([]orgListOutput, len(orgs))
	for i, o := range orgs {
		out[i] = orgListOutput{
			ID:      o.ID.String(),
			Name:    o.Name,
			Slug:    slugs[o.ID.String()],
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

	// Slug leads: it is the value --org takes, so it is what a reader is here
	// for. The ID is kept because a few flags take it instead.
	tbl := mdtable.New("Slug", "Name", "Organization ID", "VCS")
	for _, o := range out {
		// A standalone CircleCI org has no VCS provider, and the slug is absent
		// when the supplementary lookup above failed, so either column would
		// otherwise be blank.
		tbl.Row(dash("`"+o.Slug+"`", o.Slug), o.Name, "`"+o.ID+"`", dash(o.VCSType, o.VCSType))
	}
	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Organizations\n%s", tbl.Render()))
	return nil
}

// dash renders val, or "-" when the underlying value is empty, so a blank table
// cell reads as "none" rather than as a rendering fault.
func dash(val, underlying string) string {
	if underlying == "" {
		return "-"
	}
	return val
}
