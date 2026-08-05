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
	"strings"

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
	b, err := os.ReadFile(path) //#nosec:G304 // path is a user-supplied argument or --config flag value, not arbitrary external input
	if err != nil {
		if os.IsNotExist(err) {
			return "", clierrors.New("config.not_found", "Config file not found",
				fmt.Sprintf("No config file found at %q.", path)).
				WithSuggestions(
					"Check the path and try again",
					"Run from the root of your project, or pass the path to your config file",
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

// validateOrgID is resolveOrgID for `config validate`, which accepts an
// unauthenticated client (see cmdutil.LoadClientOptionalAuth).
//
// Every path to an org ID needs a token: inference looks the project up through
// the API, and an org only owns orbs the caller is authorized to read. So when
// there is no token we skip resolution entirely and compile with no owner —
// public orbs still resolve — rather than spending a round-trip on a request
// that can only 401. An explicit --org is a hard error instead: silently
// ignoring it would report a config as valid while skipping the private orb
// resolution the user asked for.
func validateOrgID(ctx context.Context, client *apiclient.Client, org string) (string, error) {
	if !client.Authenticated() {
		if org != "" {
			return "", clierrors.New("auth.token_missing", "Authentication required",
				"Resolving --org requires a CircleCI API token.").
				WithSuggestions(
					"Run: circleci auth login",
					"Or set the CIRCLE_TOKEN environment variable",
					"Or drop --org to validate against public orbs only",
				).
				WithExitCode(clierrors.ExitAuthError)
		}
		return "", nil
	}
	return resolveOrgID(ctx, client, org, "circleci config validate")
}

// printValidationErrors writes each compilation error as a bulleted line to
// stderr. An error's message may itself span multiple newline-separated
// lines (the API sometimes returns one error with embedded detail lines
// rather than several separate errors), so each line gets its own bullet to
// keep the output visually aligned.
func printValidationErrors(ctx context.Context, errs []string) {
	for _, e := range errs {
		for _, line := range strings.Split(e, "\n") {
			iostream.ErrPrintf(ctx, "  • %s\n", line)
		}
	}
}

func configAPIErr(err error) *clierrors.CLIError {
	return cmdutil.APIErr(err, "", "config.api_error", "Config API request failed")
}
