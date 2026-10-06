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
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/run"
)

// cleanupTimeout bounds the work done on the way out — cancelling the run and
// deleting the branch — which runs even after the context was cancelled.
const cleanupTimeout = 30 * time.Second

// ErrNoRun is returned when no run appeared for the pushed branch in time. The
// branch was pushed, so the project's triggers did not fire for it.
type ErrNoRun struct {
	Branch string
	Waited time.Duration
}

func (e *ErrNoRun) Error() string {
	return fmt.Sprintf("no run started for branch %s within %s", e.Branch, e.Waited)
}

// ErrTimedOut is returned when the run did not end within Options.Timeout.
var ErrTimedOut = errTimedOut

// Options configures Run.
type Options struct {
	Client      *apiclient.Client
	Repo        *Repo
	ProjectSlug string
	// ID names the preflight and its branch. Zero means generate one.
	ID uuid.UUID
	// Watch follows the run after it starts. Without it, Run returns once the
	// run exists and the branch is left in place.
	Watch bool
	// FailFast stops at the first failed job instead of waiting for the run.
	FailFast bool
	// Cleanup cancels a run the preflight stopped watching early and deletes
	// the branch.
	Cleanup  bool
	Interval time.Duration
	Timeout  time.Duration
	// RunWait is how long to wait for the push to start a run.
	RunWait  time.Duration
	RunURL   func(runID uuid.UUID) string
	JobURL   func(workflowID, jobID uuid.UUID) string
	Observer Observer
}

// Run snapshots the working tree (see Repo.Snapshot), pushes it to a preflight branch, waits for
// the push to start a run and, with Options.Watch, follows that run to the end
// (or to the first failure), then cleans up.
//
// The returned summary is nil only when no run was watched. A run that failed
// is not an error: the summary's Outcome says how it ended, and the caller
// decides the exit status. The error reports what stopped the preflight itself —
// a git or API failure, ErrNoRun, ErrTimedOut, or the context being cancelled.
func Run(ctx context.Context, opts Options) (*RunSummaryEvent, error) {
	if opts.ID == uuid.Nil {
		opts.ID = uuid.New()
	}
	branch := BranchName(opts.ID)

	proj, err := opts.Client.GetProjectBySlug(ctx, opts.ProjectSlug)
	if err != nil {
		return nil, err
	}

	snap, err := opts.Repo.Snapshot(ctx, "preflight "+opts.ID.String())
	if err != nil {
		return nil, err
	}
	pushedAt := time.Now()
	if err := opts.Repo.Push(ctx, snap.Commit, branch); err != nil {
		return nil, err
	}
	if err := opts.Observer.BranchPushed(BranchPushedEvent{
		Type: EventBranchPushed, ID: opts.ID, Branch: branch, Commit: snap.Commit,
		Base: snap.Base, BaseBranch: snap.BaseBranch, Changes: snap.Changed,
	}); err != nil {
		return nil, err
	}

	r, err := waitForRun(ctx, opts, proj.ID.String(), branch, pushedAt)
	if err != nil {
		if opts.Cleanup {
			_ = deleteBranch(ctx, opts.Repo, branch)
		}
		return nil, err
	}
	if err := opts.Observer.RunStarted(RunStartedEvent{
		Type: EventRunStarted, RunID: r.ID, Project: opts.ProjectSlug,
		Branch: branch, Commit: snap.Commit, URL: opts.RunURL(r.ID),
	}); err != nil {
		return nil, err
	}
	if !opts.Watch {
		return nil, nil
	}

	res, watchErr := watch(ctx, watchOptions{
		client:   opts.Client,
		runID:    r.ID,
		interval: opts.Interval,
		timeout:  opts.Timeout,
		failFast: opts.FailFast,
		jobURL:   opts.JobURL,
		observer: opts.Observer,
	})
	if errors.Is(watchErr, context.Canceled) {
		res.outcome = OutcomeInterrupted
	}
	if watchErr != nil && res.outcome == "" {
		// An API or observer failure mid-watch: nothing to summarise, but the
		// branch is still cleaned up.
		if opts.Cleanup {
			_ = deleteBranch(ctx, opts.Repo, branch)
		}
		return nil, watchErr
	}

	summary := RunSummaryEvent{
		Type:       EventRunSummary,
		RunID:      r.ID,
		Outcome:    res.outcome,
		Ended:      res.ended,
		DurationMS: res.elapsed.Milliseconds(),
		URL:        opts.RunURL(r.ID),
		Errors:     res.errors,
		FailedJobs: res.failed,
	}
	if summary.FailedJobs == nil {
		summary.FailedJobs = []JobRef{}
	}

	if opts.Cleanup {
		if !res.ended {
			summary.RunCancelled = cancelRun(ctx, opts.Client, r.ID)
		}
		if err := deleteBranch(ctx, opts.Repo, branch); err != nil {
			summary.CleanupError = err.Error()
		} else {
			summary.BranchDeleted = true
		}
	}

	if err := opts.Observer.RunSummary(summary); err != nil {
		return &summary, err
	}
	return &summary, watchErr
}

// waitForRun polls for the run the push started, identified by the branch:
// a preflight branch is unique, so the first run on it is this preflight's.
func waitForRun(ctx context.Context, opts Options, projectID, branch string, pushedAt time.Time) (*apiclient.RunV3, error) {
	deadline := time.Now().Add(opts.RunWait)
	notified := false
	for {
		runs, err := opts.Client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{projectID},
			// Pad the window for clock skew between this machine and CircleCI.
			From:   pushedAt.Add(-time.Hour).UTC(),
			To:     time.Now().Add(time.Hour).UTC(),
			Filter: apiclient.BuildRunFilter(branch, ""),
			Limit:  1,
		})
		if err != nil {
			return nil, err
		}
		if len(runs) > 0 {
			return &runs[0], nil
		}
		if time.Now().After(deadline) {
			return nil, &ErrNoRun{Branch: branch, Waited: opts.RunWait}
		}
		if !notified {
			opts.Observer.WaitingForRun(branch)
			notified = true
		}
		if err := sleep(ctx, opts.Interval); err != nil {
			return nil, err
		}
	}
}

// cancelRun cancels what is left of a run the preflight stopped watching, and
// reports whether anything was cancelled. Its branch is about to be deleted, so
// any job still to start would fail to check out anyway.
func cancelRun(ctx context.Context, client *apiclient.Client, runID uuid.UUID) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	return run.Cancel(ctx, client, runID) == nil
}

// deleteBranch deletes the preflight branch, even when ctx has been cancelled:
// a Ctrl+C is exactly when the branch most needs cleaning up.
func deleteBranch(ctx context.Context, repo *Repo, branch string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	return repo.DeleteBranch(ctx, branch)
}
