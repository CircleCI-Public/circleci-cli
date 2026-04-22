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

package pipeline

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// NewPipelineCmd returns the "circleci pipeline" command group.
func NewPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline <command>",
		Short: "Manage pipelines",
		Long: heredoc.Doc(`
			Work with CircleCI pipelines.

			Pipelines are the top-level unit of work in CircleCI — they contain
			one or more workflows, each of which contains jobs.
		`),
	}

	cmd.AddCommand(newCancelCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newTriggerCmd())
	cmd.AddCommand(newWatchCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"pipeline.not_found", "No pipeline found for %q.",
		"Check the pipeline UUID or branch name and try again")
}
