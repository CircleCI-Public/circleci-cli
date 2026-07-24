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
		org         string
		previewNext bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a pipeline config file",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s[path]%[1]s is the path to a pipeline config file to validate,
				for example, %[1]s.circleci/config.yml%[1]s. Pass %[1]s-%[1]s to read the config
				from stdin. When omitted, --config is used (default %[1]s.circleci/config.yml%[1]s).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Validate a CircleCI pipeline config file against the CircleCI API.

			Reads the config at [path]. When [path] is omitted it falls back to
			--config, which defaults to .circleci/config.yml. Pass "-" (as the
			path or via --config) to read from stdin.

			Private and namespaced orbs are resolved against your organization.
			When --org is omitted the org is inferred from the current project —
			a 'circleci project link' binding if present, otherwise the git
			remote. Pass --org to override this, or to resolve private orbs
			outside a project.

			JSON fields (--json):
			  valid          bool    whether the config compiled without errors
			  compiled_yaml  string  the fully expanded config (when valid)
			  errors         array   compilation error messages (when invalid)
		`),
		Example: heredoc.Doc(`
			# Validate the default config file
			$ circleci config validate

			# Validate a specific file (positional path)
			$ circleci config validate path/to/config.yml

			# Validate a specific file (--config flag)
			$ circleci config validate --config path/to/config.yml

			# Validate against a specific org (otherwise inferred from the git remote)
			$ circleci config validate --org gh/myorg

			# Validate and output as JSON
			$ circleci config validate --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// A positional <path> takes precedence over --config so that
			// `circleci config validate <path>` honours the path (matching
			// `config process` and the 0.1.x CLI). Reject the ambiguous case
			// where both a positional path and an explicit --config are given
			// rather than silently ignoring one.
			path := configPath
			if len(args) > 0 {
				if cmd.Flags().Changed("config") {
					return clierrors.New("config.conflicting_path", "Conflicting config path",
						"A config path was given both as a positional argument and with --config. Pass it one way, not both.").
						WithExitCode(clierrors.ExitBadArguments)
				}
				path = args[0]
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			yaml, err := readConfigInput(ctx, path)
			if err != nil {
				return err
			}

			orgID, err := resolveOrgID(ctx, client, org, "circleci config validate")
			if err != nil {
				return err
			}

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
						fmt.Sprintf("Config file %q contains compilation errors.", path)).
						WithExitCode(clierrors.ExitValidationFail)
				}
				return nil
			}

			if !result.Valid {
				for _, e := range result.Errors {
					iostream.ErrPrintf(ctx, "  • %s\n", e)
				}
				return clierrors.New("config.invalid", "Config is invalid",
					fmt.Sprintf("Config file %q contains compilation errors.", path)).
					WithExitCode(clierrors.ExitValidationFail)
			}

			iostream.Printf(ctx, "Config file at %q is valid.\n", path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", ".circleci/config.yml", "Path to config file (use \"-\" for stdin)")
	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "for private orb resolution", DefaultsToGitRemote: true})
	cmd.Flags().BoolVarP(&previewNext, "next", "n", false, "Enable config next which previews upcoming potentially breaking config changes")
	cmdutil.AddJSONFlag(cmd, &jsonOut)

	return cmd
}
