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

func newDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Opt out of telemetry",
		Long: heredoc.Doc(`
			Disable anonymous usage telemetry for the CircleCI CLI.

			This preference is saved to the CLI config file. You can re-enable
			telemetry at any time with 'circleci telemetry enable'.

			Alternatively, set CIRCLECI_NO_TELEMETRY, NO_ANALYTICS, or DO_NOT_TRACK
			in your environment to disable telemetry without modifying the config file.
		`),
		Example: heredoc.Doc(`
			# Opt out of telemetry
			$ circleci telemetry disable

			# Opt out using a custom config file path
			$ circleci --config ~/.config/circleci/config.yml telemetry disable

			# Disable telemetry for a single session without changing the config
			$ CIRCLECI_NO_TELEMETRY=1 circleci pipeline list
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			configPath, _ := cmd.Flags().GetString("config")
			return runDisable(ctx, configPath)
		},
	}
	return cmd
}

func runDisable(ctx context.Context, configPath string) error {
	resolvedPath := configPath
	if resolvedPath == "" {
		resolvedPath, _ = config.Path()
	}

	if err := config.SetTelemetryEnabled(ctx, false, configPath); err != nil {
		return clierrors.New("telemetry.save_failed", "Failed to save telemetry setting", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.ErrPrintf(ctx, "%s Telemetry disabled. Saved to %s\n", iostream.SymbolOK(ctx), resolvedPath)

	return nil
}
