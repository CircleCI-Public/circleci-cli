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
	"bufio"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/oauth"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui"
)

// defaultCallbackTimeout caps how long we'll wait for the user to complete the
// browser-based authorization. Long enough to log in and approve consent;
// short enough that an abandoned terminal eventually gives up.
const defaultCallbackTimeout = 5 * time.Minute

// callbackTimeout returns the timeout for the OAuth callback wait. Tests may
// override the default by setting CIRCLECI_LOGIN_TIMEOUT to a duration string
// parseable by time.ParseDuration (e.g. "100ms", "30s").
func callbackTimeout() time.Duration {
	if v := os.Getenv("CIRCLECI_LOGIN_TIMEOUT"); v != "" {
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
		Short: "Authenticate via the CircleCI OAuth flow",
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
			$ CIRCLECI_HOST=https://example.circleci.com circleci auth login
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			configPath, _ := cmd.Flags().GetString("config")
			secureStorage := cmdutil.IsSecureStorage(cmd)
			return runLogin(ctx, configPath, noBrowser, secureStorage)
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorize URL instead of opening a browser")
	return cmd
}

func runLogin(ctx context.Context, configPath string, noBrowser, secureStorage bool) error {
	cfg, err := config.LoadFrom(ctx, configPath, false)
	if err != nil {
		return clierrors.New("config.load_failed", "Failed to load config", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	host := cfg.EffectiveHost()

	// In an interactive session (and unless --no-browser forces the URL-only
	// path), let the user pick between OAuth and pasting a token. --no-browser
	// and non-interactive sessions skip the prompt and fall through to the
	// browser flow, which prints the authorize URL when it can't open one.
	if !noBrowser && iostream.IsInteractive(ctx) {
		method, err := promptAuthMethod(ctx, host)
		if err != nil {
			return err
		}
		if method == authMethodToken {
			return runToken(ctx, configPath, secureStorage)
		}
	}

	deviceID := config.EnsureDeviceID(ctx, configPath)

	flow, err := oauth.Start(ctx, host, deviceID, runtime.GOOS)
	if err != nil {
		return clierrors.New("auth.login.listen_failed",
			"Could not start local callback server", err.Error()).
			WithSuggestions("Check that no other process is blocking loopback ports").
			WithExitCode(clierrors.ExitGeneralError)
	}
	defer func() { _ = flow.Close() }()

	if noBrowser || !iostream.IsInteractive(ctx) {
		iostream.ErrPrintf(ctx, "Open this URL in your browser to continue:\n\n  %s\n\n", flow.AuthorizeURL)
	} else {
		// Show the destination URL muted so the user can verify where they're
		// being sent (and so they have a fallback if the browser doesn't open).
		iostream.ErrPrintf(ctx, "%s\n", ui.HelperStyle.Render(flow.AuthorizeURL))
		iostream.ErrPrintf(ctx, "Press Enter to open %s in your browser...", hostDisplay(host))
		if err := waitForEnter(ctx); err != nil {
			return err
		}
		iostream.ErrPrintln(ctx)
		if err := browser.OpenURL(flow.AuthorizeURL); err != nil {
			iostream.ErrPrintf(ctx, "Could not open browser automatically. Open this URL manually:\n\n  %s\n\n", flow.AuthorizeURL)
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, callbackTimeout())
	defer cancel()

	sp := iostream.Spinner(ctx, true, "Waiting for browser authentication")
	res, err := flow.Wait(waitCtx)
	sp.Stop()
	if err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			return clierrors.New("auth.login.timeout",
				"Login timed out",
				"Login timed out — no browser callback received within "+callbackTimeout().String()+".").
				WithSuggestions("Re-run 'circleci auth login' and complete the browser flow promptly").
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

	return persistToken(ctx, host, token.AccessToken, secureStorage)
}

type authMethod int

const (
	authMethodBrowser authMethod = iota
	authMethodToken
)

// promptAuthMethod presents the login method picker. The default selection is
// browser-based OAuth; users can arrow down to "paste a token" instead.
func promptAuthMethod(ctx context.Context, host string) (authMethod, error) {
	model := ui.NewSelectModel(
		"How would you like to authenticate "+hostDisplay(host)+"?",
		[]string{"Login with a web browser", "Paste an authentication token"},
	)
	p := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	)
	finalModel, err := p.Run()
	if err != nil {
		return 0, clierrors.New("auth.login.prompt_failed",
			"Failed to read authentication method", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	m := finalModel.(ui.SelectModel)
	if m.Cancelled() {
		return 0, clierrors.New("auth.login.cancelled",
			"Login cancelled",
			"Interrupted before authorization started.").
			WithExitCode(clierrors.ExitCancelled)
	}
	if m.Selected() == 1 {
		return authMethodToken, nil
	}
	return authMethodBrowser, nil
}

// hostDisplay returns the bare hostname of a configured host URL (e.g.
// "circleci.com" for "https://circleci.com"). Falls back to the input string
// if it doesn't parse as a URL with a host component.
func hostDisplay(host string) string {
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		return u.Host
	}
	return host
}

// waitForEnter blocks until the user presses Enter on stdin or ctx is cancelled.
// A bare bufio.ReadString call would ignore ctx because the underlying read
// syscall doesn't return on signal — so Ctrl+C at the prompt would appear to
// hang. The read goroutine leaks if ctx fires first, but that's fine: the
// process is exiting anyway.
func waitForEnter(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		_, err := bufio.NewReader(iostream.In(ctx)).ReadString('\n')
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, io.EOF) {
			return clierrors.New("auth.login.prompt_failed",
				"Failed to read from stdin", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		return nil
	case <-ctx.Done():
		return clierrors.New("auth.login.cancelled",
			"Login cancelled",
			"Interrupted before authorization started.").
			WithExitCode(clierrors.ExitCancelled)
	}
}
