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

package telemetry

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Opt in to telemetry",
		Long: heredoc.Doc(`
			Enable anonymous usage telemetry for the CircleCI CLI.

			This preference is saved to the CLI config file. Environment variables
			(CIRCLECI_NO_TELEMETRY, NO_ANALYTICS, DO_NOT_TRACK, CI) always take
			precedence and will disable telemetry even when this setting is enabled.
		`),
		Example: heredoc.Doc(`
			# Opt in to telemetry
			$ circleci telemetry enable

			# Opt in using a custom config file path
			$ circleci --config ~/.config/circleci/config.yml telemetry enable

			# Verify the stored preference after enabling
			$ circleci settings list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			configPath, _ := cmd.Flags().GetString("config")
			return runEnable(ctx, configPath)
		},
	}
	return cmd
}

func runEnable(ctx context.Context, configPath string) error {
	resolvedPath := configPath
	if resolvedPath == "" {
		var err error
		resolvedPath, err = config.Path()
		if err != nil {
			return clierrors.New("telemetry.config_path", "Failed to resolve config path", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
	}

	if err := config.SetTelemetryEnabled(ctx, true, resolvedPath); err != nil {
		return clierrors.New("telemetry.save_failed", "Failed to save telemetry setting", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.ErrPrintf(ctx, "%s Telemetry enabled. Saved to %s\n", iostream.SymbolOK(ctx), resolvedPath)

	for _, env := range config.ActiveTelemetryOverrides() {
		iostream.ErrPrintf(ctx, "Note: %s is set — telemetry remains disabled for this session.\n", env)
	}

	return nil
}
