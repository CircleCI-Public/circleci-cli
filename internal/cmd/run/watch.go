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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// defaultSHAWaitDuration is the maximum time to wait for a run matching a
// given SHA to appear. CIRCLECI_SHA_WAIT_MS overrides this for testing.
const defaultSHAWaitDuration = 2 * time.Minute

func shaWaitDuration() time.Duration {
	if ms := os.Getenv("CIRCLECI_SHA_WAIT_MS"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	return defaultSHAWaitDuration
}

func newWatchCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		sha         string
		timeout     time.Duration
		failFast    bool
	)

	cmd := &cobra.Command{
		Use:   "watch [<run-number-or-id>]",
		Short: "Watch a run until it completes",
		Long: heredoc.Doc(`
			Monitor a CircleCI run and block until it reaches a terminal state.

			Exit code reflects the result:
			  0 = all workflows succeeded
			  1 = one or more workflows failed
			  6 = run was cancelled
			  8 = timed out before run completed

			Without arguments, watches the latest run for the current branch.
			Pass a run number or UUID to watch a specific run.

			With --sha, searches the run list for a run matching that
			commit. If not yet found, polls for up to 2 minutes — useful when run
			immediately after git push.

			With --failfast, exits as soon as any job is observed to have failed,
			without waiting for the remaining workflows to finish.
		`),
		Example: heredoc.Doc(`
			# Watch the latest run on the current branch
			$ circleci run watch

			# Push and watch in one step
			$ git push && circleci run watch --sha $(git rev-parse HEAD)

			# Watch a specific run number
			$ circleci run watch 75

			# Watch by UUID (e.g. from 'run list --json')
			$ circleci run watch 0b0e6eca-4e9a-43d7-b74e-a7ed4b7d11cd

			# Watch a run on a different branch
			$ circleci run watch --branch main

			# Watch with a longer timeout
			$ circleci run watch --timeout 60m

			# Exit as soon as any job fails
			$ circleci run watch --failfast
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runWatch(ctx, client, args, projectSlug, branch, sha, timeout, failFast)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to watch (defaults to current branch)")
	cmd.Flags().StringVar(&sha, "sha", "", "Watch run for this commit SHA; polls up to 2m if not yet created")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Maximum time to wait for run completion")
	cmd.Flags().BoolVar(&failFast, "failfast", false, "Exit as soon as any job fails, without waiting for the rest of the run")

	return cmd
}

func runWatch(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch, sha string, timeout time.Duration, failFast bool) error {
	// If the argument looks like a UUID, we can resolve the run directly
	// without needing a project slug or branch from git.
	isUUID := len(args) == 1 && strings.Contains(args[0], "-")

	// Resolve project and branch from git if not fully specified.
	needsGit := !isUUID && (projectSlug == "" || (branch == "" && sha == "" && len(args) == 0))
	if needsGit {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify --project and --branch explicitly")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if branch == "" {
			branch = info.Branch
		}
	}

	// Find the run to watch.
	var r *apiclient.Pipeline
	var err error

	switch {
	case isUUID:
		r, err = client.GetPipeline(ctx, args[0])
		if err != nil {
			return apiErr(err, args[0])
		}

	case len(args) == 1:
		number, _ := strconv.ParseInt(args[0], 10, 64)
		r, err = client.GetPipelineByNumber(ctx, projectSlug, number)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, args[0]))
		}

	case sha != "":
		r, err = waitForRunBySHA(ctx, client, projectSlug, branch, sha)
		if err != nil {
			return err
		}

	default:
		r, err = client.GetLatestPipeline(ctx, projectSlug, branch)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", projectSlug, branch))
		}
	}

	branch = r.ProjectSlug // use slug from the found run
	if r.VCS != nil && r.VCS.Branch != "" {
		branch = r.VCS.Branch
	}

	iostream.ErrPrintf(ctx, "Watching run #%d (%s @ %s)\n\n", r.Number, r.ProjectSlug, branch)

	return watchUntilDone(ctx, client, r.ID, r.Number, timeout, failFast)
}

// waitForRunBySHA searches for a run matching the given commit SHA,
// polling every 5 seconds for up to shaWaitDuration() if not immediately found.
func waitForRunBySHA(ctx context.Context, client *apiclient.Client, projectSlug, branch, sha string) (*apiclient.Pipeline, error) {
	waitDur := shaWaitDuration()
	deadline := time.Now().Add(waitDur)
	interval := 5 * time.Second
	printed := false

	for {
		runs, err := client.ListPipelines(ctx, projectSlug, branch, 10)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, watchInterrupted()
			}
			return nil, apiErr(err, projectSlug)
		}
		for i := range runs {
			r := &runs[i]
			if r.VCS != nil && strings.HasPrefix(r.VCS.Revision, sha) {
				return r, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, clierrors.New("run.sha_not_found", "Run not found",
				fmt.Sprintf("No run found for commit %s in %s after %s.", sha, projectSlug, waitDur)).
				WithSuggestions(
					"Verify the push triggered a run in CircleCI",
					"Check the SHA is correct: git rev-parse HEAD",
				).
				WithExitCode(clierrors.ExitNotFound)
		}

		if !printed {
			iostream.ErrPrintf(ctx, "Waiting for run for commit %s...\n", sha)
			printed = true
		}
		if err := sleepOrCancel(ctx, interval); err != nil {
			return nil, watchInterrupted()
		}
	}
}

// watchUntilDone polls the given run until all workflows reach a terminal
// state or the timeout elapses. With failFast, it returns as soon as any
// job is observed to have failed.
func watchUntilDone(ctx context.Context, client *apiclient.Client, runID string, runNumber int64, timeout time.Duration, failFast bool) error {
	deadline := time.Now().Add(timeout)
	start := time.Now()
	tty := iostream.IsTerminal(ctx)

	var prevLines int
	var prevFingerprint string
	pollInterval := 5 * time.Second

	for {
		state, err := fetchWatchState(ctx, client, runID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				if tty {
					iostream.ErrPrintf(ctx, "\n")
				}
				return watchInterrupted()
			}
			return clierrors.New("api.error", "API error while watching run", err.Error()).
				WithExitCode(clierrors.ExitAPIError)
		}

		elapsed := time.Since(start)
		fingerprint := watchFingerprint(state)
		changed := fingerprint != prevFingerprint

		if tty {
			// Erase previous table, redraw with updated elapsed.
			if prevLines > 0 {
				_, _ = fmt.Fprintf(iostream.Err(ctx), "\033[%dA\033[J", prevLines)
			}
			prevLines = printWatchTable(ctx, state, elapsed)
		} else if changed {
			printWatchLine(ctx, state, elapsed)
		}
		prevFingerprint = fingerprint

		if allWorkflowsDone(state.Workflows) {
			if tty && prevLines > 0 {
				// Final redraw without the elapsed ticker line.
				_, _ = fmt.Fprintf(iostream.Err(ctx), "\033[%dA\033[J", prevLines)
				printWatchTableFinal(ctx, state)
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchFinalResult(ctx, state, runNumber, elapsed)
		}

		if failFast && hasFailedJob(state) {
			if tty && prevLines > 0 {
				_, _ = fmt.Fprintf(iostream.Err(ctx), "\033[%dA\033[J", prevLines)
				printWatchTableFinal(ctx, state)
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchFailFastResult(ctx, state, runNumber, elapsed)
		}

		if time.Now().After(deadline) {
			iostream.ErrPrintf(ctx, "\n")
			return clierrors.New("run.timeout", "Watch timed out",
				fmt.Sprintf("Run #%d did not complete within %s.", runNumber, timeout)).
				WithExitCode(clierrors.ExitTimeout)
		}

		if err := sleepOrCancel(ctx, pollInterval); err != nil {
			if tty {
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchInterrupted()
		}
		// Ramp from 5s to 30s over the first few polls.
		if pollInterval < 30*time.Second {
			pollInterval += 5 * time.Second
		}
	}
}

// hasFailedJob reports whether any job in any workflow has reached a failed
// status. Used by --failfast to bail out without waiting for the rest of the
// run.
func hasFailedJob(state runGetOutput) bool {
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Status == "failed" {
				return true
			}
		}
	}
	return false
}

// sleepOrCancel waits for d to elapse, returning nil. If ctx is cancelled
// first (e.g. user pressed Ctrl-C), it returns ctx.Err() immediately.
func sleepOrCancel(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// watchInterrupted is the structured error returned when the user cancels
// the watch (Ctrl-C) before the run reaches a terminal state.
func watchInterrupted() *clierrors.CLIError {
	return clierrors.New("run.interrupted", "Watch interrupted",
		"Stopped watching before the run completed. The run is still active in CircleCI.").
		WithExitCode(clierrors.ExitCancelled)
}

// fetchWatchState retrieves the current run state including all workflows
// and their jobs, reusing buildOutput from get.go.
func fetchWatchState(ctx context.Context, client *apiclient.Client, runID string) (runGetOutput, error) {
	r, err := client.GetPipeline(ctx, runID)
	if err != nil {
		return runGetOutput{}, err
	}
	workflows, err := client.GetPipelineWorkflows(ctx, runID)
	if err != nil {
		return runGetOutput{}, err
	}
	wfJobs := make([][]apiclient.WorkflowJob, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return runGetOutput{}, err
		}
		wfJobs[i] = jobs
	}
	return buildOutput(r, workflows, wfJobs), nil
}

// allWorkflowsDone returns true when every workflow is in a terminal state.
// Returns false when there are no workflows yet (run still starting up).
func allWorkflowsDone(workflows []workflowOutput) bool {
	if len(workflows) == 0 {
		return false
	}
	terminal := map[string]bool{
		"success": true, "failed": true, "error": true,
		"canceled": true, "unauthorized": true, "not_run": true,
	}
	for _, wf := range workflows {
		if !terminal[wf.Status] {
			return false
		}
	}
	return true
}

// watchFingerprint returns a string that changes whenever workflow or job
// statuses change, used to detect updates for non-TTY output.
func watchFingerprint(state runGetOutput) string {
	var b strings.Builder
	for _, wf := range state.Workflows {
		b.WriteString(wf.Name)
		b.WriteByte('=')
		b.WriteString(wf.Status)
		b.WriteByte(';')
		for _, j := range wf.Jobs {
			b.WriteString(j.Name)
			b.WriteByte('=')
			b.WriteString(j.Status)
			b.WriteByte(';')
		}
	}
	return b.String()
}

// printWatchTable renders the workflow/job table to Err and returns the line count
// written (used for TTY cursor rewind).
func printWatchTable(ctx context.Context, state runGetOutput, elapsed time.Duration) int {
	lines := 0
	for _, wf := range state.Workflows {
		iostream.ErrPrintf(ctx, "  %-28s  %s\n", wf.Name, wf.Status)
		lines++
		for _, j := range wf.Jobs {
			if j.Number > 0 {
				iostream.ErrPrintf(ctx, "    %-30s  %-12s  #%d\n", j.Name, j.Status, j.Number)
			} else {
				iostream.ErrPrintf(ctx, "    %-30s  %s\n", j.Name, j.Status)
			}
			lines++
		}
	}
	iostream.ErrPrintf(ctx, "\n  Elapsed: %s\n", formatElapsed(elapsed))
	lines += 2
	return lines
}

// printWatchTableFinal renders the final workflow/job table without the elapsed line.
func printWatchTableFinal(ctx context.Context, state runGetOutput) {
	for _, wf := range state.Workflows {
		iostream.ErrPrintf(ctx, "  %-28s  %s\n", wf.Name, wf.Status)
		for _, j := range wf.Jobs {
			if j.Number > 0 {
				iostream.ErrPrintf(ctx, "    %-30s  %-12s  #%d\n", j.Name, j.Status, j.Number)
			} else {
				iostream.ErrPrintf(ctx, "    %-30s  %s\n", j.Name, j.Status)
			}
		}
	}
}

// printWatchLine emits a single-line status update for non-TTY output.
func printWatchLine(ctx context.Context, state runGetOutput, elapsed time.Duration) {
	parts := make([]string, 0, len(state.Workflows))
	for _, wf := range state.Workflows {
		parts = append(parts, fmt.Sprintf("%s=%s", wf.Name, wf.Status))
	}
	iostream.ErrPrintf(ctx, "[%s]  %s\n", formatElapsed(elapsed), strings.Join(parts, "  "))
}

// watchFailFastResult prints the early-exit message for --failfast and
// returns a structured failure error listing the failing job(s).
func watchFailFastResult(ctx context.Context, state runGetOutput, number int64, elapsed time.Duration) error {
	names := failedJobNames(state)
	iostream.ErrPrintf(ctx, "%s Run #%d has failing job(s): %s — exiting (%s)\n",
		iostream.SymbolFail(ctx), number, strings.Join(names, ", "), formatElapsed(elapsed))
	return clierrors.New("run.failed", "Run failed",
		fmt.Sprintf("Run #%d has %d failing job(s); exiting due to --failfast.", number, len(names))).
		WithSuggestions(failedJobLogSuggestions(state)...).
		WithExitCode(clierrors.ExitGeneralError)
}

// failedJobNames returns the names of all failed jobs across all workflows.
func failedJobNames(state runGetOutput) []string {
	var names []string
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Status == "failed" {
				names = append(names, j.Name)
			}
		}
	}
	return names
}

// watchFinalResult prints the outcome and returns an appropriate error (or nil).
func watchFinalResult(ctx context.Context, state runGetOutput, number int64, elapsed time.Duration) error {
	switch state.Status {
	case "success":
		iostream.ErrPrintf(ctx, "%s Run #%d succeeded (%s)\n",
			iostream.SymbolOK(ctx), number, formatElapsed(elapsed))
		return nil
	case "canceled":
		iostream.ErrPrintf(ctx, "Run #%d was cancelled (%s)\n", number, formatElapsed(elapsed))
		return clierrors.New("run.cancelled", "Run cancelled",
			fmt.Sprintf("Run #%d was cancelled.", number)).
			WithExitCode(clierrors.ExitCancelled)
	default:
		iostream.ErrPrintf(ctx, "%s Run #%d failed (%s)\n",
			iostream.SymbolFail(ctx), number, formatElapsed(elapsed))
		return clierrors.New("run.failed", "Run failed",
			fmt.Sprintf("Run #%d failed.", number)).
			WithSuggestions(failedJobLogSuggestions(state)...).
			WithExitCode(clierrors.ExitGeneralError)
	}
}

// failedJobLogSuggestions returns commands the user can run to view logs for
// the failed jobs in the run. Caps individual job suggestions so the
// suggestion list stays readable when many jobs fail.
func failedJobLogSuggestions(state runGetOutput) []string {
	const maxJobs = 3
	var suggestions []string
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Status != "failed" || j.Number <= 0 {
				continue
			}
			if len(suggestions) >= maxJobs {
				suggestions = append(suggestions,
					"More jobs failed; see all with: circleci run get")
				return append(suggestions,
					"Or fetch logs for the latest failed job: circleci logs --last-failed")
			}
			suggestions = append(suggestions,
				fmt.Sprintf("View logs for failed job %q: circleci logs %d", j.Name, j.Number))
		}
	}
	suggestions = append(suggestions,
		"Or fetch logs for the latest failed job: circleci logs --last-failed")
	return suggestions
}

func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
