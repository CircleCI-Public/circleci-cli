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
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

func newSignupCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Sign up for CircleCI via the browser",
		Long: heredoc.Doc(`
				Create a CircleCI account by opening the signup page in your browser.
				This uses the same browser-based OAuth flow as 'circleci auth login'
				but routes you to the signup page instead of the login page.

				If a token is already configured, run 'circleci auth logout' first
				(signup will not silently overwrite an existing token). If you
				already have a CircleCI account, use 'circleci auth login' instead.
			`),
		Example: heredoc.Doc(`
			# Open the signup page in your browser
			$ circleci auth signup

			# Print the authorize URL instead of opening a browser
			$ circleci auth signup --no-browser

			# Authenticate against a non-default host
			$ CIRCLECI_HOST=https://example.circleci.com circleci auth signup
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			configPath, _ := cmd.Flags().GetString("config")
			secureStorage := cmdutil.IsSecureStorage(cmd)
			return runSignup(ctx, configPath, noBrowser, secureStorage)
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorize URL instead of opening a browser")
	return cmd
}

func runSignup(ctx context.Context, configPath string, noBrowser, secureStorage bool) error {
	cfg, err := config.LoadFrom(ctx, configPath, secureStorage)
	if err != nil {
		return clierrors.New("config.load_failed",
			"Failed to load config", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if cfg.EffectiveToken() != "" {
		return clierrors.New("auth.signup.already_authenticated",
			"Already authenticated",
			"Already authenticated — a token is already configured and signup will not overwrite it.").
			WithSuggestions(
				"Run 'circleci auth logout' to clear it, then re-run 'circleci auth signup'",
				"If you already have a CircleCI account, use 'circleci auth login'",
			).
			WithExitCode(clierrors.ExitAuthError)
	}

	host := cfg.EffectiveHost()
	deviceID := config.EnsureDeviceID(ctx, configPath)

	if !noBrowser && iostream.IsInteractive(ctx) {
		return runSignupInteractive(ctx, configPath, secureStorage)
	}

	return runLoginBrowser(ctx, host, deviceID, true, secureStorage)
}

func runSignupInteractive(ctx context.Context, configPath string, secureStorage bool) error {
	deviceID := config.EnsureDeviceID(ctx, configPath)

	model := ui.NewLoginFlow(ctx, ui.LoginFlowOptions{
		DeviceID:        deviceID,
		OSInfo:          runtime.GOOS,
		Signup:          true,
		CallbackTimeout: callbackTimeout(),
		Color:           iostream.ColorEnabled(ctx),
		GetUsername: func(ctx context.Context, host, token string) (string, error) {
			me, err := apiclient.New(host, token, nil).GetMe(ctx)
			if err != nil {
				return "", err
			}
			return me.Login, nil
		},
	})
	p := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	)
	final, err := p.Run()
	if err != nil {
		return clierrors.New("auth.signup.prompt_failed",
			"Failed to run signup prompt", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	m := final.(ui.LoginFlowModel)
	defer m.Close()
	res := m.Result()

	switch {
	case res.Cancelled:
		return clierrors.New("auth.signup.cancelled",
			"Signup cancelled",
			"Interrupted before authorization started.").
			WithExitCode(clierrors.ExitCancelled)
	case res.Err != nil:
		return clierrors.New("auth.signup.failed",
			"Signup failed", res.Err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	default:
		return persistToken(ctx, res.Host, res.Token, secureStorage)
	}
}
