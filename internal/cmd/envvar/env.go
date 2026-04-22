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

// Package envvar implements the "circleci envvar" top-level command, which is the
// primary user-facing alias for "circleci project envvar".
package envvar

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/project"
)

// NewEnvVarCmd returns the "circleci envvar" command, the primary entry point for
// managing project environment variables. It is an alias for "circleci project envvar"
// and exists to satisfy the 2-level nesting rule (circleci project envvar list
// is 3 levels deep).
func NewEnvVarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envvar <command>",
		Short: "Manage project environment variables",
		Long: heredoc.Doc(`
			List, set, and delete environment variables for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			Environment variable values are masked in list output (shown as "xxxx").
			The full value is never retrievable after it has been set.

			Also available as: circleci project envvar <command>
		`),
		Example: heredoc.Doc(`
			# List all environment variables for the current project
			$ circleci envvar list

			# Set an environment variable
			$ circleci envvar set MY_SECRET s3cr3t --project gh/myorg/myrepo

			# Delete an environment variable
			$ circleci envvar delete MY_SECRET
		`),
	}

	cmd.AddCommand(project.NewEnvListCmd())
	cmd.AddCommand(project.NewEnvSetCmd())
	cmd.AddCommand(project.NewEnvDeleteCmd())

	return cmd
}
