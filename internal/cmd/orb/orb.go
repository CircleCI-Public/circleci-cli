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

// Package orb implements the "circleci orb" command group.
package orb

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// NewOrbCmd returns the "circleci orb" command group.
func NewOrbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orb <command>",
		Short: "Manage orbs",
		Long: heredoc.Doc(`
			Managed orbs in the orb registry.

			Orbs are reusable packages of CircleCI configuration. They can be
			published to a namespace and shared with the community or kept private.

			Use 'circleci namespace' to manage namespaces before publishing orbs.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newListCategoriesCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newProcessCmd())
	cmd.AddCommand(newPublishCmd())
	cmd.AddCommand(newSourceCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newUnlistCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newAddToCategoryCmd())
	cmd.AddCommand(newRemoveFromCategoryCmd())
	cmd.AddCommand(newPackCmd())

	return cmd
}

func fmtBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orbAPIErr(err error, subject string) *clierrors.CLIError {
	if errors.Is(err, apiclient.ErrOrbNotFound) {
		return clierrors.New("orb.not_found", "Orb not found",
			fmt.Sprintf("No orb named %q exists.", subject)).
			WithSuggestions(
				"Check the orb name and namespace",
				"Use 'circleci orb list' to browse available orbs",
			).
			WithExitCode(clierrors.ExitNotFound)
	}
	if errors.Is(err, apiclient.ErrOrbVersionNotFound) {
		return clierrors.New("orb.version_not_found", "Orb version not found",
			fmt.Sprintf("No orb version found for %q.", subject)).
			WithSuggestions("Check the version string and try again").
			WithExitCode(clierrors.ExitNotFound)
	}
	if errors.Is(err, apiclient.ErrOrbCategoryNotFound) {
		return clierrors.New("orb.category_not_found", "Category not found",
			fmt.Sprintf("No category named %q exists.", subject)).
			WithSuggestions("Use 'circleci orb list-categories' to see available categories").
			WithExitCode(clierrors.ExitNotFound)
	}
	if errors.Is(err, apiclient.ErrNamespaceNotFound) {
		return clierrors.New("namespace.not_found", "Namespace not found",
			fmt.Sprintf("No namespace named %q exists.", subject)).
			WithSuggestions("Check the namespace name and try again").
			WithExitCode(clierrors.ExitNotFound)
	}
	return cmdutil.APIErr(err, subject, "orb.api_error", "Orb API request failed for %q.")
}
