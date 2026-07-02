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
	"net/http"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

func newLinkCmd() *cobra.Command {
	var (
		projectSlug string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Bind this checkout to a CircleCI project",
		Long: heredoc.Doc(`
			Record the CircleCI project for the current checkout in
			.circleci/info.yml so other commands can resolve it without
			re-detecting from the git remote each time.

			Resolution order:
			  1. --project slug, if given.
			  2. The git remote (origin) of the current directory.
			  3. An interactive prompt — used when neither of the above
			     yields a project the API recognises.

			If you are not authenticated, this command exits with an
			authentication error rather than prompting blindly: the
			lookup against the CircleCI API is what verifies the slug.

			An existing .circleci/info.yml is preserved unless --force
			is passed.
		`),
		Example: heredoc.Doc(`
			# Auto-detect from the current git repository
			$ circleci project link

			# Bind to a specific project (skips lookup if it exists)
			$ circleci project link --project gh/myorg/myrepo

			# Bind to a standalone project by its CircleCI slug
			$ circleci project link --project circleci/<orgID>/<projectID>

			# Overwrite an existing .circleci/info.yml
			$ circleci project link --force
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runProjectLink(ctx, projectSlug, force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo or circleci/orgID/projectID)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite an existing .circleci/info.yml")

	return cmd
}

func runProjectLink(ctx context.Context, projectSlug string, force bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return clierrors.New("project.link.cwd_failed", "Could not determine working directory", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if !force {
		if _, err := os.Stat(projectref.Path(cwd)); err == nil {
			return clierrors.New("project.link.exists", "Project already linked",
				fmt.Sprintf("%s already exists.", projectref.FilePath)).
				WithSuggestions("Re-run with --force to overwrite").
				WithExitCode(clierrors.ExitGeneralError)
		}
	}

	// Step 1: candidate slug from --project, then git remote.
	// Note: we deliberately do NOT consult an existing .circleci/info.yml here
	// — `project link` is what writes that file, so reading it first would
	// turn re-running link into a no-op against the existing entry.
	slug := strings.TrimSpace(projectSlug)
	if slug == "" {
		if info, gerr := gitremote.DetectFromRemote(); gerr == nil {
			slug = info.Slug
		}
	}

	// Step 2: load the API client. If the user has no token at all we cannot
	// verify the slug — LoadClient returns a structured "log in" error in that
	// case, which we propagate directly.
	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		return err
	}

	// Step 3: try to fetch project info for the candidate slug.
	var info *apiclient.ProjectInfo
	if slug != "" {
		pi, perr := client.GetProjectInfo(ctx, slug)
		switch {
		case perr == nil:
			info = pi
		case httpcl.HasStatusCode(perr, http.StatusUnauthorized):
			return clierrors.New("auth.token_invalid", "Authentication failed",
				"The API token was rejected by CircleCI.").
				WithSuggestions("Run: circleci auth login").
				WithRef("https://app.circleci.com/settings/user/tokens").
				WithExitCode(clierrors.ExitAuthError)
		case httpcl.HasStatusCode(perr, http.StatusNotFound):
			// Fall through to manual prompt below.
		default:
			return cmdutil.APIErr(perr, slug, "project.not_found", "Could not look up project %q.")
		}
	}

	// Step 4: if we still don't have project info, prompt for the slug.
	// Interactive sessions get a bubbletea text-input; non-interactive callers
	// (CI, agents driving the CLI without a TTY) get a structured error that
	// tells them exactly which flag to pass instead.
	if info == nil {
		if !iostream.IsInteractive(ctx) {
			return clierrors.New("project.link.cannot_resolve", "Could not resolve project",
				"No project found via --project flag or git remote, and this session is not interactive.").
				WithSuggestions(
					"Re-run with --project <slug>, e.g. circleci project link --project gh/myorg/myrepo",
					"For a standalone project, use the slug circleci/<orgID>/<projectID> from its CircleCI Project Settings page",
					"Or run from inside a git checkout whose origin is connected to CircleCI",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		entered, info2, perr := promptAndLookup(ctx, client)
		if perr != nil {
			return perr
		}
		slug = entered
		info = info2
	}

	// Step 5: write .circleci/info.yml.
	out := &projectref.Info{
		Project: projectref.Project{Slug: slug},
	}
	if info != nil {
		out.Project.ID = info.ID
		out.Project.Name = info.Name
		out.Organization.ID = info.OrganizationID
		out.Organization.Name = info.OrganizationName
	}
	if err := projectref.Write(cwd, out); err != nil {
		return clierrors.New("project.link.write_failed", "Could not write .circleci/info.yml",
			err.Error()).WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Printf(ctx, "%s Linked %s → %s\n", iostream.SymbolOK(ctx), projectref.FilePath, slug)
	return nil
}

// promptAndLookup asks the user for a project identifier — either a slug like
// "circleci/<orgID>/<projectID>" or a bare project UUID — validates it against
// the API, and returns the canonical slug + info. The /project/{slug} V2
// endpoint accepts both forms, so a single prompt covers both cases. The
// function loops on lookup failure until the user enters a value the API
// recognises, cancels (esc / ctrl+c / empty input), or an unrecoverable
// error occurs.
//
// Callers must guard this function with iostream.IsInteractive(ctx) — running
// a bubbletea program without a TTY produces an unhelpful error and would
// hang an agent that pipes the CLI's stdin.
func promptAndLookup(ctx context.Context, client *apiclient.Client) (string, *apiclient.ProjectInfo, error) {
	iostream.ErrPrintln(ctx,
		"Could not resolve a CircleCI project for this directory.")
	iostream.ErrPrintln(ctx,
		"Enter a project slug (e.g. circleci/<orgID>/<projectID>) or project UUID (e.g. dcb570ed-b01e-4dd1-ac60-95fcc0f16872).")

	for {
		entered, err := iostream.PromptText(ctx,
			"Project slug or UUID",
			"circleci/<orgID>/<projectID> or dcb570ed-b01e-4dd1-ac60-95fcc0f16872")
		if err != nil {
			return "", nil, clierrors.New("project.link.prompt_failed", "Could not read project identifier",
				err.Error()).WithExitCode(clierrors.ExitGeneralError)
		}
		entered = strings.TrimSpace(entered)
		if entered == "" {
			return "", nil, clierrors.New("project.link.cancelled", "Link cancelled",
				"No project identifier entered.").WithExitCode(clierrors.ExitCancelled)
		}

		info, perr := client.GetProjectInfo(ctx, entered)
		switch {
		case perr == nil:
			return info.Slug, info, nil
		case httpcl.HasStatusCode(perr, http.StatusUnauthorized):
			return "", nil, clierrors.New("auth.token_invalid", "Authentication failed",
				"The API token was rejected by CircleCI.").
				WithSuggestions("Run: circleci auth login").
				WithExitCode(clierrors.ExitAuthError)
		case httpcl.HasStatusCode(perr, http.StatusNotFound):
			iostream.ErrPrintf(ctx, "%s No project found for %q. Try again.\n", iostream.SymbolWarn(ctx), entered)
			continue
		default:
			return "", nil, cmdutil.APIErr(perr, entered, "project.not_found", "Could not look up project %q.")
		}
	}
}
