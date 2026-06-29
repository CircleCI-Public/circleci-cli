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

package dlc

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewPurgeCmd returns the "circleci dlc purge" command.
func NewPurgeCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "purge --project <slug>",
		Short: "Purge the Docker Layer Cache for a project",
		Long: heredoc.Doc(`
			Purge the docker layer cache (DLC) for a project.

			Docker layer caching stores individual Docker image layers between
			pipeline runs to speed up builds. Purging the cache forces CircleCI
			to rebuild all layers from scratch on the next run, which is useful
			when a cached layer contains stale or corrupt data.

			The project is inferred from the current git repository's remote
			unless overridden with --project. The slug must be in the form
			vcs/org/repo (e.g. gh/myorg/myrepo).

			JSON fields (--json): project_id, project_slug
		`),
		Example: heredoc.Doc(`
			# Purge DLC for the current git repository's project
			$ circleci dlc purge

			# Purge DLC for a specific project
			$ circleci dlc purge --project gh/myorg/myrepo

			# Purge DLC and output result as JSON
			$ circleci dlc purge --project gh/myorg/myrepo --json

			# Purge DLC for a Bitbucket project
			$ circleci dlc purge --project bb/myorg/myrepo
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if projectSlug == "" {
				info, err := gitremote.Detect()
				if err != nil {
					return cmdutil.GitDetectErr(err, "Or specify the project: circleci dlc purge --project gh/org/repo")
				}
				projectSlug = info.Slug
			}

			_, _, _, err := cmdutil.ParseSlug(projectSlug)
			if err != nil {
				return clierrors.New("args.invalid_slug", "Invalid project slug",
					fmt.Sprintf("%q is not a valid project slug.", projectSlug)).
					WithSuggestions("Use the form: vcs/org/repo (e.g. gh/myorg/myrepo)").
					WithExitCode(clierrors.ExitBadArguments)
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			proj, err := client.GetProjectBySlug(ctx, projectSlug)
			if err != nil {
				return cmdutil.APIErr(err, projectSlug, "project.not_found", "No project found for %q.",
					"Check the project slug and try again",
					"Use 'circleci project list' to see followed projects")
			}

			if err := client.PurgeDLC(ctx, proj.ID.String()); err != nil {
				if errors.Is(err, apiclient.ErrDLCGone) {
					return clierrors.New("dlc.gone", "DLC purge unavailable",
						"This DLC feature is no longer supported by this version of the CLI. Please upgrade.").
						WithSuggestions("Upgrade to the latest version of the circleci CLI").
						WithExitCode(clierrors.ExitAPIError)
				}
				return cmdutil.APIErr(err, projectSlug, "dlc.purge_failed", "Could not purge DLC for %q.")
			}

			type purgeOutput struct {
				ProjectID   string `json:"project_id"`
				ProjectSlug string `json:"project_slug"`
			}

			out := purgeOutput{
				ProjectID:   proj.ID.String(),
				ProjectSlug: projectSlug,
			}

			if jsonOut {
				return cmdutil.WriteJSON(iostream.Out(ctx), out)
			}

			iostream.Printf(ctx, "%s DLC purged for %s\n", iostream.SymbolOK(ctx), projectSlug)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}
