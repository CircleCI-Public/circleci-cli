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

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
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
		Short:   "Guided onboarding: scan, test, generate config, sign up",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is optional and is the directory to scan. Defaults to
				the current directory.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Guided onboarding: scan a local repository, run its detected tests,
			generate a starter .circleci/config.yml, sign up for CircleCI, and —
			for CircleCI-native organizations — create the project along with a
			pipeline definition and an all-pushes trigger so your next push starts
			a build.

			When run interactively without --scan or --signup, a prompt lets you
			choose between scanning the current repo or signing up directly.

			Setting up the pipeline needs your repository's numeric ID. For a GitHub
			repository it is resolved through the CircleCI GitHub App. Pass --repo-id
			to skip that lookup, or when the repository is not on GitHub. Without an
			ID the project is still created and the steps to finish setup by hand are
			printed.
		`),
		Example: heredoc.Doc(`
			# Interactive mode: choose scan or signup
			$ circleci onboard

			# Scan the current directory (skip the choice prompt)
			$ circleci onboard --scan

			# Scan and wire up the first pipeline without prompts
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
	cmd.Flags().BoolVar(&scan, "scan", false, "Skip prompt: scan the repo and generate config")
	cmd.Flags().BoolVar(&signup, "signup", false, "Skip prompt: sign up for CircleCI")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository external ID (numeric) for the pipeline")
	return cmd
}
