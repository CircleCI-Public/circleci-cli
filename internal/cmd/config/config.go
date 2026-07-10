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

// Package cmdconfig implements the "circleci config" command group, which
// works with the pipeline configuration file (.circleci/config.yml).
//
// The package is named cmdconfig rather than config to avoid colliding with
// internal/config (the CLI's own settings store).
package cmdconfig

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewConfigCmd returns the "circleci config" command group.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config <command>",
		GroupID: "ci",
		Short:   "Generate, validate, process and pack config YAML",
		Long: heredoc.Doc(`
			Work with the pipeline configuration file at .circleci/config.yml.

			This group manages the pipeline YAML that CircleCI executes. For CLI
			tool settings (API token, host, defaults), use 'circleci setting'.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newProcessCmd())
	cmd.AddCommand(newPackCmd())

	return cmd
}

// readConfigInput reads config YAML from path, or from stdin when path is "-".
func readConfigInput(ctx context.Context, path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(iostream.Get(ctx).In)
		if err != nil {
			return "", clierrors.New("config.read_failed", "Could not read config",
				fmt.Sprintf("Reading config from stdin: %s", err)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path) //#nosec:G304 // path is a user-supplied --config flag value, not arbitrary external input
	if err != nil {
		if os.IsNotExist(err) {
			return "", clierrors.New("config.not_found", "Config file not found",
				fmt.Sprintf("No config file found at %q.", path)).
				WithSuggestions(
					"Check the path and try again",
					"Run from the root of your project, or pass --config <path>",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		return "", clierrors.New("config.read_failed", "Could not read config",
			fmt.Sprintf("Reading %q: %s", path, err)).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return string(b), nil
}

// resolveOrgID returns the org UUID to use for private orb resolution during
// compilation.
//
// When --org is empty the org is inferred from the current project as a
// best-effort convenience, so configs that reference private or namespaced orbs
// validate without requiring the flag. Inference honours a `circleci project
// link` binding first and falls back to the git remote (see cmdutil.InferOrgID).
// If the org can't be determined (not a linked or git checkout, an unrecognised
// remote, or the project isn't found) it falls back to "" and private orb
// resolution is skipped — the compile call still proceeds so public configs
// validate anywhere.
//
// When --org is set explicitly, the slug or UUID is resolved via the API and an
// unresolvable value is a hard error, so a typo isn't silently ignored.
//
// cmdName is used only in the suggestion text of any resulting error.
func resolveOrgID(ctx context.Context, client *apiclient.Client, org, cmdName string) (string, error) {
	if org == "" {
		return cmdutil.InferOrgID(ctx, client), nil
	}
	id, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, cmdName)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func configAPIErr(err error) *clierrors.CLIError {
	return cmdutil.APIErr(err, "", "config.api_error", "Config API request failed")
}
