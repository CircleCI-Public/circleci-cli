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
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/onboarder"
)

// NewOnboardCmd returns the "circleci onboard" command.
func NewOnboardCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "onboard [path]",
		Short: "Guided onboarding: scan, test, generate config, sign up",
		Long: heredoc.Doc(`
			Scan a local repository, run its detected tests, generate a starter
			.circleci/config.yml when one does not already exist, and sign up for
			CircleCI through the browser-based auth flow.
		`),
		Example: heredoc.Doc(`
			# Onboard the current directory
			$ circleci onboard

			# Onboard a specific project path
			$ circleci onboard ./my-app

			# Print the signup URL instead of opening a browser
			$ circleci onboard --no-browser
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			configPath, _ := cmd.Flags().GetString("config")
			return onboarder.Run(ctx, dir, onboarder.Options{
				ConfigPath:    configPath,
				NoBrowser:     noBrowser,
				SecureStorage: cmdutil.IsSecureStorage(cmd),
			})
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the signup URL instead of opening a browser")
	return cmd
}
