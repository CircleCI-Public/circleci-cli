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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	"github.com/CircleCI-Public/circleci-cli/internal/configprompt"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

type generatePromptJSON struct {
	Prompt        string `json:"prompt"`
	PromptVersion string `json:"prompt_version"`
}

func newGenerateCmd() *cobra.Command {
	var template, jsonOut bool
	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Print a prompt for your coding agent to write .circleci/config.yml",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is optional and is the directory to scan. Defaults to
				the current directory.
			`, "`"),
		},
		Long: heredoc.Docf(`
			Detect the repository's stack and print a prompt for a coding agent to
			write %[1]s<path>/.circleci/config.yml%[1]s, or improve it if it exists. Nothing
			is written; %[1]s--template%[1]s writes a starter config from a local template
			instead.

			JSON fields: prompt, prompt_version.
		`, "`"),
		Example: heredoc.Doc(`
			# Print a prompt for the current directory
			$ circleci config generate

			# Print a prompt for a specific project path
			$ circleci config generate ./my-app

			# Get the prompt as JSON
			$ circleci config generate --json

			# Write a starter config from the local template
			$ circleci config generate --template
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if template {
				return runGenerateTemplate(cmd.Context(), dir)
			}
			return runGeneratePrompt(cmd.Context(), dir, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&template, "template", false, "write a starter config from a local template instead of printing a prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmd.MarkFlagsMutuallyExclusive("template", "json")
	return cmd
}

func checkGenerateDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil && info.IsDir() {
		return nil
	}
	return clierrors.New(
		"config.path_not_found",
		"Path not found",
		fmt.Sprintf("No directory exists at %q.", dir),
	).WithSuggestions(
		"Check the path you passed and try again",
		"Omit the argument to scan the current directory",
	).WithExitCode(clierrors.ExitBadArguments)
}

func scanFailed(err error) error {
	return clierrors.New(
		"config.scan_failed",
		"Repository scan failed",
		fmt.Sprintf("Could not detect the project stack: %s.", err),
	).WithSuggestions(
		"Re-run with --debug to see scan details",
		"Try again; image resolution requires network access",
	).WithExitCode(clierrors.ExitGeneralError)
}

func runGeneratePrompt(ctx context.Context, dir string, jsonOut bool) error {
	if err := checkGenerateDir(dir); err != nil {
		return err
	}

	sp := iostream.Spinner(ctx, !jsonOut, "Scanning repository")
	result, err := reposcan.NewDefaultScanner().Scan(ctx, dir)
	sp.Stop()
	if err != nil {
		return scanFailed(err)
	}

	client := cmdutil.LoadClientOptionalAuth(ctx)
	sp = iostream.Spinner(ctx, !jsonOut, "Fetching the config prompt")
	prompt, err := client.ConfigPrompt(ctx, configprompt.NewRequest(dir, result))
	sp.Stop()
	if err != nil {
		return cmdutil.APIErr(err, "config prompt", "config.prompt_not_found", "The %s is not available on this CircleCI host.").
			WithSuggestions("Or write a starter config without the server: circleci config generate --template")
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, generatePromptJSON{Prompt: prompt.Prompt, PromptVersion: prompt.PromptVersion})
	}
	iostream.Print(ctx, prompt.Prompt)
	if iostream.IsTerminal(ctx) {
		iostream.ErrPrintf(ctx, "\n%s Paste this prompt into your coding agent, or rerun with --template to write a starter config.\n",
			iostream.SymbolOK(ctx))
	}
	return nil
}

func runGenerateTemplate(ctx context.Context, dir string) error {
	if err := checkGenerateDir(dir); err != nil {
		return err
	}

	// Skip-check before scan so a no-op re-run never makes a network call.
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		iostream.Printf(ctx, "%s Using existing config at %s\n",
			iostream.SymbolOK(ctx), configPath)
		return nil
	}

	result, err := reposcan.NewDefaultScanner().Scan(ctx, dir)
	if err != nil {
		return scanFailed(err)
	}

	if !result.IsEmpty() {
		reposcan.Render(ctx, result)
	}
	return configgen.Generate(ctx, dir, result)
}
