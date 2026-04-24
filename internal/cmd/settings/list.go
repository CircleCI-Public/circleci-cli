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

package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List current CLI settings",
		Long: heredoc.Doc(`
			Display the current CLI settings.

			The token value is masked for security. Settings are read from
			$XDG_CONFIG_HOME/circleci/config.yml (default: ~/.config/circleci/config.yml).

			JSON fields: token_set, host
		`),
		Example: heredoc.Doc(`
			# Show current settings
			$ circleci settings list

			# Output as JSON
			$ circleci settings list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			secureStorage := cmdutil.IsSecureStorage(cmd)

			return runList(ctx, secureStorage, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func runList(ctx context.Context, secureStorage bool, jsonOut bool) error {
	cfg, err := config.Load(ctx, secureStorage)
	if err != nil {
		return clierrors.New("settings.load_failed", "Failed to load settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	path, _ := config.Path()

	tokenSet := cfg.EffectiveToken() != ""

	if jsonOut {
		out := map[string]any{
			"token_set": tokenSet,
			"host":      cfg.EffectiveHost(),
		}
		enc := json.NewEncoder(iostream.Out(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	iostream.Printf(ctx, "Config file: %s\n\n", path)
	iostream.Printf(ctx, "%-10s  %s\n", "token", maskToken(cfg.EffectiveToken()))
	iostream.Printf(ctx, "%-10s  %s\n", "host", cfg.EffectiveHost())
	return nil
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s...%s", token[:4], token[len(token)-4:])
}
