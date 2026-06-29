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

package run

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// NewRunCmd returns the "circleci run" command group.
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run <command>",
		GroupID: "ci",
		Short:   "Trigger, watch and cancel CI runs",
		Long: heredoc.Doc(`
			Work with CircleCI runs.

			A run is created each time a trigger fires for a pipeline. It carries
			the VCS context for that firing and groups the workflows it produced;
			each workflow in turn contains jobs.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newListCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newCancelCmd(),
		newOpenCmd(),
		newGetCmd(),
		newTriggerCmd(),
		newWatchCmd(),
	)

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"run.not_found", "No run found for %q.",
		"Check the run UUID or branch name and try again")
}

// looksLikeNumber returns true if s is a plain positive integer (run number),
// as opposed to a UUID (which contains hyphens).
func looksLikeNumber(s string) bool {
	return !strings.Contains(s, "-") && len(s) > 0
}
