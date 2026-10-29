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
	force          bool
	jsonOut        bool
}

func newCleanupCmd() *cobra.Command {
	var opts cleanupOptions

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete preflight branches left on the remote",
		Annotations: map[string]string{
			"destructiveHint": "true",
		},
		Long: heredoc.Docf(`
			Delete the %[1]s%[2]s*%[1]s branches that preflights left behind — after
			--no-watch or --no-cleanup, or when a preflight was killed. A branch is
			skipped while its run is still going, or when it was pushed minutes ago and
			its run has not started: a preflight may still be watching or waiting.
			Asks before deleting; pass --force when not at a terminal.

			JSON fields: preflight_id, branch, commit, run_id, status (deleted,
			would_delete, skipped_running, skipped_starting, delete_failed,
			lookup_failed), error
		`, "`", preflight.BranchPrefix),
		Example: heredoc.Doc(`
			# Delete every finished preflight branch, after confirming
			$ circleci preflight cleanup

			# Show what would be deleted
			$ circleci preflight cleanup --dry-run

			# Delete one preflight's branch without confirming, even if its run is still going
			$ circleci preflight cleanup --id 4b1f8e2a-6c1d-4f7e-9a3b-2d5c8e1f0a4b --include-running --force
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
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "skip confirmation prompt")
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
	repo, err := preflight.OpenRepo(cwd, opts.remote)
	if err != nil {
		return noRepoErr(err)
	}
	defer func() { _ = repo.Close() }()
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
		Confirm: func(toDelete []preflight.CleanupEntry) error {
			return confirmCleanup(ctx, opts, toDelete)
		},
	})
	if err != nil {
		if cliErr := (*clierrors.CLIError)(nil); errors.As(err, &cliErr) {
			return err
		}
		if gitE := (*preflight.GitError)(nil); errors.As(err, &gitE) {
			return gitErr(err, "Could not list preflight branches", opts.remote)
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

	deleteFailed, lookupFailed := 0, 0
	for _, e := range entries {
		switch e.Status {
		case preflight.CleanupDeleteFailed:
			deleteFailed++
		case preflight.CleanupLookupFailed:
			lookupFailed++
		}
	}
	switch {
	case deleteFailed+lookupFailed > 0:
		var suggestions []string
		if deleteFailed > 0 {
			suggestions = append(suggestions, "Check that you can push to the remote: git push --dry-run "+opts.remote+" HEAD")
		}
		if lookupFailed > 0 {
			suggestions = append(suggestions,
				"Run the cleanup again: a branch whose run could not be looked up is kept, in case a preflight is still watching it",
				"Or delete those branches regardless with --include-running")
		}
		return clierrors.New("preflight.cleanup_failed", "Cleanup incomplete",
			fmt.Sprintf("%d of %d preflight branch(es) could not be cleaned up.", deleteFailed+lookupFailed, len(entries))).
			WithSuggestions(suggestions...).
			WithExitCode(clierrors.ExitGeneralError)
	case id != uuid.Nil && len(entries) == 0:
		return clierrors.New("preflight.not_found", "Preflight branch not found",
			fmt.Sprintf("No branch %s on %s.", preflight.BranchName(id), opts.remote)).
			WithExitCode(clierrors.ExitNotFound)
	}
	return nil
}

// confirmCleanup asks before toDelete is deleted from the remote, or requires
// --force when there is no terminal to ask at.
func confirmCleanup(ctx context.Context, opts cleanupOptions, toDelete []preflight.CleanupEntry) error {
	what := fmt.Sprintf("%d preflight branches", len(toDelete))
	if len(toDelete) == 1 {
		what = toDelete[0].Branch
	}
	return cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), opts.force,
		fmt.Sprintf("Delete %s from %s?", what, opts.remote),
		clierrors.New("preflight.cleanup_aborted", "Cleanup aborted",
			"Deleting the preflight branches was not confirmed; nothing was deleted.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("preflight.cleanup_requires_force", "Cleanup requires --force",
			fmt.Sprintf("Cleanup would delete %s from %s; nothing was deleted.", what, opts.remote)).
			WithSuggestions("See what would be deleted: circleci preflight cleanup --dry-run").
			WithExitCode(clierrors.ExitBadArguments),
	)
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
		case preflight.CleanupSkippedStart:
			iostream.Printf(ctx, "Skipped %s (its run has not started yet; pass --include-running to delete)\n", e.Branch)
		case preflight.CleanupDeleteFailed:
			iostream.Printf(ctx, "Failed to delete %s: %s\n", e.Branch, e.Error)
		case preflight.CleanupLookupFailed:
			iostream.Printf(ctx, "Kept %s (could not look up its run: %s)\n", e.Branch, e.Error)
		}
	}
}
