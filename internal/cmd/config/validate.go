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

package cmdconfig

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newValidateCmd() *cobra.Command {
	var (
		configPath  string
		orgID       string
		orgSlug     string
		previewNext bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a pipeline config file",
		Long: heredoc.Doc(`
			Validate a CircleCI pipeline config file against the CircleCI API.

			Reads .circleci/config.yml by default. Pass --config to specify a
			different file, or "-" to read from stdin.

			JSON fields (--json):
			  valid          bool    whether the config compiled without errors
			  compiled_yaml  string  the fully expanded config (when valid)
			  errors         array   compilation error messages (when invalid)
		`),
		Example: heredoc.Doc(`
			# Validate the default config file
			$ circleci config validate

			# Validate a specific file
			$ circleci config validate --config path/to/config.yml

			# Validate with private orb resolution for your org
			$ circleci config validate --org-slug gh/myorg

			# Validate and output as JSON
			$ circleci config validate --json
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			yaml, err := readConfigInput(ctx, configPath)
			if err != nil {
				return err
			}

			orgID = resolveOrgID(ctx, client, orgSlug, orgID)

			result, err := configcmd.Validate(ctx, client, yaml, orgID, previewNext)
			if err != nil {
				return configAPIErr(err)
			}

			if jsonOut {
				if err := cmdutil.WriteJSON(iostream.Out(ctx), result); err != nil {
					return err
				}
				if !result.Valid {
					return clierrors.New("config.invalid", "Config is invalid",
						fmt.Sprintf("Config file %q contains compilation errors.", configPath)).
						WithExitCode(clierrors.ExitValidationFail)
				}
				return nil
			}

			if !result.Valid {
				for _, e := range result.Errors {
					iostream.ErrPrintf(ctx, "  • %s\n", e)
				}
				return clierrors.New("config.invalid", "Config is invalid",
					fmt.Sprintf("Config file %q contains compilation errors.", configPath)).
					WithExitCode(clierrors.ExitValidationFail)
			}

			iostream.Printf(ctx, "Config file at %q is valid.\n", configPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", ".circleci/config.yml", "Path to config file (use \"-\" for stdin)")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization UUID for private orb resolution")
	cmd.Flags().BoolVarP(&previewNext, "next", "n", false, "Enable config next which previews upcoming potentially breaking config changes")
	cmd.Flags().StringVarP(&orgSlug, "org-slug", "o", "", "Organization slug for private orb resolution (e.g. gh/myorg)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)

	return cmd
}
