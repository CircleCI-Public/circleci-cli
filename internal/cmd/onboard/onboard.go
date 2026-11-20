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
	"fmt"
	"slices"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/forgejoconn"
	"github.com/CircleCI-Public/circleci-cli/internal/onboarder"
	"github.com/CircleCI-Public/circleci-cli/internal/provider"
)

// NewOnboardCmd returns the "circleci onboard" command.
func NewOnboardCmd() *cobra.Command {
	var (
		noBrowser bool
		scan      bool
		signup    bool
		org       string
		repoID    string
		prov      string
		forgejo   forgejoconn.Options
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
			Writes a starter config if none exists, creates and follows the project, adds a trigger.
		`),
		Example: heredoc.Doc(`
			# Set up the current directory (skip the choice prompt)
			$ circleci onboard --scan

			# Set up in a named org (required when you belong to several)
			$ circleci onboard --scan --org gh/acme

			# Connect a self-hosted Forgejo repo without prompts
			$ circleci onboard --scan --provider forgejo --connection-audience u:1:ab12 --checkout-audience u:1:cd34
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if scan && signup {
				return clierrors.New("onboard.invalid_args", "Invalid arguments",
					"--scan and --signup are mutually exclusive").
					WithExitCode(clierrors.ExitBadArguments)
			}

			if err := validateProvider(prov); err != nil {
				return err
			}
			if _, err := forgejoconn.ParseHeaders(forgejo.Headers); err != nil {
				return err
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
				Org:           org,
				RepoID:        repoID,
				Provider:      prov,
				Forgejo:       forgejo,
			})
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the signup URL instead of opening a browser")
	cmd.Flags().BoolVar(&scan, "scan", false, "Skip prompt: set up this repo on CircleCI")
	cmd.Flags().BoolVar(&signup, "signup", false, "Skip prompt: sign up for CircleCI")
	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "to set the project up in"})
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID, if it cannot be resolved from the git remote")
	cmd.Flags().StringVar(&prov, "provider", "", "Provider of a self-hosted remote: "+provider.List(provider.ConfigSource, provider.Provider.ConnectsInTerminal))
	cmd.Flags().StringVar(&forgejo.URL, "provider-url", "", "Forgejo instance URL (default: https://<remote host>)")
	cmd.Flags().StringVar(&forgejo.Owner, "owner", "", "Forgejo org or user that owns the repo (default: remote owner)")
	cmd.Flags().StringVar(&forgejo.ConnectionAudience, "connection-audience", "", "Audience of the bot's CircleCI connection integration")
	cmd.Flags().StringVar(&forgejo.CheckoutAudience, "checkout-audience", "", "Audience of the bot's CircleCI checkout integration")
	cmd.Flags().StringArrayVarP(&forgejo.Headers, "header", "H", nil, fmt.Sprintf("Request header for Forgejo, \"Name: Value\" (repeatable, max %d)", forgejoconn.MaxRequestHeaders))
	return cmd
}

// validateProvider accepts only the integrations onboard connects in the
// terminal: the others are recognised from the remote's host.
func validateProvider(v string) error {
	valid := provider.Names(provider.ConfigSource, provider.Provider.ConnectsInTerminal)
	if v == "" || slices.Contains(valid, v) {
		return nil
	}
	return clierrors.New("args.invalid_provider", "Invalid --provider value",
		fmt.Sprintf("%q is not a provider onboard can connect.", v)).
		WithSuggestions(
			"Valid values: "+provider.List(provider.ConfigSource, provider.Provider.ConnectsInTerminal),
			"Omit --provider for a remote on a host CircleCI recognises, such as github.com",
		).
		WithExitCode(clierrors.ExitBadArguments)
}
