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

// resolveOrgID returns the org UUID to use for compilation.
// --org-id takes precedence. --org-slug triggers a direct org lookup.
// If the lookup fails, a warning is printed and "" is returned
// (private orb resolution will be skipped, but the compile call still proceeds).
func resolveOrgID(ctx context.Context, client *apiclient.Client, orgSlug, orgID string) string {
	if orgID != "" {
		return orgID
	}
	if orgSlug == "" {
		return ""
	}
	org, err := client.GetOrg(ctx, orgSlug)
	if err != nil {
		iostream.ErrPrintf(ctx, "warning: could not resolve org %q: %s\n", orgSlug, err)
		return ""
	}
	return org.ID
}

func configAPIErr(err error) *clierrors.CLIError {
	return cmdutil.APIErr(err, "", "config.api_error", "Config API request failed")
}
