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

package runner

import (
	"context"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

const runnerTermsURL = "https://circleci.com/legal/runner-terms/"

func newTermsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terms <command>",
		Short: "Accept the self-hosted runner terms",
		Long: heredoc.Doc(`
			Manage your organization's acceptance of the self-hosted runner terms.

			An organization admin must accept the terms once before the organization
			can use self-hosted runners.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "General commands",
		newTermsAcceptCmd(),
	)

	return cmd
}

func newTermsAcceptCmd() *cobra.Command {
	var (
		orgRef  string
		force   bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "accept",
		Short: "Accept the self-hosted runner terms for an organization",
		// Accepting a legal agreement on an org's behalf is irreversible from
		// the CLI, so MCP hosts should confirm with the user before calling it.
		Annotations: map[string]string{"destructiveHint": "true"},
		Long: heredoc.Docf(`
			Accept CircleCI's self-hosted runner terms on behalf of an organization.
			You must be an admin of the organization. Asks for confirmation first;
			does nothing if the terms are already accepted.

			Terms: %[1]s

			JSON fields: org_id, accepted, already_accepted
		`, runnerTermsURL),
		Example: heredoc.Doc(`
			# Accept the terms for the org of the current git repository
			$ circleci runner terms accept

			# Accept the terms for a specific org
			$ circleci runner terms accept --org gh/myorg

			# Accept without a confirmation prompt, for scripting
			$ circleci runner terms accept --org gh/myorg --force --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgRef, "circleci runner terms accept")
			if err != nil {
				return err
			}
			label := orgRef
			if label == "" {
				// The org came from the current project, so name it by the
				// project's org slug (gh/myorg) rather than its UUID.
				label = orgID.String()
				if slug, err := cmdutil.ResolveOrgSlug("", "circleci runner terms accept"); err == nil && slug != "" {
					label = slug
				}
			}
			return runTermsAccept(ctx, client, orgID, label, force, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &orgRef, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type termsAcceptOutput struct {
	OrgID           string `json:"org_id"`
	Accepted        bool   `json:"accepted"`
	AlreadyAccepted bool   `json:"already_accepted"`
}

func runTermsAccept(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, label string, force, jsonOut bool) error {
	settings, err := client.GetOrgSettings(ctx, orgID)
	if err != nil {
		return termsErr(err, orgID, label)
	}

	if settings.RunnerTOSAccepted {
		if jsonOut {
			return iostream.PrintJSON(ctx, termsAcceptOutput{OrgID: orgID.String(), Accepted: true, AlreadyAccepted: true})
		}
		iostream.Printf(ctx, "%s The self-hosted runner terms are already accepted for %s\n", iostream.SymbolOK(ctx), label)
		return nil
	}

	if !force && !jsonOut {
		iostream.ErrPrintf(ctx, "CircleCI's self-hosted runner terms: %s\n", runnerTermsURL)
	}
	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Accept the runner terms for %s?", label),
		clierrors.New("runner.terms_aborted", "Terms not accepted",
			"The self-hosted runner terms were not accepted.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("runner.terms_requires_force", "Accepting the terms requires --force",
			fmt.Sprintf("Accepting the self-hosted runner terms (%s) on behalf of %s needs confirmation.", runnerTermsURL, label)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	accepted := true
	settings, err = client.UpdateOrgSettings(ctx, orgID, apiclient.OrgSettingsUpdate{RunnerTOSAccepted: &accepted})
	if err != nil {
		return termsErr(err, orgID, label)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, termsAcceptOutput{OrgID: orgID.String(), Accepted: settings.RunnerTOSAccepted})
	}
	iostream.Printf(ctx, "%s Accepted the self-hosted runner terms for %s\n", iostream.SymbolOK(ctx), label)
	iostream.ErrPrintf(ctx, "\nNext, create a resource class: circleci runner resource-class create <namespace>/<name>\n")
	return nil
}

func termsErr(err error, orgID uuid.UUID, label string) error {
	switch {
	case httpcl.HasStatusCode(err, http.StatusNotFound):
		return orgNotAccessibleErr(orgID)
	case httpcl.HasStatusCode(err, http.StatusForbidden):
		return clierrors.New("runner.admin_required", "Organization admin required",
			fmt.Sprintf("Accepting the self-hosted runner terms for %s requires the admin role in that organization.", label)).
			WithSuggestions(
				"Check your role with: circleci org get --org "+label,
				"Ask an organization admin to accept the terms",
			).
			WithRef(runnerTermsURL).
			WithExitCode(clierrors.ExitAPIError)
	}
	return cmdutil.APIErr(err, label, "runner.org_not_found", "No organization found for %q.")
}
