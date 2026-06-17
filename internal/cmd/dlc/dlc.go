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

// Package dlc implements the "circleci dlc" command group.
package dlc

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// NewDLCCmd returns the "circleci dlc" parent command.
func NewDLCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dlc <command>",
		GroupID: "management",
		Short:   "Purge a project's Docker layer cache (DLC)",
		Long: heredoc.Doc(`
			Manage docker layer caching (DLC) for projects.

			Docker layer caching allows CircleCI to cache individual Docker image
			layers between pipeline runs. Use 'circleci dlc purge' to invalidate
			the cache for a project and force a fresh image build on the next run.

			These commands are also available under 'circleci project dlc'.
		`),
		Example: heredoc.Doc(`
			# Purge DLC for the current git repository's project
			$ circleci dlc purge

			# Purge DLC for a specific project
			$ circleci dlc purge --project gh/myorg/myrepo

			# Purge DLC and output result as JSON
			$ circleci dlc purge --project gh/myorg/myrepo --json
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(NewPurgeCmd())

	return cmd
}
