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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

// NewRunCmd returns the "circleci run" command group.
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run <command>",
		GroupID: "ci",
		Short:   "Trigger, watch, approve and cancel pipeline runs",
		Long: heredoc.Doc(`
			Work with CircleCI runs.

			A run is one execution of a pipeline, created each time a trigger
			fires. It carries the VCS context for that firing and groups the
			workflows it produced; each workflow in turn contains jobs.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newListCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newApproveCmd(),
		newCancelCmd(),
		newCompareCmd(),
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

// resolveRunRef turns a <run-number-or-id> argument into a run UUID and the
// name to show for it: "#<number>" for a run number, the UUID as given
// otherwise. A run number is looked up in projectSlug, or in the project of the
// git remote when projectSlug is empty. verb is the subcommand, used in the
// suggestion shown when the git remote can't be detected.
func resolveRunRef(ctx context.Context, client *apiclient.Client, verb, arg, projectSlug string) (uuid.UUID, string, error) {
	if !looksLikeNumber(arg) {
		id, err := uuid.Parse(arg)
		if err != nil {
			return uuid.Nil, "", apiErr(err, arg)
		}
		return id, arg, nil
	}

	number, _ := strconv.ParseInt(arg, 10, 64)
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return uuid.Nil, "", cmdutil.GitDetectErr(err, "Or specify the project: circleci run "+verb+" "+arg+" --project gh/org/repo")
		}
		projectSlug = info.Slug
	}
	r, err := client.GetPipelineByNumber(ctx, projectSlug, number)
	if err != nil {
		return uuid.Nil, "", apiErr(err, fmt.Sprintf("%s #%s", projectSlug, arg))
	}
	return r.ID, fmt.Sprintf("#%d", r.Number), nil
}

// looksLikeNumber returns true if s is a plain positive integer (run number),
// as opposed to a UUID (which contains hyphens).
func looksLikeNumber(s string) bool {
	return !strings.Contains(s, "-") && len(s) > 0
}
