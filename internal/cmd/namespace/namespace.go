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

// Package namespace implements the "circleci namespace" command group.
package namespace

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// NewNamespaceCmd returns the "circleci namespace" command group.
func NewNamespaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "namespace <command>",
		GroupID: "management",
		Short:   "Manage the org namespace orbs publish under",
		Long: heredoc.Doc(`
			Work with CircleCI orb namespaces.

			Namespaces are unique identifiers used to publish orbs.
			Each organization may claim one namespace. Orbs are published
			and referenced as namespace/orb. All published orbs are world-readable.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "General commands",
		newCreateCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newGetCmd(),
		newDeleteCmd(),
		newRenameCmd(),
	)

	return cmd
}

type namespaceOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func apiErr(err error, name string) *clierrors.CLIError {
	if errors.Is(err, apiclient.ErrNamespaceNotFound) {
		return clierrors.New("namespace.not_found", "Namespace not found",
			fmt.Sprintf("No namespace named %q exists.", name)).
			WithSuggestions("Check the namespace name and try again").
			WithExitCode(clierrors.ExitNotFound)
	}
	return cmdutil.APIErr(err, name, "namespace.api_error", "Namespace API request failed for %q.")
}
