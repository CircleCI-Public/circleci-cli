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

package cmdauth

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewAuthCmd returns the "circleci auth" command group.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth <command>",
		GroupID: "user",
		Short:   "Log in, sign up and check your CircleCI identity",
		Long: heredoc.Doc(`
			Manage authentication for the CLI.

			Use 'circleci auth signup' to create a new CircleCI account via the browser.
			Use 'circleci auth login' to authenticate via the browser-based OAuth flow.
			Use 'circleci auth me' to get current user info.
			Use 'circleci auth logout' to clear your stored credentials.
		`),
	}

	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newSignupCmd())
	cmd.AddCommand(newMeCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newDeviceIDCmd())

	cmdutil.DisableTelemetryForSubcommands(cmd)

	return cmd
}

// persistToken probes the new token via /me, prints "Logged in as <login>" on
// success, then saves the token to the configured backend (keyring by default,
// or the YAML config when --insecure-storage is set) and prints a
// "Saved token to <path>" status line on stderr. Shared by `auth login`
// (after OAuth token exchange) and `auth token` (after the TUI prompt).
func persistToken(ctx context.Context, host, token string, userID uuid.UUID, secureStorage bool, path string) error {
	storage, err := config.SetLogin(ctx, host, token, userID, secureStorage)
	if err != nil {
		return clierrors.New("auth.save_failed", "Failed to save token", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	tokenLoc := path
	if storage == config.StoredInKeyring {
		tokenLoc = "keyring"
	}
	iostream.ErrPrintf(ctx, "%s Saved token to %s\n", iostream.SymbolOK(ctx), tokenLoc)
	iostream.ErrPrintf(ctx, "%s Saved host to %s\n", iostream.SymbolOK(ctx), path)
	return nil
}
