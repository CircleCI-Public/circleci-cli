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

package runner

import (
	"context"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newConfigCmd() *cobra.Command {
	var nickname string
	var tokenValue string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "config <resource-class>",
		Short: "Generate a runner agent configuration file",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				resource-class is the runner resource class to generate config for,
				in the form "namespace/name" (e.g. my-org/my-runner).
			`),
		},
		Long: heredoc.Doc(`
			Generate a runner agent configuration YAML for a resource class.

			By default, a new authentication token is created and the resulting
			config is written to stdout. Redirect stdout or use --output to
			write directly to a file.

			If you already have a token value (e.g. from a prior
			'circleci runner token create'), pass it with --token to generate
			the YAML without making an API call.

			The generated YAML is suitable for use as the runner agent's
			launch-agent-config.yaml.
		`),
		Example: heredoc.Doc(`
			# Create a new token and print config to stdout
			$ circleci runner config my-org/my-runner

			# Write config directly to a file
			$ circleci runner config my-org/my-runner --output launch-agent-config.yaml

			# Create a nicely-labeled token and write to a file
			$ circleci runner config my-org/my-runner --nickname "prod-server-1" --output /etc/circleci/launch-agent-config.yaml

			# Generate config from an existing token value (no API token creation)
			$ circleci runner config my-org/my-runner --token "$EXISTING_TOKEN_VALUE"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "resource-class"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()

			var tok string
			if tokenValue != "" {
				tok = tokenValue
			} else {
				client, err := cmdutil.LoadClient(ctx)
				if err != nil {
					return err
				}
				created, err := runConfigCreateToken(ctx, client, args[0], nickname)
				if err != nil {
					return err
				}
				tok = created
			}

			var w io.Writer
			if outputPath != "" {
				f, err := os.Create(outputPath) //#nosec:G304 // path is user-supplied
				if err != nil {
					return clierrors.New("runner.config_write_failed", "Could not write config file",
						err.Error()).
						WithExitCode(clierrors.ExitGeneralError)
				}
				defer func() { _ = f.Close() }()
				w = f
			} else {
				w = iostream.Get(ctx).Out
			}

			return writeAgentConfig(tok, w)
		},
	}

	cmd.Flags().StringVar(&nickname, "nickname", "", "Nickname for the new token")
	cmd.Flags().StringVar(&tokenValue, "token", "", "Use an existing token value instead of creating a new one")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write config to this file instead of stdout")

	return cmd
}

func runConfigCreateToken(ctx context.Context, client *apiclient.Client, resourceClass, nickname string) (string, error) {
	tok, err := client.CreateRunnerToken(ctx, resourceClass, nickname)
	if err != nil {
		return "", apiErr(err, resourceClass)
	}
	return tok.Token, nil
}

type agentConfig struct {
	API agentAPIConfig `yaml:"api"`
}

type agentAPIConfig struct {
	AuthToken string `yaml:"auth_token"`
}

func writeAgentConfig(token string, w io.Writer) error {
	return yaml.NewEncoder(w).Encode(agentConfig{
		API: agentAPIConfig{AuthToken: token},
	})
}
