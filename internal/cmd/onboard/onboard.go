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
	)

	cmd := &cobra.Command{
		Use:     "onboard [path]",
		GroupID: "user",
		Short:   "Guided onboarding: scan, test, generate config, sign up",
		Long: heredoc.Doc(`
			Guided onboarding: scan a local repository, run its detected tests,
			generate a starter .circleci/config.yml, and sign up for CircleCI.

			When run interactively without --scan or --signup, a prompt lets you
			choose between scanning the current repo or signing up directly.
		`),
		Example: heredoc.Doc(`
			# Interactive mode: choose scan or signup
			$ circleci onboard

			# Scan the current directory (skip the choice prompt)
			$ circleci onboard --scan

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
			})
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the signup URL instead of opening a browser")
	cmd.Flags().BoolVar(&scan, "scan", false, "Skip prompt: scan the repo and generate config")
	cmd.Flags().BoolVar(&signup, "signup", false, "Skip prompt: sign up for CircleCI")
	return cmd
}
