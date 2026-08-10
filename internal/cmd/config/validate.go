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
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
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
		Use:   "validate [<path>]",
		Short: "Validate a pipeline config file",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the pipeline config file to validate, by default
				%[1]s.circleci/config.yml%[1]s. Pass %[1]s-%[1]s to read the config from stdin.
			`, "`"),
		},
		Long: heredoc.Doc(`
			No API token is required; without one, only public orbs resolve.
			Private and namespaced orbs need a token and resolve against your org, taken
			from --org, a 'circleci project link' binding, or the git remote, in that order.

			JSON fields (--json): valid (bool), compiled_yaml (string, when valid), errors (array of compilation messages, when invalid)
		`),
		Example: heredoc.Doc(`
			# Validate the default config file
			$ circleci config validate

			# Validate a specific file
			$ circleci config validate path/to/config.yml

			# Validate against a specific org (otherwise inferred from the git remote)
			$ circleci config validate --org gh/myorg

			# Validate and output as JSON
			$ circleci config validate --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// A positional path wins over --config. The 0.1.x CLI documented
			// `circleci config validate <path>` as the primary form and kept
			// --config as a hidden alias, so scripts pass either or both.
			path := configPath
			if len(args) == 1 {
				path = args[0]
			}

			// Validate (like process) works without a token: the compile endpoint
			// serves anonymous callers, and linting a config that uses only public
			// orbs is the first thing a new user — or a pre-commit hook, or a CI
			// job without a token — wants to do. An authenticated call is unchanged.
			client := cmdutil.LoadClientOptionalAuth(ctx)

			yaml, err := readConfigInput(ctx, path)
			if err != nil {
				return err
			}

			orgID, err := optionalAuthOrgID(ctx, client, org, "circleci config validate",
				"Or drop --org to validate against public orbs only")
			if err != nil {
				return err
			}

			result, err := configcmd.Validate(ctx, client, yaml, orgID, previewNext)
			if err != nil {
				// A 401 on an anonymous call means this host will not compile
				// without credentials, so the generic "token was rejected" wording
				// APIErr uses for an authenticated 401 would be wrong here.
				if !client.Authenticated() && httpcl.HasStatusCode(err, http.StatusUnauthorized) {
					return clierrors.New("auth.token_missing", "Authentication required",
						"This CircleCI host requires an API token to validate config.").
						WithSuggestions(
							"Run: circleci auth login",
							"Or set the CIRCLE_TOKEN environment variable",
						).
						WithExitCode(clierrors.ExitAuthError)
				}
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
				printValidationErrors(ctx, result.Errors)
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
