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

// Package cmdinit implements the "circleci init" onboarding command.
package cmdinit

import (
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

const signupURL = "https://circleci.com/signup"

// NewInitCmd returns the "circleci init" command.
//
// This is the shell for the CLI onboarding flow (WEBXP-987). Each of the four
// phases (Scan, Test, Generate, Sign up) is a stub here and will be filled in
// by follow-up tickets.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up CircleCI for the current project",
		Long: heredoc.Doc(`
			Walk through onboarding for the current project. circleci init will:

			  1. Scan your repo for tests
			  2. Run the tests in a Docker container using CircleCI's test framework
			  3. Generate a config file that can run your tests with CircleCI
			  4. Sign up for CircleCI to run your generated config

			Run from inside a git repository.
		`),
		Example: heredoc.Doc(`
			# Start onboarding from the current repo
			$ circleci init

			# Inspect what each step will do
			$ circleci init --help

			# Re-run onboarding to regenerate config
			$ cd path/to/repo && circleci init
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			if !gitremote.InsideWorkTree() {
				return clierrors.New("init.not_in_git_repo",
					"Not in a git repository",
					"You must be in a valid git repository directory to run this command.").
					WithSuggestions(
						"Use `cd <path>` to navigate to your project, then try again.",
					).
					WithExitCode(clierrors.ExitBadArguments)
			}

			ok := iostream.SymbolOK(ctx)
			cwd, _ := os.Getwd()
			repoName := filepath.Base(cwd)

			iostream.ErrPrintf(ctx, "%s Git repository detected.\n\n", ok)
			iostream.ErrPrintf(ctx, "circleci init will:\n")
			iostream.ErrPrintf(ctx, "  • Scan your repo for tests\n")
			iostream.ErrPrintf(ctx, "  • Run the tests in a Docker container using CircleCI's test framework\n")
			iostream.ErrPrintf(ctx, "  • Generate a config file that can run your tests with CircleCI\n\n")
			iostream.ErrPrintf(ctx, "This will run in your selected repo: %s\n\n", repoName)

			iostream.ErrPrintf(ctx, "[1/3] Scanning repository (stub)\n")
			iostream.ErrPrintf(ctx, "[2/3] Running tests in Docker (stub)\n")
			iostream.ErrPrintf(ctx, "[3/3] Generating config (stub)\n\n")

			iostream.ErrPrintf(ctx, "Next: sign up for CircleCI to run your generated config file.\n")
			iostream.ErrPrintf(ctx, "  %s\n", signupURL)
			return nil
		},
	}
}
