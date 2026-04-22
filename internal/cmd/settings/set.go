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
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a CLI setting",
		Long: heredoc.Doc(`
			Set a CLI setting by key.

			Supported keys:
			  token   Your CircleCI personal API token
			  host    CircleCI server host (default: https://circleci.com)
		`),
		Example: heredoc.Doc(`
			# Store your personal API token
			$ circleci settings set token mytoken123

			# Point to a self-hosted CircleCI server
			$ circleci settings set host https://circleci.mycompany.com

			# You can also supply the token via environment variable
			$ CIRCLECI_TOKEN=mytoken123 circleci pipeline get
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "key", "value"); cliErr != nil {
				return cliErr
			}
			streams := iostream.FromCmd(cmd)
			return runSet(streams, args[0], args[1])
		},
	}
	return cmd
}

func runSet(streams iostream.Streams, key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return clierrors.New("settings.load_failed", "Failed to load settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	switch key {
	case "token":
		cfg.Token = value
	case "host":
		cfg.Host = value
	default:
		return clierrors.New("settings.unknown_key", "Unknown setting", "Unknown setting key: "+key).
			WithSuggestions("Valid keys are: token, host").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if err := config.Save(cfg); err != nil {
		return clierrors.New("settings.save_failed", "Failed to save settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	path, _ := config.Path()
	streams.ErrPrintf("%s Saved %s to %s\n", streams.Symbol("✓", "OK:"), key, path)
	return nil
}
