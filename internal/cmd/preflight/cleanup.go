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

package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/preflight"
)

type cleanupOptions struct {
	projectSlug    string
	remote         string
	id             string
	includeRunning bool
	dryRun         bool
	jsonOut        bool
}

func newCleanupCmd() *cobra.Command {
	var opts cleanupOptions

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete preflight branches left on the remote",
		Long: heredoc.Docf(`
			Delete the %[1]s%[2]s*%[1]s branches that preflights left behind — after
			--no-watch or --no-cleanup, or when a preflight was killed. Branches whose
			run is still going are skipped, since a preflight may still be watching them.

			JSON fields: preflight_id, branch, commit, run_id, status (deleted,
			would_delete, skipped_running, delete_failed), error
		`, "`", preflight.BranchPrefix),
		Example: heredoc.Doc(`
			# Delete every finished preflight branch
			$ circleci preflight cleanup

			# Show what would be deleted
			$ circleci preflight cleanup --dry-run

			# Delete one preflight's branch, even if its run is still going
			$ circleci preflight cleanup --id 4b1f8e2a-6c1d-4f7e-9a3b-2d5c8e1f0a4b --include-running
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanup(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.projectSlug, "project", "", "project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&opts.remote, "remote", preflight.DefaultRemote, "git remote to delete branches from")
	cmd.Flags().StringVar(&opts.id, "id", "", "only clean up the preflight with this ID")
	cmd.Flags().BoolVar(&opts.includeRunning, "include-running", false, "also delete branches whose run has not ended")
	cmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "n", false, "show what would be deleted without deleting")
	cmdutil.AddJSONFlag(cmd, &opts.jsonOut)

	return cmd
}

func runCleanup(ctx context.Context, opts cleanupOptions) error {
	var id uuid.UUID
	if opts.id != "" {
		var err error
		id, err = uuid.Parse(opts.id)
		if err != nil {
			return clierrors.New("preflight.invalid_id", "Invalid preflight ID",
				fmt.Sprintf("--id must be a preflight UUID (got: %q).", opts.id)).
				WithSuggestions("Copy the ID from the branch name: " + preflight.BranchPrefix + "<id>").
				WithExitCode(clierrors.ExitBadArguments)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return noRepoErr(err)
	}
	repo, err := preflight.OpenRepo(ctx, cwd, opts.remote)
	if err != nil {
		return noRepoErr(err)
	}
	slug, err := cmdutil.ResolveProjectSlug(opts.projectSlug)
	if err != nil {
		return err
	}
	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		return err
	}
	proj, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		return projectErr(err, slug)
	}

	entries, err := preflight.Cleanup(ctx, preflight.CleanupOptions{
		Client:         client,
		Repo:           repo,
		ProjectID:      proj.ID.String(),
		ID:             id,
		IncludeRunning: opts.includeRunning,
		DryRun:         opts.dryRun,
	})
	if err != nil {
		if gitE := (*preflight.GitError)(nil); errors.As(err, &gitE) {
			return gitErr(err, "Could not list preflight branches")
		}
		return projectErr(err, slug)
	}

	if opts.jsonOut {
		if err := iostream.PrintJSON(ctx, entries); err != nil {
			return err
		}
	} else {
		printCleanup(ctx, entries)
	}

	failed := 0
	for _, e := range entries {
		if e.Status == preflight.CleanupDeleteFailed {
			failed++
		}
	}
	switch {
	case failed > 0:
		return clierrors.New("preflight.cleanup_failed", "Cleanup incomplete",
			fmt.Sprintf("%d of %d preflight branch(es) could not be deleted.", failed, len(entries))).
			WithSuggestions("Check that you can push to the remote: git push --dry-run " + opts.remote + " HEAD").
			WithExitCode(clierrors.ExitGeneralError)
	case id != uuid.Nil && len(entries) == 0:
		return clierrors.New("preflight.not_found", "Preflight branch not found",
			fmt.Sprintf("No branch %s on %s.", preflight.BranchName(id), opts.remote)).
			WithExitCode(clierrors.ExitNotFound)
	}
	return nil
}

func printCleanup(ctx context.Context, entries []preflight.CleanupEntry) {
	if len(entries) == 0 {
		iostream.Printf(ctx, "No preflight branches to clean up.\n")
		return
	}
	for _, e := range entries {
		switch e.Status {
		case preflight.CleanupDeleted:
			iostream.Printf(ctx, "Deleted %s\n", e.Branch)
		case preflight.CleanupWouldDelete:
			iostream.Printf(ctx, "Would delete %s\n", e.Branch)
		case preflight.CleanupSkippedRun:
			iostream.Printf(ctx, "Skipped %s (run %s is still going; pass --include-running to delete)\n", e.Branch, e.RunID)
		case preflight.CleanupDeleteFailed:
			iostream.Printf(ctx, "Failed to delete %s: %s\n", e.Branch, e.Error)
		}
	}
}
