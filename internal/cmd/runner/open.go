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
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/browser"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOpenCmd() *cobra.Command {
	var orgSlug string

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the runners inventory page in the browser",
		Long: heredoc.Doc(`
			Open the CircleCI runners inventory page for an organization in your
			default web browser.

			The organization is inferred from the current git repository's remote
			unless overridden with --org. Supports GitHub, Bitbucket, and GitLab
			remotes.
		`),
		Example: heredoc.Doc(`
			# Open runners for the org inferred from git remote
			$ circleci runner open

			# Open runners for a specific organization
			$ circleci runner open --org gh/myorg

			# Open when your remote is on CircleCI server
			$ circleci runner open --host https://circleci.example.com
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			slug, err := cmdutil.ResolveOrgSlug(orgSlug, "circleci runner open")
			if err != nil {
				return err
			}

			appURL, err := cmdutil.AppURL(ctx)
			if err != nil {
				return err
			}

			u, err := cmdutil.RunnersURL(appURL, slug)
			if err != nil {
				return err
			}

			return browser.OpenURLOrPrint(iostream.Err(ctx), u)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); defaults to git remote")

	return cmd
}
