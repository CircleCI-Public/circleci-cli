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
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newProcessCmd() *cobra.Command {
	var (
		orgID          string
		orgSlug        string
		pipelineParams string
	)

	cmd := &cobra.Command{
		Use:   "process <path>",
		Short: "Compile and expand a pipeline config file",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<path> is the path to a pipeline config file to compile,
				e.g. ".circleci/config.yml". Pass "-" to read the config
				from stdin.
			`),
		},
		Long: heredoc.Doc(`
			Compile a CircleCI pipeline config file and print the fully expanded YAML.

			Orb references are inlined, matrix jobs are expanded, and pipeline
			parameter expressions are resolved. The output is the config that
			CircleCI would actually execute.

			Pass a file path or "-" to read from stdin.
		`),
		Example: heredoc.Doc(`
			# Process the default config
			$ circleci config process .circleci/config.yml

			# Process with pipeline parameters
			$ circleci config process .circleci/config.yml --pipeline-parameters 'env: staging'

			# Process with private orb resolution
			$ circleci config process .circleci/config.yml --org-slug gh/myorg

			# Read from stdin
			$ cat .circleci/config.yml | circleci config process -
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

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

			orgID = resolveOrgID(ctx, client, orgSlug, orgID)

			result, err := configcmd.Process(ctx, client, configYAML, orgID, params)
			if err != nil {
				return configAPIErr(err)
			}

			if !result.Valid {
				for _, e := range result.Errors {
					iostream.ErrPrintf(ctx, "  • %s\n", e)
				}
				return clierrors.New("config.invalid", "Config is invalid",
					fmt.Sprintf("Config file %q contains compilation errors.", args[0])).
					WithExitCode(clierrors.ExitValidationFail)
			}

			_, _ = fmt.Fprint(iostream.Out(ctx), result.CompiledYAML)
			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization UUID for private orb resolution")
	cmd.Flags().StringVarP(&orgSlug, "org-slug", "o", "", "Organization slug for private orb resolution (e.g. gh/myorg)")
	cmd.Flags().StringVar(&pipelineParams, "pipeline-parameters", "", "Pipeline parameters as a YAML map or path to a YAML file")

	return cmd
}

// parsePipelineParams parses pipeline parameters from either a YAML/JSON string
// or a file path. File is tried first; if not found, the value is parsed as inline YAML.
func parsePipelineParams(input string) (map[string]any, error) {
	if input == "" {
		return nil, nil
	}

	// Try as file first; only fall through to inline parsing when the path does not exist.
	b, fileErr := os.ReadFile(input) //#nosec:G304 // input is a user-supplied --pipeline-parameters flag value
	if fileErr != nil && !os.IsNotExist(fileErr) {
		return nil, fmt.Errorf("reading parameter file: %w", fileErr)
	}
	if fileErr == nil {
		var params map[string]any
		if err := yaml.Unmarshal(b, &params); err != nil {
			return nil, fmt.Errorf("parsing parameter file: %w", err)
		}
		return params, nil
	}

	// Fall back to inline YAML/JSON.
	var params map[string]any
	if err := yaml.Unmarshal([]byte(input), &params); err != nil {
		return nil, fmt.Errorf("parsing inline parameters: %w", err)
	}
	if params == nil && strings.TrimSpace(input) != "" {
		return nil, fmt.Errorf("parameters must be a YAML map")
	}
	return params, nil
}
