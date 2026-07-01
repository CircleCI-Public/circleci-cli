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

// Package test implements the "circleci test" command group.
package test

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// NewTestCmd returns the "circleci test" command group.
func NewTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "test <command>",
		GroupID: "ci",
		Short:   "Inspect test results for a job",
		Long: heredoc.Doc(`
			Work with the test metadata CircleCI collects for a job.

			When a job stores test results (via the 'store_test_results' step),
			CircleCI parses them into per-test records. Use these commands to
			review which tests failed, passed or were skipped without opening the
			web UI.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "General commands",
		newListCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newGetCmd(),
	)

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"test.not_found", "No test results found for job %q.",
		"Check the job ID with: circleci job get")
}
