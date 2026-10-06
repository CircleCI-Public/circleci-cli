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
	CleanupDeleteFailed = "delete_failed"
)

// CleanupOptions configures Cleanup.
type CleanupOptions struct {
	Client    *apiclient.Client
	Repo      *Repo
	ProjectID string
	// ID limits the cleanup to one preflight. Zero means every preflight
	// branch on the remote.
	ID uuid.UUID
	// IncludeRunning also deletes branches whose run has not ended. By
	// default those are skipped: the preflight that pushed them may still be
	// watching.
	IncludeRunning bool
	DryRun         bool
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
// deleted is reported in its entry rather than stopping the rest.
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
		if err != nil {
			return nil, err
		}
		if r != nil {
			e.RunID = &r.ID
		}

		switch {
		case r != nil && r.Phase != apiclient.PhaseEnded && !opts.IncludeRunning:
			e.Status = CleanupSkippedRun
		case opts.DryRun:
			e.Status = CleanupWouldDelete
		default:
			if err := opts.Repo.DeleteBranch(ctx, b.Name); err != nil {
				e.Status = CleanupDeleteFailed
				e.Error = err.Error()
			} else {
				e.Status = CleanupDeleted
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// latestRun returns the most recent run on branch, or nil when it has none.
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
