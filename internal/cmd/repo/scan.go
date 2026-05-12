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

package repo

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/reposcan"
)

const scanFailedCode = "repo.scan_failed"

func newScanCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Detect the language stack and setup commands for this repo",
		Long: heredoc.Doc(`
			Scan the current working directory and report the detected language
			stack, recommended container image, and the install/test commands
			that would run on CircleCI.

			Detection runs entirely on the local filesystem. Image versions are
			resolved against Docker Hub, so the command requires network access.

			JSON fields:
			  stack          string  detected stack (e.g. "go", "python"); "unknown" if none
			  image          string  CircleCI Docker image (e.g. "cimg/go")
			  image_version  string  detected image tag
			  setup          array   ordered setup steps
			  setup[].name   string  step name ("system", "install", "test")
			  setup[].command string shell command to execute
		`),
		Example: heredoc.Doc(`
			# Scan the current directory
			$ circleci repo scan

			# Emit structured output for scripting
			$ circleci repo scan --json

			# Pull out just the detected stack
			$ circleci repo scan --json --jq '.stack'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			cwd, err := os.Getwd()
			if err != nil {
				return clierrors.New("repo.cwd_failed",
					"Could not read current working directory", err.Error()).
					WithExitCode(clierrors.ExitGeneralError)
			}
			return runScan(ctx, reposcan.NewDefaultScanner(), cwd, jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runScan(ctx context.Context, scanner reposcan.Scanner, dir string, jsonOut bool) error {
	sp := iostream.Spinner(ctx, !jsonOut, "Scanning repository...")
	result, err := scanner.Scan(ctx, dir)
	sp.Stop()

	if err != nil {
		return clierrors.New(scanFailedCode, "Repo scan failed", err.Error()).
			WithSuggestions("Re-run with --debug for details.").
			WithExitCode(clierrors.ExitGeneralError)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, result)
	}

	reposcan.Render(ctx, result)
	return nil
}
