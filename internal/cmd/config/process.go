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

func newProcessCmd() *cobra.Command {
	var (
		org            string
		previewNext    bool
		pipelineParams string
	)

	cmd := &cobra.Command{
		Use:   "process <path>",
		Short: "Compile and expand a pipeline config file",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the path to a pipeline config file to compile,
				for example, %[1]s.circleci/config.yml%[1]s. Pass %[1]s-%[1]s to read the config
				from stdin.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Compile a CircleCI pipeline config and print the fully expanded YAML —
			orbs inlined, matrices expanded, parameters resolved.

			No API token is required; without one, only public orbs resolve.
			Private and namespaced orbs need a token and resolve against your org, taken
			from --org, a 'circleci project link' binding, or the git remote, in that order.
		`),
		Example: heredoc.Doc(`
			# Process the default config
			$ circleci config process .circleci/config.yml

			# Process with pipeline parameters
			$ circleci config process .circleci/config.yml --pipeline-parameters 'env: staging'

			# Process against a specific org (otherwise inferred from the git remote)
			$ circleci config process .circleci/config.yml --org gh/myorg

			# Read from stdin
			$ cat .circleci/config.yml | circleci config process -
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Process (like validate) works without a token: the compile endpoint
			// serves anonymous callers, and expanding a config that uses only
			// public orbs is common in CI jobs and orb pipelines that have no
			// token. An authenticated call is unchanged.
			client := cmdutil.LoadClientOptionalAuth(ctx)

			configYAML, err := readConfigInput(ctx, args[0])
			if err != nil {
				return err
			}

			params, err := parsePipelineParams(pipelineParams)
			if err != nil {
				return clierrors.New("config.invalid_params", "Invalid pipeline parameters",
					fmt.Sprintf("Could not parse pipeline parameters: %s", err)).
					WithSuggestions("Pass parameters as a YAML map: --pipeline-parameters 'key: value'").
					WithExitCode(clierrors.ExitBadArguments)
			}

			orgID, err := optionalAuthOrgID(ctx, client, org, "circleci config process",
				"Or drop --org to process against public orbs only")
			if err != nil {
				return err
			}

			result, err := configcmd.Process(ctx, client, configYAML, orgID, previewNext, params)
			if err != nil {
				// A 401 on an anonymous call means this host will not compile
				// without credentials, so the generic "token was rejected" wording
				// APIErr uses for an authenticated 401 would be wrong here.
				if !client.Authenticated() && httpcl.HasStatusCode(err, http.StatusUnauthorized) {
					return clierrors.New("auth.token_missing", "Authentication required",
						"This CircleCI host requires an API token to process config.").
						WithSuggestions(
							"Run: circleci auth login",
							"Or set the CIRCLE_TOKEN environment variable",
						).
						WithExitCode(clierrors.ExitAuthError)
				}
				return configAPIErr(err)
			}

			if !result.Valid {
				printValidationErrors(ctx, result.Errors)
				return clierrors.New("config.invalid", "Config is invalid",
					fmt.Sprintf("Config file %q contains compilation errors.", args[0])).
					WithExitCode(clierrors.ExitValidationFail)
			}

			_, _ = fmt.Fprint(iostream.Out(ctx), result.CompiledYAML)
			return nil
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "for private orb resolution", DefaultsToGitRemote: true})
	cmd.Flags().BoolVarP(&previewNext, "next", "n", false, "Enable config next which previews upcoming potentially breaking config changes")
	cmd.Flags().StringVar(&pipelineParams, "pipeline-parameters", "", "Pipeline parameters as a YAML map or path to a YAML file")

	return cmd
}

// parsePipelineParams parses pipeline parameters from either a YAML/JSON string
// or a file path. File is tried first; if not found, the value is parsed as inline YAML.
func parsePipelineParams(input string) (map[string]any, error) {
	return configcmd.ParsePipelineParams(input)
}
