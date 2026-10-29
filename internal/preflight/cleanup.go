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
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// Cleanup statuses, as reported in CleanupEntry.Status.
const (
	CleanupDeleted      = "deleted"
	CleanupWouldDelete  = "would_delete"
	CleanupSkippedRun   = "skipped_running"
	CleanupSkippedStart = "skipped_starting"
	CleanupDeleteFailed = "delete_failed"
	CleanupLookupFailed = "lookup_failed"
)

// startGrace is how long after its commit a branch with no run is taken to
// belong to a preflight still waiting for that run to start: the wait, plus
// slack for the snapshot and push before it.
const startGrace = DefaultRunWait + 3*time.Minute

// CleanupOptions configures Cleanup.
type CleanupOptions struct {
	Client    *apiclient.Client
	Repo      *Repo
	ProjectID string
	// ID limits the cleanup to one preflight. Zero means every preflight
	// branch on the remote.
	ID uuid.UUID
	// IncludeRunning also deletes branches whose run has not ended, or has
	// not started within startGrace of the commit. By default those are
	// skipped: the preflight that pushed them may still be watching or waiting.
	IncludeRunning bool
	DryRun         bool
	// Confirm is called with the branches about to be deleted, once they are
	// known and before any is deleted; an error stops the cleanup with nothing
	// deleted. It is not called for a dry run, or when nothing is to be
	// deleted. Nil means no confirmation.
	Confirm func(toDelete []CleanupEntry) error
}

// CleanupEntry is the outcome for one preflight branch.
type CleanupEntry struct {
	ID     uuid.UUID  `json:"preflight_id"`
	Branch string     `json:"branch"`
	Commit string     `json:"commit"`
	RunID  *uuid.UUID `json:"run_id,omitempty"`
	Status string     `json:"status"`
	Error  string     `json:"error,omitempty"`
}

// Cleanup deletes preflight branches left on the remote — by a preflight run
// with --no-cleanup or --no-watch, or one that was killed before it could clean
// up. It returns one entry per branch considered; a branch that could not be
// deleted, or whose run could not be looked up, is reported in its entry rather
// than stopping the rest. Without IncludeRunning, a branch whose run is unknown
// is kept: it may belong to a preflight that is still watching. So is one with
// no run yet whose commit is recent, as a preflight waiting for its run to
// start would have. Every branch is
// looked up before any is deleted, so Confirm sees the whole set.
func Cleanup(ctx context.Context, opts CleanupOptions) ([]CleanupEntry, error) {
	branches, err := opts.Repo.ListBranches(ctx)
	if err != nil {
		return nil, err
	}

	entries := []CleanupEntry{}
	for _, b := range branches {
		if opts.ID != uuid.Nil && b.ID != opts.ID {
			continue
		}
		e := CleanupEntry{ID: b.ID, Branch: b.Name, Commit: b.Commit}

		r, err := latestRun(ctx, opts.Client, opts.ProjectID, b.Name)
		if err != nil && !opts.IncludeRunning {
			e.Status = CleanupLookupFailed
			e.Error = err.Error()
			entries = append(entries, e)
			continue
		}
		if r != nil {
			e.RunID = &r.ID
		}

		switch {
		case opts.IncludeRunning:
			e.Status = CleanupWouldDelete
		case r != nil && r.Phase != apiclient.PhaseEnded:
			e.Status = CleanupSkippedRun
		case r == nil && awaitingRun(opts.Repo, b.Commit):
			e.Status = CleanupSkippedStart
		default:
			e.Status = CleanupWouldDelete
		}
		entries = append(entries, e)
	}
	if opts.DryRun {
		return entries, nil
	}

	var toDelete []CleanupEntry
	for _, e := range entries {
		if e.Status == CleanupWouldDelete {
			toDelete = append(toDelete, e)
		}
	}
	if len(toDelete) > 0 && opts.Confirm != nil {
		if err := opts.Confirm(toDelete); err != nil {
			return nil, err
		}
	}

	for i, e := range entries {
		if e.Status != CleanupWouldDelete {
			continue
		}
		if err := opts.Repo.DeleteBranch(ctx, e.Branch); err != nil {
			entries[i].Status = CleanupDeleteFailed
			entries[i].Error = err.Error()
		} else {
			entries[i].Status = CleanupDeleted
		}
	}
	return entries, nil
}

// latestRun returns the most recent run on branch, or nil when it has none.
// awaitingRun reports whether a branch with no run may belong to a preflight
// still waiting for that run to start. A commit made in another clone cannot
// be dated, and is not.
func awaitingRun(repo *Repo, sha string) bool {
	when, ok := repo.CommitTime(sha)
	return ok && time.Since(when) < startGrace
}

func latestRun(ctx context.Context, client *apiclient.Client, projectID, branch string) (*apiclient.RunV3, error) {
	now := time.Now().UTC()
	runs, err := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
		ProjectIDs: []string{projectID},
		From:       now.AddDate(0, 0, -90),
		To:         now.Add(time.Hour),
		Filter:     apiclient.BuildRunFilter(branch, ""),
		Limit:      1,
	})
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}
