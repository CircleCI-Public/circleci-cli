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

// Package setup provides the hidden "circleci setup" command. It exists only
// for backwards compatibility with the CircleCI CLI orb, whose setup.sh runs
// `circleci setup --no-prompt --host <host> --token <token>`. The legacy CLI
// exposed an interactive `setup` command; this rewrite replaces it with `auth
// login` and `setting set`, so the only supported path here is the
// non-interactive one the orb relies on.
package setup

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewSetupCmd returns the hidden "circleci setup" command.
func NewSetupCmd() *cobra.Command {
	var (
		host  string
		token string
	)

	cmd := &cobra.Command{
		Use:    "setup",
		Short:  "Configure the CLI with a host and token (legacy compatibility)",
		Hidden: true,
		Long: heredoc.Doc(`
			Persist a CircleCI host and API token to the CLI config file.

			This command exists only for backwards compatibility with the
			CircleCI CLI orb. Only the non-interactive path is supported: you
			must pass --no-prompt, --host, and --token. The interactive setup
			flow from the legacy CLI is not reimplemented.

			For new usage, prefer:
			  circleci auth login                 interactive, browser-based
			  circleci setting set host <host>    store the host
			  circleci setting set token <token>  store the token
		`),
		Example: heredoc.Doc(`
			# Configure the CLI non-interactively (the path the orb uses)
			$ circleci setup --no-prompt --host https://circleci.com --token mytoken123

			# Point at a self-hosted CircleCI server
			$ circleci setup --no-prompt --host https://circleci.example.com --token mytoken123
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			secureStorage := cmdutil.IsSecureStorage(cmd)
			configPath := cmdutil.ConfigPath(cmd)
			return runSetup(ctx, host, token, secureStorage, configPath)
		},
	}

	// All three flags are required: the only supported path is non-interactive.
	// Marking --no-prompt required forces callers to opt into that explicitly,
	// matching the legacy CLI's flag and the orb's invocation.
	cmd.Flags().Bool("no-prompt", false, "Run without interactive prompts (required)")
	cmd.Flags().StringVar(&host, "host", "", "CircleCI host, e.g. https://circleci.com (required)")
	cmd.Flags().StringVar(&token, "token", "", "CircleCI API token (required)")
	for _, name := range []string{"no-prompt", "host", "token"} {
		cobra.CheckErr(cmd.MarkFlagRequired(name))
	}

	return cmd
}

func runSetup(ctx context.Context, host, token string, secureStorage bool, path string) error {
	if err := config.SetHost(ctx, host); err != nil {
		return clierrors.New("setup.save_failed", "Failed to save host", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	// Let the config layer route the token to the keyring or the config file
	// based on secureStorage, exactly as 'setting set token' does.
	storage, err := config.SetToken(ctx, token, secureStorage)
	if err != nil {
		return clierrors.New("setup.save_failed", "Failed to save token", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.ErrPrintf(ctx, "%s Saved host to %s\n", iostream.SymbolOK(ctx), path)
	if storage == config.StoredInKeyring {
		iostream.ErrPrintf(ctx, "%s Saved token to keyring\n", iostream.SymbolOK(ctx))
	} else {
		iostream.ErrPrintf(ctx, "%s Saved token to %s\n", iostream.SymbolOK(ctx), path)
	}
	return nil
}
