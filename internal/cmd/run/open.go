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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/browser"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOpenCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
	)

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the current project's runs page in the browser",
		Long: heredoc.Doc(`
			Open the CircleCI runs page for the current project in your
			default web browser.

			The project is inferred from the current git repository's remote.
			Supports GitHub, Bitbucket, and GitLab remotes.

			Use --current-branch or --branch/-b to filter runs to a specific branch.
		`),
		Example: heredoc.Doc(`
			# Open runs for the current repo
			$ circleci run open

			# Open runs filtered to the current git branch
			$ circleci run open --current-branch

			# Open runs filtered to a specific branch
			$ circleci run open --branch my-feature

			# Open runs filtered to a specific branch (short flag)
			$ circleci run open -b main

			# Open when your remote is on CircleCI server
			$ circleci run open --host https://circleci.example.com
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOpen(ctx, client, args, projectSlug, branch)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used for latest-run lookup")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")

	return cmd
}

func runOpen(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string) error {

	var (
		id  uuid.UUID
		err error
	)

	if len(args) == 1 {
		id, err = uuid.Parse(args[0])
		if err != nil {
			return apiErr(err, args[0])
		}
	} else {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide a run UUID: circleci run get <uuid>")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		proj, err := client.GetProjectInfo(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}

		sp := iostream.Spinner(ctx, true, fmt.Sprintf("Fetching latest run for %s on branch %s", projectSlug, effectiveBranch))
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildRunFilter(effectiveBranch, ""),
			Limit:      1,
		})
		sp.Stop()
		if searchErr != nil {
			return apiErr(searchErr, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		if len(runs) == 0 {
			return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		id = runs[0].ID
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.RunURL(appURL, id)

	return browser.OpenURLOrPrint(iostream.Err(ctx), u)
}
