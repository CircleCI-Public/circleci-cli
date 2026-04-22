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

// Package runner implements the "circleci runner" command group.
package runner

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// NewRunnerCmd returns the "circleci runner" command group.
func NewRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runner <command>",
		Short: "Manage self-hosted runners",
		Long: heredoc.Doc(`
			Manage CircleCI self-hosted runner resources.

			Self-hosted runners let you run CircleCI jobs on your own infrastructure.
			Use these commands to manage resource classes, authentication tokens, and
			view connected runner instances.

			Resource class names use the format: namespace/name
			(e.g. my-org/my-runner)
		`),
	}

	cmd.AddCommand(newResourceClassCmd())
	cmd.AddCommand(newTokenCmd())
	cmd.AddCommand(newInstanceCmd())
	cmd.AddCommand(newTasksCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"runner.not_found", "No runner resource found for %q.",
		"List available resource classes with: circleci runner resource-class list")
}

func runnerNotEnabledErr() *clierrors.CLIError {
	return clierrors.New("runner.not_enabled", "Runner not available",
		"Self-hosted runners are not available for this token or account. The API returned 404.").
		WithSuggestions(
			"Confirm your token has runner permissions",
			"Check that your plan includes self-hosted runners: https://app.circleci.com/settings/plan",
		).
		WithRef("https://circleci.com/docs/runner-overview/").
		WithExitCode(clierrors.ExitAPIError)
}
