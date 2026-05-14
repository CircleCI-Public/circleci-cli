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
	"errors"
	"runtime"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/oauth"
)

func newSignupCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Sign up for CircleCI via the browser",
		Long: heredoc.Doc(`
				Create a CircleCI account by opening the signup page in your browser.
				After you complete signup, return to your terminal and press Enter
				to authenticate the CLI with the regular browser login flow.

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

	flow, err := oauth.StartSignup(ctx, host, deviceID, runtime.GOOS)
	if err != nil {
		return clierrors.New("auth.signup.listen_failed",
			"Could not start local callback server", err.Error()).
			WithSuggestions("Check that no other process is blocking loopback ports").
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.ErrPrintf(ctx, "Open this URL in your browser to sign up:\n\n  %s\n\n", flow.AuthorizeURL)
	if !noBrowser {
		_ = browser.OpenURL(flow.AuthorizeURL) // best-effort
	}

	if iostream.IsInteractive(ctx) {
		continued, err := iostream.PromptContinue(ctx, "\nOnce you're signed in to CircleCI in the browser, press Enter here to continue with CLI authentication...")
		if err != nil {
			_ = flow.Close()
			return clierrors.New("auth.signup.prompt_failed",
				"Could not read login prompt", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		if !continued {
			_ = flow.Close()
			return signupCancelledError()
		}
	} else {
		_ = flow.Close()
		iostream.ErrPrintf(ctx, "After you finish signing up, run 'circleci auth login' to authenticate the CLI.\n")
		return nil
	}

	if finished, err := finishSignupCallbackIfAvailable(ctx, host, flow, secureStorage); err != nil {
		_ = flow.Close()
		return err
	} else if finished {
		_ = flow.Close()
		return nil
	}

	// Signup is only a launch step in this fallback path; release its loopback
	// server before starting the real login callback flow.
	_ = flow.Close()
	return runLoginBrowser(ctx, host, deviceID, !noBrowser, secureStorage)
}

func signupCancelledError() *clierrors.CLIError {
	return clierrors.New("auth.signup.cancelled",
		"Signup cancelled",
		"Signup cancelled before signup completed.").
		WithExitCode(clierrors.ExitCancelled)
}

func finishSignupCallbackIfAvailable(ctx context.Context, host string, flow *oauth.Flow, secureStorage bool) (bool, error) {
	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	res, err := flow.Wait(waitCtx)
	if errors.Is(err, context.DeadlineExceeded) {
		return false, nil
	}
	if err != nil {
		return false, clierrors.New("auth.signup.callback_error",
			"Signup failed", err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	}

	token, err := flow.Exchange(ctx, res.Code)
	if err != nil {
		return false, clierrors.New("auth.signup.token_exchange_failed",
			"Failed to exchange authorization code for token", err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	}

	return true, persistLoginToken(ctx, host, token.AccessToken, secureStorage)
}
