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
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/runnerconfig"
)

func newConfigCmd() *cobra.Command {
	var nickname string
	var tokenValue string
	var outputPath string
	var product string
	var name string
	var workingDirectory string

	cmd := &cobra.Command{
		Use:   "config <resource-class>",
		Short: "Generate configuration for a self-hosted runner",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<resource-class>%[1]s is the runner resource class to generate config for,
				in the form %[1]snamespace/name%[1]s (for example, %[1]smy-org/my-runner%[1]s).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Generate the configuration a self-hosted runner needs to start. --product
			machine (the default) emits an agent circleci-runner-config.yaml for machine
			runner 3 (circleci-runner 3.x); container and provisioner emit Helm values.
		`),
		Example: heredoc.Doc(`
			# Generate machine runner 3 config, using the hostname as the runner name
			$ circleci runner config my-org/my-runner

			# Write machine runner config to the path the agent reads
			$ circleci runner config my-org/my-runner --name prod-server-1 --output /etc/circleci-runner/circleci-runner-config.yaml

			# Generate Helm values for container runner
			$ circleci runner config my-org/my-runner --product container --output values.yaml

			# Generate config from an existing token value (no API token creation)
			$ circleci runner config my-org/my-runner --token "$EXISTING_TOKEN_VALUE"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "resource-class"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()

			// Everything that can fail without touching the API is resolved
			// first, so a bad flag or a cancelled prompt never leaves a freshly
			// minted token stranded on the resource class.
			target, err := resolveProduct(ctx, product)
			if err != nil {
				return err
			}

			opts := runnerconfig.Options{
				ResourceClass:    args[0],
				Name:             name,
				WorkingDirectory: workingDirectory,
			}
			if cliErr := prepareOptions(target, &opts); cliErr != nil {
				return cliErr
			}

			if tokenValue != "" {
				opts.Token = tokenValue
			} else {
				client, err := cmdutil.LoadClient(ctx)
				if err != nil {
					return err
				}
				created, err := runConfigCreateToken(ctx, client, opts.ResourceClass, nickname)
				if err != nil {
					return err
				}
				opts.Token = created
			}

			body, err := runnerconfig.Render(target, opts)
			if err != nil {
				return err
			}

			return writeGeneratedConfig(ctx, outputPath, body)
		},
	}

	cmd.Flags().StringVar(&product, "product", "",
		"Runner product: machine|container|provisioner (default machine, prompts when unset)")
	cmd.Flags().StringVar(&name, "name", "", "Runner name, machine only (default: hostname)")
	cmd.Flags().StringVar(&workingDirectory, "working-directory", "", "Job working directory, machine only")
	cmd.Flags().StringVar(&nickname, "nickname", "", "Nickname for the new token")
	cmd.Flags().StringVar(&tokenValue, "token", "", "Use an existing token value instead of creating a new one")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write config to this file instead of stdout")

	return cmd
}

// resolveProduct turns the --product flag into a target. When the flag is
// omitted an interactive run is asked to confirm, and a non-interactive one
// keeps the documented machine default so scripts predating the flag still work.
func resolveProduct(ctx context.Context, flag string) (runnerconfig.Product, error) {
	if flag != "" {
		target, cliErr := runnerconfig.ParseProduct(flag)
		if cliErr != nil {
			return "", cliErr
		}
		return target, nil
	}
	if !iostream.IsInteractive(ctx) {
		return runnerconfig.Machine, nil
	}

	idx, err := iostream.PromptSelectDefault(ctx, "Runner product", runnerconfig.Products, 0)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("runner.config_cancelled", "Aborted", "No runner product selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	return runnerconfig.Product(runnerconfig.Products[idx]), nil
}

// prepareOptions fills in the machine defaults and rejects machine-only flags
// on the Helm products, rather than silently dropping them from the output.
func prepareOptions(target runnerconfig.Product, opts *runnerconfig.Options) *clierrors.CLIError {
	if target != runnerconfig.Machine {
		// Ordered, so the reported flag does not depend on map iteration.
		for _, f := range []struct{ flag, value string }{
			{"--name", opts.Name},
			{"--working-directory", opts.WorkingDirectory},
		} {
			if f.value != "" {
				return clierrors.New("runner.flag_not_applicable", "Flag does not apply to this product",
					f.flag+" applies to --product machine only.").
					WithSuggestions("Drop " + f.flag + ", or use --product machine").
					WithExitCode(clierrors.ExitBadArguments)
			}
		}
		return nil
	}

	if opts.Name == "" {
		opts.Name = runnerconfig.DefaultName()
	}
	return runnerconfig.ValidateName(opts.Name)
}

// writeGeneratedConfig writes body to path, or to stdout when path is empty.
// The file holds a runner token, so it is created 0600 and chmodded on the way
// through in case it already existed with looser permissions.
func writeGeneratedConfig(ctx context.Context, path string, body []byte) error {
	if path == "" {
		_, err := iostream.Get(ctx).Out.Write(body)
		return err
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return configWriteErr(err)
		}
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return configWriteErr(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return configWriteErr(err)
	}
	return nil
}

func configWriteErr(err error) *clierrors.CLIError {
	return clierrors.New("runner.config_write_failed", "Could not write config file", err.Error()).
		WithExitCode(clierrors.ExitGeneralError)
}

func runConfigCreateToken(ctx context.Context, client *apiclient.Client, resourceClass, nickname string) (string, error) {
	tok, err := client.CreateRunnerToken(ctx, resourceClass, nickname)
	if err != nil {
		return "", apiErr(err, resourceClass)
	}
	return tok.Token, nil
}
