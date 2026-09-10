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
	"os"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/browser"
	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/agent"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/oauth"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// defaultCallbackTimeout caps how long we'll wait for the user to complete the
// browser-based authorization. Long enough to log in and approve consent;
// short enough that an abandoned terminal eventually gives up.
const defaultCallbackTimeout = 5 * time.Minute

// callbackTimeout returns the timeout for the OAuth callback wait. Tests may
// override the default by setting CIRCLE_LOGIN_TIMEOUT to a duration string
// parseable by time.ParseDuration (e.g. "100ms", "30s").
func callbackTimeout() time.Duration {
	if v := os.Getenv("CIRCLE_LOGIN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultCallbackTimeout
}

func newLoginCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a CircleCI account",
		Long: heredoc.Doc(`
			Log in to CircleCI by opening the OAuth authorization page in your
			browser. After you approve the request, an authorization code is
			delivered back to a temporary loopback server on 127.0.0.1, then
			exchanged for an access token via POST /oauth/token.

			The token is saved to the system keyring (or to the YAML config
			when --insecure-storage is set) and used automatically by all
			subsequent CLI commands.
		`),
		Example: heredoc.Doc(`
			# Open the browser and authorize the CLI
			$ circleci auth login

			# Print the authorize URL instead of opening a browser
			$ circleci auth login --no-browser

			# Authenticate against a non-default host
			$ CIRCLE_HOST=https://example.circleci.com circleci auth login
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			secureStorage := cmdutil.IsSecureStorage(cmd)
			configPath := cmdutil.ConfigPath(cmd)
			return runLogin(ctx, noBrowser, secureStorage, configPath)
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorize URL instead of opening a browser")
	return cmd
}

func runLogin(ctx context.Context, noBrowser, secureStorage bool, configPath string) error {
	// Interactive path: the full TUI (host selection → method → browser OAuth)
	// is handled by LoginFlowModel.
	if !noBrowser && iostream.IsInteractive(ctx) {
		return runLoginInteractive(ctx, secureStorage, configPath)
	}

	// Non-interactive / --no-browser: load host from config and print the URL.
	cfg := cmdutil.GetConfig(ctx)
	host := cfg.EffectiveHost()
	deviceID := cfg.DeviceID()

	return runLoginBrowser(ctx, host, deviceID.String(), false, noBrowser, secureStorage, configPath)
}

// shouldOpenBrowser reports whether the non-interactive flow should open the
// authorize URL itself rather than only printing it. agentName is the result of
// agent.Detect() — empty when no AI coding agent is driving the CLI. It is
// passed in rather than detected here so the decision stays testable without
// unsetting every agent environment variable.
//
// The interactive TUI deliberately waits for the user to press Enter before
// opening a browser, and is unaffected by this: it runs in runLoginInteractive
// and never reaches here. This only ever applies to the non-interactive path,
// and there only when an AI coding agent is driving the CLI. An agent has no
// TTY, so it never gets the TUI — but a human is sitting at this machine
// watching the agent rather than the terminal, and the URL we print lands in
// tool output they may never see. Opening the browser is what actually puts
// them in front of the consent screen.
//
// --no-browser always wins, and CI / CIRCLE_NO_INTERACTIVE suppress it so an
// agent running inside a CI job never tries to spawn a browser on a headless
// runner.
func shouldOpenBrowser(noBrowser bool, agentName string) bool {
	return !noBrowser && agentName != "" && iostream.EnvAllowsInteractive()
}

func runLoginBrowser(ctx context.Context, host string, deviceID string, signup, noBrowser, secureStorage bool, configPath string) error {
	var flow *oauth.Flow
	var err error
	if signup {
		flow, err = oauth.StartSignup(ctx, host, deviceID, runtime.GOOS)
	} else {
		flow, err = oauth.Start(ctx, host, deviceID, runtime.GOOS)
	}
	if err != nil {
		return clierrors.New("auth.login.start_failed",
			"Could not start the login flow", err.Error()).
			WithSuggestions(
				"Check that no other process is blocking loopback ports",
				"Verify the host is reachable and supports OAuth: "+host).
			WithExitCode(clierrors.ExitGeneralError)
	}
	defer func() { _ = flow.Close() }()

	if shouldOpenBrowser(noBrowser, agent.Detect()) {
		iostream.ErrPrintf(ctx, "Opening this URL in your browser — approve the request there to continue:\n\n  %s\n\n", flow.AuthorizeURL)
		// Fire and forget, and deliberately after the URL is printed. The
		// opener runs under cmd.Run(), which for some Linux browsers does not
		// return until the browser itself exits, so it must not sit between
		// here and flow.Wait below. The URL is already on stderr, so failing
		// to launch anything costs nothing over just printing it.
		go func() { _ = browser.OpenURL(flow.AuthorizeURL) }()
	} else {
		iostream.ErrPrintf(ctx, "Open this URL in your browser to continue:\n\n  %s\n\n", flow.AuthorizeURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, callbackTimeout())
	defer cancel()

	sp := iostream.Spinner(ctx, true, "Waiting for browser authentication")
	res, err := flow.Wait(waitCtx)
	sp.Stop()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			cmd := "circleci auth login"
			if signup {
				cmd = "circleci auth signup"
			}
			return clierrors.New("auth.login.timeout",
				"Login timed out",
				"Login timed out — no browser callback received within "+callbackTimeout().String()+".").
				WithSuggestions("Re-run '" + cmd + "' and complete the browser flow promptly").
				WithExitCode(clierrors.ExitTimeout)
		}
		return clierrors.New("auth.login.callback_error",
			"Authorization failed", err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	}

	token, err := flow.Exchange(ctx, res.Code)
	if err != nil {
		return clierrors.New("auth.login.token_exchange_failed",
			"Failed to exchange authorization code for token", err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	}

	return persistLoginToken(ctx, host, token.AccessToken, secureStorage, configPath)
}

func persistLoginToken(ctx context.Context, host, accessToken string, secureStorage bool, path string) error {
	c := apiclient.New(apiclient.Config{
		BaseURL: host,
		Token:   accessToken,
		Version: cmdutil.GetVersion(ctx),
	})
	me, err := c.GetMe(ctx)
	if err != nil {
		return err
	}

	iostream.ErrPrintf(ctx, "%s Logged in as %s\n", iostream.SymbolOK(ctx), me.Login)
	return persistToken(ctx, host, accessToken, me.ID, secureStorage, path)
}

// runLoginInteractive runs the full multi-stage login TUI. It covers host
// selection, auth method selection, and the browser OAuth flow. If the user
// picks "Paste a token", it falls through to the token input TUI using the
// host they selected.
func runLoginInteractive(ctx context.Context, secureStorage bool, configPath string) error {
	cfg := cmdutil.GetConfig(ctx)

	model := ui.NewLoginFlow(ctx, ui.LoginFlowOptions{
		DeviceID:        cfg.DeviceID().String(),
		OSInfo:          runtime.GOOS,
		CallbackTimeout: callbackTimeout(),
		Color:           iostream.ColorEnabled(ctx),
		GetUser: func(ctx context.Context, host, token string) (id uuid.UUID, username string, err error) {
			client := apiclient.New(apiclient.Config{
				BaseURL: host,
				Token:   token,
				Version: cmdutil.GetVersion(ctx),
			})
			me, err := client.GetMe(ctx)
			if err != nil {
				return uuid.Nil, "", err
			}
			return me.ID, me.Login, nil
		},
	})
	p := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	)
	final, err := p.Run()
	if err != nil {
		return clierrors.New("auth.login.prompt_failed",
			"Failed to run login prompt", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	m := final.(ui.LoginFlowModel)
	defer m.Close()
	res := m.Result()

	switch {
	case res.Cancelled:
		return clierrors.New("auth.login.cancelled",
			"Login cancelled",
			"Interrupted before authorization started.").
			WithExitCode(clierrors.ExitCancelled)
	case res.Err != nil:
		return clierrors.New("auth.login.failed",
			"Login failed", res.Err.Error()).
			WithExitCode(clierrors.ExitAuthError)
	default:
		return persistToken(ctx, res.Host, res.Token, res.UserID, secureStorage, configPath)
	}
}
