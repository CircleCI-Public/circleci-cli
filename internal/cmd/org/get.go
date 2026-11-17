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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	orgops "github.com/CircleCI-Public/circleci-cli/internal/org"
)

func newGetCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show organization details and your role in it",
		Long: heredoc.Doc(`
			Show an organization's ID, name and VCS provider, and whether you are
			an admin of it.

			admin is null in JSON (and "unknown" otherwise) when the CircleCI
			server does not support the permission check.

			JSON fields: id, name, vcs_type, admin
		`),
		Example: heredoc.Doc(`
			# Show the org of the current git repository
			$ circleci org get

			# Show a specific org
			$ circleci org get --org gh/myorg

			# Check whether you are an admin, for scripting
			$ circleci org get --org gh/myorg --json --jq '.admin'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci org get")
			if err != nil {
				return err
			}
			return runOrgGet(ctx, client, orgID, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &orgSlug, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orgGetOutput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	VCSType string `json:"vcs_type"`
	Admin   *bool  `json:"admin"`
}

func runOrgGet(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, jsonOut bool) error {
	info, err := orgops.Details(ctx, client, orgID)
	if err != nil {
		if errors.Is(err, orgops.ErrNotMember) || httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("org.not_found", "Organization not found",
				fmt.Sprintf("You are not a member of an organization with ID %q.", orgID)).
				WithSuggestions("Run 'circleci org list' to see the organizations you belong to").
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, orgID.String(), "org.not_found", "No organization found for ID %q.")
	}

	out := orgGetOutput{
		ID:      info.ID.String(),
		Name:    info.Name,
		VCSType: info.VCS,
		Admin:   info.Admin,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	vcs := out.VCSType
	if vcs == "" {
		vcs = "-"
	}
	admin := "unknown"
	if out.Admin != nil {
		admin = yesNo(*out.Admin)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", out.Name)
	fmt.Fprintf(&b, "- ID: `%s`\n", out.ID)
	fmt.Fprintf(&b, "- VCS: %s\n", vcs)
	fmt.Fprintf(&b, "- Admin: %s\n", admin)
	iostream.PrintMarkdown(ctx, b.String())

	if info.Admin == nil {
		iostream.ErrPrintf(ctx, "%s Could not check whether you are an admin of this organization: "+
			"this CircleCI server does not support the permission check.\n",
			iostream.SymbolWarn(ctx))
	}
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
