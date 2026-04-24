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

package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newFollowCmd() *cobra.Command {
	var projectSlug string

	cmd := &cobra.Command{
		Use:   "follow",
		Short: "Follow a project",
		Long: heredoc.Doc(`
			Follow a CircleCI project to enable builds and receive status updates.

			The project is inferred from the current git repository's remote
			unless overridden with --project. The slug must be in the form
			vcs/org/repo (e.g. gh/myorg/myrepo).

			Following a project that is already followed is a no-op.
		`),
		Example: heredoc.Doc(`
			# Follow the project for the current git repository
			$ circleci project follow

			# Follow a specific project
			$ circleci project follow --project gh/myorg/myrepo

			# Follow a Bitbucket project
			$ circleci project follow --project bb/myorg/myrepo
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return runProjectFollow(ctx, client, projectSlug)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")

	return cmd
}

func runProjectFollow(ctx context.Context, client *apiclient.Client, projectSlug string) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify the project: circleci project follow --project gh/org/repo",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}

	vcsType, org, repo, err := parseSlug(projectSlug)
	if err != nil {
		return clierrors.New("args.invalid_slug", "Invalid project slug",
			fmt.Sprintf("%q is not a valid project slug.", projectSlug)).
			WithSuggestions("Use the form: vcs/org/repo (e.g. gh/myorg/myrepo)").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if apiErr := client.FollowProject(ctx, vcsType, org, repo); apiErr != nil {
		return cmdutil.APIErr(apiErr, projectSlug, "project.follow_failed", "Could not follow project %q.")
	}

	iostream.Printf(ctx, "%s Now following %s\n", iostream.Symbol(ctx, "✓", "OK:"), projectSlug)
	return nil
}

// parseSlug splits "gh/org/repo" → ("gh", "org", "repo").
func parseSlug(slug string) (vcs, org, repo string, err error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid slug")
	}
	return parts[0], parts[1], parts[2], nil
}
