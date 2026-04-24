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

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Set personal access token for authenticating to CircleCI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			if !iostream.IsInteractive(ctx) {
				return clierrors.New("auth.token.aborted", "Login aborted",
					"Login requires an interactive session.").
					WithExitCode(clierrors.ExitCancelled)
			}
			secureStorage := cmdutil.IsSecureStorage(cmd)
			return runToken(ctx, secureStorage)
		},
	}
	return cmd
}

func runToken(ctx context.Context, secureStorage bool) error {
	cfg, err := config.Load(ctx, secureStorage)
	if err != nil {
		return clierrors.New("settings.load_failed", "Failed to load settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	p := tea.NewProgram(ui.NewTokenModel())
	anyModel, err := p.Run()
	if err != nil {
		return err
	}

	m := anyModel.(ui.TokenModel)
	if m.Quitting() {
		return nil
	}

	cfg.Token = m.Token()
	if err := config.Save(ctx, cfg, secureStorage); err != nil {
		return clierrors.New("settings.save_failed", "Failed to save settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	path, _ := config.Path()
	if secureStorage {
		path = "keyring"
	}
	iostream.ErrPrintf(ctx, "%s Saved %s to %s\n", iostream.Symbol(ctx, "✓", "OK:"), "token", path)
	return nil
}
