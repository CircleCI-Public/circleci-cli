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
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a stored CLI setting",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<key>%[1]s is the setting to remove. Currently only "token" is supported.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Remove a stored CLI setting by key.

			Supported keys:
			  token      Remove your stored CircleCI personal API token
		`),
		Example: heredoc.Doc(`
			# Remove your stored API token
			$ circleci setting unset token
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			secureStorage := cmdutil.IsSecureStorage(cmd)
			configPath := cmdutil.ConfigPath(cmd)

			switch args[0] {
			case "token":
				res, err := config.DeleteToken(ctx, secureStorage)
				if err != nil {
					return clierrors.New("setting.unset_failed", "Failed to remove token", err.Error()).
						WithExitCode(clierrors.ExitGeneralError)
				}
				if res.Storage == config.StoredInKeyring {
					iostream.ErrPrintf(ctx, "%s Removed token from keyring\n", iostream.SymbolOK(ctx))
				} else {
					iostream.ErrPrintf(ctx, "%s Removed token from %s\n", iostream.SymbolOK(ctx), configPath)
				}
				return nil
			default:
				return clierrors.New("setting.unknown_key", "Unknown setting", "Unknown setting key: "+args[0]).
					WithSuggestions("Valid keys are: token").
					WithExitCode(clierrors.ExitBadArguments)
			}
		},
	}
	return cmd
}
