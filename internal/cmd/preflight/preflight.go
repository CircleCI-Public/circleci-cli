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

// Package preflight implements the "circleci preflight" command group.
package preflight

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/preflight"
)

// NewPreflightCmd returns the "circleci preflight" command group.
func NewPreflightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "preflight <command>",
		GroupID: "ci",
		Short:   "Run uncommitted changes through CI before you commit",
		Long: heredoc.Docf(`
			Run your working tree through CircleCI without committing it.

			A preflight snapshots the working tree — staged, unstaged and untracked
			changes, minus ignored files — into one commit on a throwaway branch
			under %[1]s%[2]s%[1]s, diffed against the default branch, pushes it,
			and lets the project's push trigger start a run. Your HEAD, index and
			working tree are never touched.
			Jobs can detect a preflight from the branch name, e.g.
			%[1]s[[ $CIRCLE_BRANCH == %[2]s* ]]%[1]s.
		`, "`", preflight.BranchPrefix),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(
		newRunCmd(),
		newCleanupCmd(),
	)

	return cmd
}

// gitErr converts a failed git invocation into a structured CLIError.
func gitErr(err error, title string) *clierrors.CLIError {
	return clierrors.New("preflight.git_failed", title, err.Error()).
		WithSuggestions(
			"Check that you can push to the remote: git push --dry-run origin HEAD",
			"Choose another remote with --remote",
		).
		WithExitCode(clierrors.ExitGeneralError)
}

// noRepoErr is returned when the working directory is not inside a git checkout.
func noRepoErr(err error) *clierrors.CLIError {
	return clierrors.New("preflight.no_repository", "Not a git repository", err.Error()).
		WithSuggestions("Run from inside the git checkout you want to preflight").
		WithExitCode(clierrors.ExitBadArguments)
}

func projectErr(err error, slug string) *clierrors.CLIError {
	return cmdutil.APIErr(err, slug, "project.not_found", "No project found for %q.",
		"Check the project slug, or pass it with --project gh/org/repo",
		"Run 'circleci project follow' if the project is not set up on CircleCI yet")
}
