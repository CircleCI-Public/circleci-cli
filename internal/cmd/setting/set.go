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

package setting

import (
	"context"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a CLI setting",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<key>%[1]s is the setting to change. Options are: %[1]stoken%[1]s, %[1]shost%[1]s, %[1]stelemetry%[1]s, or %[1]stheme%[1]s.
				- %[1]s<value>%[1]s is the value to store. Pass %[1]s-%[1]s to read it from stdin.
				  May be omitted for %[1]stheme%[1]s to pick interactively.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Set a CLI setting by key.

			Supported keys:
			  token      Your CircleCI personal API token
			  host       CircleCI server host (default: https://circleci.com)
			  telemetry  Enable or disable anonymous usage telemetry (on/off)
			  theme      Color theme for rendered output (default: auto)

			Pass "-" as the value to read it from stdin, keeping secrets out of
			shell history and process listings.

			Run 'circleci setting set theme' with no value in an interactive
			terminal to pick a theme from a list.
		`),
		Example: heredoc.Doc(`
			# Store your personal API token
			$ circleci setting set token mytoken123

			# Read the token from stdin to avoid shell history exposure
			$ echo "mytoken123" | circleci setting set token -

			# Point to a self-hosted CircleCI server
			$ circleci setting set host https://circleci.mycompany.com

			# Enable telemetry
			$ circleci setting set telemetry on

			# Disable telemetry
			$ circleci setting set telemetry off

			# Set the color theme used for rendered output
			$ circleci setting set theme dracula

			# Pick the color theme interactively
			$ circleci setting set theme
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			secureStorage := cmdutil.IsSecureStorage(cmd)
			configPath := cmdutil.ConfigPath(cmd)

			// "theme" may be selected interactively when the value is omitted
			// and we have an interactive terminal.
			if len(args) == 1 && args[0] == "theme" && iostream.IsInteractive(ctx) {
				value, err := promptTheme(ctx)
				if err != nil {
					return err
				}
				if value == "" {
					return nil // cancelled
				}
				return runSet(ctx, secureStorage, configPath, "theme", value)
			}

			if cliErr := cmdutil.RequireArgs(args, "key", "value"); cliErr != nil {
				return cliErr
			}
			value, err := iostream.ReadSecret(ctx, args[1])
			if err != nil {
				return clierrors.New("args.stdin_read_failed", "Failed to read value from stdin", err.Error()).
					WithExitCode(clierrors.ExitBadArguments)
			}
			return runSet(ctx, secureStorage, configPath, args[0], value)
		},
	}
	return cmd
}

func runSet(ctx context.Context, secureStorage bool, path, key, value string) (err error) {
	var res config.SaveResult
	switch key {
	case "token":
		res, err = config.SetToken(ctx, value, secureStorage)
	case "host":
		err = config.SetHost(ctx, value)
	case "telemetry":
		return runSetTelemetry(ctx, path, value)
	case "theme":
		return runSetTheme(ctx, path, value)
	default:
		return clierrors.New("setting.unknown_key", "Unknown setting", "Unknown setting key: "+key).
			WithSuggestions("Valid keys are: token, host, telemetry, theme").
			WithExitCode(clierrors.ExitBadArguments)
	}
	if err != nil {
		return clierrors.New("setting.save_failed", "Failed to save settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if res.Storage == config.StoredInKeyring {
		iostream.ErrPrintf(ctx, "%s Saved %s to keyring\n", iostream.SymbolOK(ctx), key)
	} else {
		iostream.ErrPrintf(ctx, "%s Saved %s to %s\n", iostream.SymbolOK(ctx), key, path)
	}
	if hint := cmdutil.KeyringConnectHint(res.KeyringErr); hint != "" {
		iostream.ErrPrintf(ctx, "%s %s\n", iostream.SymbolWarn(ctx), hint)
	}
	return nil
}

// promptTheme presents an interactive theme picker. The default theme is
// marked in the list, and the cursor starts on the currently-configured theme
// (falling back to the default when none is set). Returns "" if the user
// cancels.
func promptTheme(ctx context.Context) (string, error) {
	themes := iostream.ValidThemes()

	// Display labels mark the default theme, but the underlying values stay
	// intact so the chosen index maps back to a real theme name.
	labels := make([]string, len(themes))
	for i, t := range themes {
		if t == config.DefaultTheme {
			labels[i] = t + " (default)"
		} else {
			labels[i] = t
		}
	}

	// Start the cursor on the current theme; EffectiveTheme returns the default
	// when none is configured, so the cursor lands on the default in that case.
	current := cmdutil.GetConfig(ctx).EffectiveTheme()
	cursorIdx := slices.Index(themes, current)
	if cursorIdx < 0 {
		cursorIdx = slices.Index(themes, config.DefaultTheme)
	}

	idx, err := iostream.PromptThemePreview(ctx, "Select a theme", labels, themes, cursorIdx, themePreviewMarkdown)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", nil // cancelled
	}
	return themes[idx], nil
}

// themePreviewMarkdown is the sample document rendered in the right-hand pane of
// the interactive theme picker. It exercises the markdown elements a theme most
// visibly affects — headings, emphasis, links, lists, inline and block code,
// blockquotes, and tables — so each theme's colors are easy to compare.
const themePreviewMarkdown = `# Title

Preview of the **selected** theme.

## List

- ` + "`circleci run list`" + ` — *recent* runs

> Tip: pass ` + "`--json`" + ` to any command.

## Table

| Status  | Count |
| ------- | ----- |
| success |    42 |
| failed  |     3 |
`

func runSetTheme(ctx context.Context, path, value string) error {
	if !iostream.IsValidTheme(value) {
		return clierrors.New("setting.invalid_value", "Invalid theme value", "Invalid value for theme: "+value).
			WithSuggestions("Valid themes are: " + strings.Join(iostream.ValidThemes(), ", ")).
			WithExitCode(clierrors.ExitBadArguments)
	}

	if err := config.SetTheme(ctx, value); err != nil {
		return clierrors.New("setting.save_failed", "Failed to save theme setting", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.ErrPrintf(ctx, "%s Saved theme to %s\n", iostream.SymbolOK(ctx), path)
	return nil
}

func runSetTelemetry(ctx context.Context, path, value string) error {
	var enabled bool
	switch strings.ToLower(value) {
	case "on", "true", "yes", "1", "enabled":
		enabled = true
	case "off", "false", "no", "0", "disabled":
		enabled = false
	default:
		return clierrors.New("setting.invalid_value", "Invalid telemetry value", "Invalid value for telemetry: "+value).
			WithSuggestions("Valid values are: on, off").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if err := config.SetTelemetry(ctx, enabled, ""); err != nil {
		return clierrors.New("setting.save_failed", "Failed to save telemetry setting", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if enabled {
		iostream.ErrPrintf(ctx, "%s Telemetry enabled. Saved to %s\n", iostream.SymbolOK(ctx), path)
		for _, env := range config.ActiveTelemetryOverrides() {
			iostream.ErrPrintf(ctx, "Note: %s is set — telemetry remains disabled for this session.\n", env)
		}
	} else {
		iostream.ErrPrintf(ctx, "%s Telemetry disabled. Saved to %s\n", iostream.SymbolOK(ctx), path)
	}
	return nil
}
