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

// Package context implements the "circleci context" command group.
package context

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// NewContextCmd returns the "circleci context" command group.
func NewContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context <command>",
		GroupID: "management",
		Short:   "Manage secret env vars shared across pipelines",
		Long: heredoc.Doc(`
			Work with CircleCI contexts.

			Contexts are named collections of secret environment variables that
			can be shared across runs within an organization. Jobs can
			reference a context to inject its variables into the build environment.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newListCmd(),
		newCreateCmd(),
		newOpenCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newGetCmd(),
		newDeleteCmd(),
	)
	cmdutil.AddGroup(cmd, "Subcommands",
		newSecretCmd(),
		newRestrictionCmd(),
	)

	return cmd
}

// apiErr maps an error from a context call scoped to an organization —
// list and create. A 403 stays a 403: it means the token cannot see the org,
// which is worth saying plainly rather than leaving to the generic handler.
func apiErr(err error, subject string) *clierrors.CLIError {
	if httpcl.HasStatusCode(err, http.StatusForbidden) {
		return clierrors.New("context.org_forbidden", "Access denied",
			fmt.Sprintf("Your token does not have access to organization %q.", subject)).
			WithSuggestions(
				"Check the organization with: circleci org list",
				"Confirm you are logged in as the right user: circleci auth me",
			).
			WithExitCode(clierrors.ExitAPIError)
	}
	return cmdutil.APIErr(err, subject, //nolint:wrapcheck // APIErr returns a *CLIError, not a wrapped error.
		"context.not_found", "No context found for %q.",
		"Check the context name or ID and try again",
		"Run: circleci context list --org <org-slug>")
}

// contextIDErr maps an error from a context call addressed by context id —
// get and delete.
func contextIDErr(err error, subject string) *clierrors.CLIError {
	if e := byIDForbiddenErr(err); e != nil {
		return e
	}
	return cmdutil.APIErr(err, subject, //nolint:wrapcheck // APIErr returns a *CLIError, not a wrapped error.
		"context.not_found", "No context found for %q.",
		"Check the context name or ID and try again",
		"Run: circleci context list --org <org-slug>")
}

// byIDForbiddenErr maps a 403 from a request addressed by a context, variable
// or restriction id, returning nil for any other error.
//
// On these endpoints a 403 has exactly one meaning: the caller can see the
// context but lacks the permission for this action — the read-only-member case.
// A context that does not exist, and one in an organization the caller cannot
// see, both answer 404 so the two cannot be told apart. That makes the message
// below safe to state plainly; it was not, while a missing context could also
// produce a 403.
//
// The subject is deliberately not named: callers pass a context id, a variable
// name or a restriction id here, and the permission is on the context in every
// case.
func byIDForbiddenErr(err error) *clierrors.CLIError {
	if !httpcl.HasStatusCode(err, http.StatusForbidden) {
		return nil
	}
	return clierrors.New("context.forbidden", "Permission denied",
		"Your token can see this context but does not have permission for this action.").
		WithSuggestions(
			"Ask an organization administrator to grant you the permission",
			"Confirm you are logged in as the right user: circleci auth me",
		).
		WithExitCode(clierrors.ExitAPIError)
}
