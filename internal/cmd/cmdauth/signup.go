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
	"fmt"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

func newSignupCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Sign-up for a new CircleCI account",
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
			$ CIRCLE_HOST=https://example.circleci.com circleci auth signup
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			secureStorage := cmdutil.IsSecureStorage(cmd)
			configPath := cmdutil.ConfigPath(cmd)
			return runSignup(ctx, noBrowser, secureStorage, configPath)
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorize URL instead of opening a browser")
	return cmd
}

// SignupOutcome describes what SignupIfNeeded did.
type SignupOutcome string

// Possible SignupOutcome values.
const (
	SignupAlreadyAuthenticated SignupOutcome = "already_authenticated"
	SignupCompleted            SignupOutcome = "completed"
)

// SignupResult is returned by SignupIfNeeded so callers can distinguish
// between "already signed in" and "signup just completed."
type SignupResult struct {
	Outcome SignupOutcome
}

// SignupIfNeeded is the idempotent variant of signup used by orchestrators
// like `circleci onboard`. If a valid token already exists, it prints a
// confirmation and returns the AlreadyAuthenticated outcome. Otherwise it
// runs the standard signup flow and returns the Completed outcome.
func SignupIfNeeded(ctx context.Context, noBrowser, secureStorage bool, configPath string) (SignupResult, error) {
	cfg := cmdutil.GetConfig(ctx)

	token := cfg.EffectiveToken()
	if token == "" {
		iostream.Printf(ctx, "%s", heredoc.Doc(`

			Sign up to create your CircleCI account. Unlock:
			  • Run only tests changed in your PR, up to 4x faster
			  • Parallelize your test suite across containers instantly
			  • Connect Cursor, Claude, or Copilot to your pipelines
			  • Insights: flag flaky tests and slow jobs after run 1
			  • Deploy visibility across your whole org, out of the box

		`))
		if err := runSignup(ctx, noBrowser, secureStorage, configPath); err != nil {
			return SignupResult{}, err
		}
		return SignupResult{Outcome: SignupCompleted}, nil
	}

	client := apiclient.New(apiclient.Config{
		BaseURL: cfg.EffectiveHost(),
		Token:   token,
		Version: cmdutil.GetVersion(ctx),
	})
	me, err := client.GetMe(ctx)
	if err != nil {
		return SignupResult{}, clierrors.New(
			"auth.signup.stale_token",
			"Could not verify existing token",
			fmt.Sprintf("A token is configured but the identity check failed: %s.", err),
		).WithSuggestions(
			"Run 'circleci auth logout', then re-run 'circleci onboard'",
			"Check connectivity to the CircleCI host",
		).WithExitCode(clierrors.ExitAuthError)
	}

	iostream.Printf(ctx, "%s Already signed in as %s\n", iostream.SymbolOK(ctx), me.Login)
	return SignupResult{Outcome: SignupAlreadyAuthenticated}, nil
}

func runSignup(ctx context.Context, noBrowser, secureStorage bool, configPath string) error {
	cfg := cmdutil.GetConfig(ctx)

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
	deviceID := cfg.DeviceID()

	if !noBrowser && iostream.IsInteractive(ctx) {
		return runSignupInteractive(ctx, secureStorage, configPath)
	}

	return runLoginBrowser(ctx, host, deviceID.String(), true, secureStorage, configPath)
}

func runSignupInteractive(ctx context.Context, secureStorage bool, configPath string) error {
	cfg := cmdutil.GetConfig(ctx)
	deviceID := cfg.DeviceID()

	model := ui.NewLoginFlow(ctx, ui.LoginFlowOptions{
		DeviceID:        deviceID.String(),
		OSInfo:          runtime.GOOS,
		Signup:          true,
		CallbackTimeout: callbackTimeout(),
		Color:           iostream.ColorEnabled(ctx),
		GetUser: func(ctx context.Context, host, token string) (uuid.UUID, string, error) {
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
		return persistToken(ctx, res.Host, res.Token, res.UserID, secureStorage, configPath)
	}
}
