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

package cmdtest

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
	"github.com/CircleCI-Public/circleci-cli/internal/testrunner"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run detected project tests",
		Long: heredoc.Doc(`
			Detect the project's language stack and test command, then run that test
			command locally. If tests fail, a prompt is printed for use with an AI
			assistant before the command exits non-zero.
		`),
		Example: heredoc.Doc(`
			# Run tests for the current directory
			$ circleci test run

			# Run tests for a specific project path
			$ circleci test run ./my-app
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: runTest,
	}
	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	ctx := iostream.FromCmd(cmd.Context(), cmd)

	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return clierrors.New(
			"test.path_not_found",
			"Path not found",
			fmt.Sprintf("No directory exists at %q.", dir),
		).WithSuggestions(
			"Check the path you passed and try again",
			"Omit the argument to scan the current directory",
		).WithExitCode(clierrors.ExitBadArguments)
	}

	result, err := reposcan.NewDefaultScanner().Scan(ctx, dir)
	if err != nil {
		return clierrors.New(
			"test.scan_failed",
			"Repository scan failed",
			fmt.Sprintf("Could not detect the project stack: %s.", err),
		).WithSuggestions(
			"Re-run with --debug to see scan details",
			"Try again; image resolution requires network access",
		).WithExitCode(clierrors.ExitGeneralError)
	}

	if !result.IsEmpty() {
		reposcan.Render(ctx, result)
	}
	return testrunner.Run(ctx, dir, result)
}
