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

package run

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

func newOpenCmd() *cobra.Command {
	var branch string
	var currentBranch bool

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the current project's runs page in the browser",
		Long: heredoc.Doc(`
			Open the CircleCI runs page for the current project in your
			default web browser.

			The project is inferred from the current git repository's remote.
			Supports GitHub, Bitbucket, and GitLab remotes.

			Use --current-branch or --branch/-b to filter runs to a specific branch.
		`),
		Example: heredoc.Doc(`
			# Open runs for the current repo
			$ circleci run open

			# Open runs filtered to the current git branch
			$ circleci run open --current-branch

			# Open runs filtered to a specific branch
			$ circleci run open --branch my-feature

			# Open runs filtered to a specific branch (short flag)
			$ circleci run open -b main

			# Open when your remote is on CircleCI server
			$ circleci run open --host https://circleci.example.com
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if branch != "" && currentBranch {
				return clierrors.New("flag.conflict",
					"conflicting flags",
					"--branch and --current-branch cannot be used together").
					WithSuggestions("Use --current-branch to filter by your checked-out branch, or --branch <name> to specify one explicitly").
					WithExitCode(clierrors.ExitBadArguments)
			}

			ctx := cmd.Context()

			info, err := gitremote.Detect()
			if err != nil {
				return clierrors.New("git.detect_failed",
					"Could not detect project from git remote", err.Error()).
					WithSuggestions(
						"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					).
					WithExitCode(clierrors.ExitBadArguments)
			}

			appURL, err := cmdutil.AppURL(ctx, cmd)
			if err != nil {
				return err
			}

			if currentBranch {
				branch = info.Branch
			}

			var u string
			if branch != "" {
				u, err = cmdutil.PipelinesURLForBranch(appURL, info.Slug, branch)
			} else {
				u, err = cmdutil.PipelinesURL(appURL, info.Slug)
			}
			if err != nil {
				return err
			}

			return browser.OpenURL(u)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter runs to a specific branch")
	cmd.Flags().BoolVar(&currentBranch, "current-branch", false, "Filter runs to the current git branch")

	return cmd
}
