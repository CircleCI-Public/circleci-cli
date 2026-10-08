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

// Package runner implements the "circleci runner" command group.
package runner

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// NewRunnerCmd returns the "circleci runner" command group.
func NewRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runner <command>",
		GroupID: "management",
		Short:   "Manage self-hosted runners",
		Long: heredoc.Doc(`
			Manage self-hosted runner resources.

			Self-hosted runners let you run CircleCI jobs on your own infrastructure.
			Resource class names use the format namespace/name (e.g. my-org/my-runner).
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newOpenCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newConfigCmd(),
	)
	cmdutil.AddGroup(cmd, "Subcommands",
		newResourceClassCmd(),
		newTokenCmd(),
		newInstanceCmd(),
		newFleetCmd(),
	)

	return cmd
}

// forbiddenTitle is the error title runner-admin sends with the 403 for an org
// member who lacks the admin role (forbiddenError in its api/v3/auth.go). Other
// 403s, such as a resource class or token limit being reached, carry their own
// title, which is how the CLI tells them apart.
const forbiddenTitle = "Not permitted to perform this action for this organization."

// isAdminRequired reports whether err is runner-admin's 403 for an org member
// without the admin role.
func isAdminRequired(err error) bool {
	if !httpcl.HasStatusCode(err, http.StatusForbidden) {
		return false
	}
	apiErr, ok := apiclient.ParseError(err)
	return ok && apiErr.Title == forbiddenTitle
}

func apiErr(err error, subject string) *clierrors.CLIError {
	if isAdminRequired(err) {
		return adminRequiredErr(subject)
	}
	return cmdutil.APIErr(err, subject,
		"runner.not_found", "No runner resource found for %q, or it belongs to an organization your token cannot access.",
		"List available resource classes with: circleci runner resource-class list")
}

// adminRequiredErr reports a write refused because the caller can view the
// organization's runners but does not hold its admin role. runner-admin only
// answers this way once the resource is known to exist and be visible to the
// caller, so it is never a disguised "not found".
func adminRequiredErr(subject string) *clierrors.CLIError {
	return clierrors.New("runner.admin_required", "Organization admin required",
		fmt.Sprintf("You can view %q, but changing it requires the admin role in the organization that owns it. "+
			"Creating, updating and deleting runner resource classes and tokens is limited to organization admins.",
			subject)).
		WithSuggestions(
			"Ask an organization admin to make the change, or to grant you the admin role",
			"Confirm which account the CLI is using with: circleci auth me",
		).
		WithRef("https://circleci.com/docs/guides/execution-runner/runner-overview/").
		WithExitCode(clierrors.ExitAPIError)
}

// orgNotAccessibleErr reports runner-admin's 404 for an organization the caller
// is not a member of. The API does not say whether the organization exists, so
// neither can the CLI.
func orgNotAccessibleErr(orgID uuid.UUID) *clierrors.CLIError {
	return clierrors.New("runner.org_not_found", "Organization not found",
		fmt.Sprintf("Organization %s was not found, or your token is not a member of it.", orgID)).
		WithSuggestions(
			"Check the organization given by --org",
			"Confirm which account the CLI is using with: circleci auth me",
		).
		WithExitCode(clierrors.ExitNotFound)
}

func runnerNotEnabledErr() *clierrors.CLIError {
	return clierrors.New("runner.not_enabled", "Runner not available",
		"Self-hosted runners are not available for this token, or the token cannot access this organization's runners.").
		WithSuggestions(
			"Confirm your token can view or administer self-hosted runners",
			"Confirm the token belongs to the organization that owns the resource class",
		).
		WithRef("https://circleci.com/docs/guides/execution-runner/runner-overview/").
		WithExitCode(clierrors.ExitAPIError)
}
