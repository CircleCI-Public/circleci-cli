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

// Package cmdonboard implements the "circleci onboard" command.
package cmdonboard

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/onboarder"
)

// NewOnboardCmd returns the "circleci onboard" command.
func NewOnboardCmd() *cobra.Command {
	var (
		noBrowser bool
		scan      bool
		signup    bool
		repoID    string
	)

	cmd := &cobra.Command{
		Use:     "onboard [path]",
		GroupID: "user",
		Short:   "Guided onboarding: generate config, sign up, connect project",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the directory to onboard. Defaults to the current directory.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Prompts for repo setup or signup unless --scan or --signup is given. Writes a
			starter config if the repo has none, creates the project, adds a push trigger.
		`),
		Example: heredoc.Doc(`
			# Interactive mode: choose repo setup or signup
			$ circleci onboard

			# Set up the current directory (skip the choice prompt)
			$ circleci onboard --scan

			# Set up and wire up the first pipeline without prompts
			$ circleci onboard --scan --repo-id 123456789

			# Sign up for CircleCI (no repo needed)
			$ circleci onboard --signup

			# Onboard a specific project path
			$ circleci onboard --scan ./my-app

			# Print the signup URL instead of opening a browser
			$ circleci onboard --signup --no-browser
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if scan && signup {
				return clierrors.New("onboard.invalid_args", "Invalid arguments",
					"--scan and --signup are mutually exclusive").
					WithExitCode(clierrors.ExitBadArguments)
			}

			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			configPath := cmdutil.ConfigPath(cmd)
			return onboarder.Run(ctx, dir, onboarder.Options{
				NoBrowser:     noBrowser,
				SecureStorage: cmdutil.IsSecureStorage(cmd),
				ConfigPath:    configPath,
				Scan:          scan,
				Signup:        signup,
				RepoID:        repoID,
			})
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the signup URL instead of opening a browser")
	cmd.Flags().BoolVar(&scan, "scan", false, "Skip prompt: set up this repo on CircleCI")
	cmd.Flags().BoolVar(&signup, "signup", false, "Skip prompt: sign up for CircleCI")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID, if it cannot be resolved from the git remote")
	return cmd
}
