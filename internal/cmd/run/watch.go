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
// given SHA to appear. CIRCLE_SHA_WAIT_MS overrides this for testing.
const defaultSHAWaitDuration = 2 * time.Minute

func shaWaitDuration() time.Duration {
	if ms := os.Getenv("CIRCLE_SHA_WAIT_MS"); ms != "" {
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
		Use:   "watch [<run-id>]",
		Short: "Watch a run until it completes",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<run-id> is optional and selects the run to watch. It can be:
				- a run UUID (shown in "circleci run list --json")
				- a run number (shown in "circleci run list"); the project is
				  inferred from the git remote unless overridden with --project

				When omitted, the latest run for the current branch is watched
				(override the branch with --branch, or match a commit with --sha).
			`),
		},
		Long: heredoc.Doc(`
			Monitor a CircleCI run and block until it reaches a terminal state.

			Exit code reflects the result:
			  0 = all workflows succeeded
			  1 = one or more workflows failed
			  6 = run was cancelled
			  8 = timed out before run completed

			Without arguments, watches the latest run for the current branch.
			Pass a run UUID to watch a specific run.

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
	isUUID := len(args) == 1 && strings.Contains(args[0], "-")

	needsGit := !isUUID && (projectSlug == "" || (branch == "" && sha == "" && len(args) == 0))
	if needsGit {
		info, err := gitremote.Detect()
		if err != nil {
			suggestion := "Or specify --project and --branch explicitly"
			if sha != "" {
				suggestion = "Or specify --project explicitly"
			}
			return cmdutil.GitDetectErr(err, suggestion)
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if branch == "" && sha == "" {
			branch = info.Branch
		}
	}

	var r *apiclient.RunV3
	var err error

	switch {
	case isUUID:
		r, err = client.GetRunV3(ctx, args[0])
		if err != nil {
			return apiErr(err, args[0])
		}

	case len(args) == 1:
		// Number lookup via V2, then resolve to V3.
		number, _ := strconv.ParseInt(args[0], 10, 64)
		p, pErr := client.GetPipelineByNumber(ctx, projectSlug, number)
		if pErr != nil {
			return apiErr(pErr, fmt.Sprintf("%s #%s", projectSlug, args[0]))
		}
		r, err = client.GetRunV3(ctx, p.ID)
		if err != nil {
			return apiErr(err, p.ID)
		}

	case sha != "":
		r, err = waitForRunBySHA(ctx, client, projectSlug, branch, sha)
		if err != nil {
			return err
		}

	default:
		proj, pErr := client.GetProjectInfo(ctx, projectSlug)
		if pErr != nil {
			return apiErr(pErr, projectSlug)
		}
		now := time.Now().UTC()
		runs, sErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildRunFilter(branch, ""),
			Limit:      1,
		})
		if sErr != nil {
			return apiErr(sErr, fmt.Sprintf("%s@%s", projectSlug, branch))
		}
		if len(runs) == 0 {
			return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, branch))
		}
		r = &runs[0]
	}

	displayBranch := r.Branch
	if displayBranch == "" {
		displayBranch = branch
	}

	iostream.ErrPrintf(ctx, "Watching run %s (%s)\n\n", r.ID, displayBranch)

	return watchUntilDone(ctx, client, r.ID, timeout, failFast)
}

// waitForRunBySHA searches for a run matching the given commit SHA via V3 search,
// polling every 5 seconds for up to shaWaitDuration() if not immediately found.
func waitForRunBySHA(ctx context.Context, client *apiclient.Client, projectSlug, branch, sha string) (*apiclient.RunV3, error) {
	proj, err := client.GetProjectInfo(ctx, projectSlug)
	if err != nil {
		return nil, apiErr(err, projectSlug)
	}

	waitDur := shaWaitDuration()
	deadline := time.Now().Add(waitDur)
	interval := 5 * time.Second
	printed := false

	expanded, expandErr := gitremote.ExpandSHA(sha)
	switch {
	case expandErr == nil:
		sha = expanded
	case errors.Is(expandErr, gitremote.ErrSHANotHex):
		return nil, clierrors.New("run.invalid_sha_format", "Invalid SHA format",
			fmt.Sprintf("%q does not look like a commit SHA; expected hex characters only.", sha)).
			WithSuggestions("Pass a hex commit SHA, e.g. from 'git log --oneline'").
			WithExitCode(clierrors.ExitBadArguments)
	case errors.Is(expandErr, gitremote.ErrSHARepoInaccessible):
		return nil, clierrors.New("run.sha_unresolvable", "Could not resolve short SHA",
			fmt.Sprintf("Cannot expand %q: local git repository is not accessible.", sha)).
			WithSuggestions("Pass the full 40-character SHA to skip local resolution").
			WithExitCode(clierrors.ExitBadArguments)
	case errors.Is(expandErr, gitremote.ErrSHANotFound):
		return nil, clierrors.New("run.invalid_sha", "Commit not found",
			fmt.Sprintf("Commit %q does not exist in the local repository.", sha)).
			WithSuggestions(
				"Check the SHA is correct: git log --oneline",
				"Pass the full 40-character SHA to skip local resolution",
			).
			WithExitCode(clierrors.ExitNotFound)
	}

	filter := fmt.Sprintf("pipeline.git.revision == %q", sha)
	if branch != "" {
		filter += fmt.Sprintf(" and pipeline.git.branch == %q", branch)
	}

	for {
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID},
			From:       now.AddDate(0, 0, -1),
			To:         now,
			Filter:     filter,
			Limit:      1,
		})
		if searchErr != nil {
			if errors.Is(searchErr, context.Canceled) {
				return nil, watchInterrupted()
			}
			return nil, apiErr(searchErr, projectSlug)
		}
		if len(runs) > 0 {
			return &runs[0], nil
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
// state or the timeout elapses.
func watchUntilDone(ctx context.Context, client *apiclient.Client, runID string, timeout time.Duration, failFast bool) error {
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
				_, _ = fmt.Fprintf(iostream.Err(ctx), "\033[%dA\033[J", prevLines)
				printWatchTableFinal(ctx, state)
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchFinalResult(ctx, state, runID, elapsed)
		}

		if failFast && hasFailedJob(state) {
			if tty && prevLines > 0 {
				_, _ = fmt.Fprintf(iostream.Err(ctx), "\033[%dA\033[J", prevLines)
				printWatchTableFinal(ctx, state)
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchFailFastResult(ctx, state, runID, elapsed)
		}

		if time.Now().After(deadline) {
			iostream.ErrPrintf(ctx, "\n")
			return clierrors.New("run.timeout", "Watch timed out",
				fmt.Sprintf("Run %s did not complete within %s.", runID, timeout)).
				WithExitCode(clierrors.ExitTimeout)
		}

		if err := sleepOrCancel(ctx, pollInterval); err != nil {
			if tty {
				iostream.ErrPrintf(ctx, "\n")
			}
			return watchInterrupted()
		}
		if pollInterval < 30*time.Second {
			pollInterval += 5 * time.Second
		}
	}
}

func hasFailedJob(state runGetOutput) bool {
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Outcome == "failed" {
				return true
			}
		}
	}
	return false
}

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

func watchInterrupted() *clierrors.CLIError {
	return clierrors.New("run.interrupted", "Watch interrupted",
		"Stopped watching before the run completed. The run is still active in CircleCI.").
		WithExitCode(clierrors.ExitCancelled)
}

// fetchWatchState retrieves the current run state including all workflows
// and their jobs, reusing buildOutput from get.go.
func fetchWatchState(ctx context.Context, client *apiclient.Client, runID string) (runGetOutput, error) {
	r, err := client.GetRunV3(ctx, runID)
	if err != nil {
		return runGetOutput{}, err
	}
	workflows, err := client.GetRunWorkflowsV3(ctx, runID)
	if err != nil {
		return runGetOutput{}, err
	}
	wfJobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return runGetOutput{}, err
		}
		wfJobs[i] = jobs
	}
	return buildOutput(r, workflows, wfJobs), nil
}

func allWorkflowsDone(workflows []workflowOutput) bool {
	if len(workflows) == 0 {
		return false
	}
	for _, wf := range workflows {
		if wf.Phase != "ended" {
			return false
		}
	}
	return true
}

func watchFingerprint(state runGetOutput) string {
	var b strings.Builder
	for _, wf := range state.Workflows {
		b.WriteString(wf.Name)
		b.WriteByte('=')
		b.WriteString(wf.Phase)
		b.WriteByte('/')
		b.WriteString(wf.Outcome)
		b.WriteByte(';')
		for _, j := range wf.Jobs {
			b.WriteString(j.Name)
			b.WriteByte('=')
			b.WriteString(j.Phase)
			b.WriteByte('/')
			b.WriteString(j.Outcome)
			b.WriteByte(';')
		}
	}
	return b.String()
}

func printWatchTable(ctx context.Context, state runGetOutput, elapsed time.Duration) int {
	lines := 0
	for _, wf := range state.Workflows {
		wfStatus := apiclient.PhaseOutcomeStatus(wf.Phase, wf.Outcome, wf.CurrentOutcome)
		if wf.Duration != "" {
			iostream.ErrPrintf(ctx, "  %-28s  %-12s  %s\n", wf.Name, wfStatus, wf.Duration)
		} else {
			iostream.ErrPrintf(ctx, "  %-28s  %s\n", wf.Name, wfStatus)
		}
		lines++
		for _, j := range wf.Jobs {
			iostream.ErrPrintf(ctx, "    %-30s  %-10s  %s\n", j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type)
			lines++
		}
	}
	iostream.ErrPrintf(ctx, "\n  Elapsed: %s\n", formatElapsed(elapsed))
	lines += 2
	return lines
}

func printWatchTableFinal(ctx context.Context, state runGetOutput) {
	for _, wf := range state.Workflows {
		wfStatus := apiclient.PhaseOutcomeStatus(wf.Phase, wf.Outcome, wf.CurrentOutcome)
		if wf.Duration != "" {
			iostream.ErrPrintf(ctx, "  %-28s  %-12s  %s\n", wf.Name, wfStatus, wf.Duration)
		} else {
			iostream.ErrPrintf(ctx, "  %-28s  %s\n", wf.Name, wfStatus)
		}
		for _, j := range wf.Jobs {
			iostream.ErrPrintf(ctx, "    %-30s  %-10s  %s\n", j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type)
		}
	}
}

func printWatchLine(ctx context.Context, state runGetOutput, elapsed time.Duration) {
	parts := make([]string, 0, len(state.Workflows))
	for _, wf := range state.Workflows {
		parts = append(parts, fmt.Sprintf("%s=%s", wf.Name, apiclient.PhaseOutcomeStatus(wf.Phase, wf.Outcome, wf.CurrentOutcome)))
	}
	iostream.ErrPrintf(ctx, "[%s]  %s\n", formatElapsed(elapsed), strings.Join(parts, "  "))
}

func watchFailFastResult(ctx context.Context, state runGetOutput, runID string, elapsed time.Duration) error {
	names := failedJobNames(state)
	iostream.ErrPrintf(ctx, "%s Run %s has failing job(s): %s — exiting (%s)\n",
		iostream.SymbolFail(ctx), runID, strings.Join(names, ", "), formatElapsed(elapsed))
	return clierrors.New("run.failed", "Run failed",
		fmt.Sprintf("Run %s has %d failing job(s); exiting due to --failfast.", runID, len(names))).
		WithSuggestions(failedJobLogSuggestions(state)...).
		WithExitCode(clierrors.ExitGeneralError)
}

func failedJobNames(state runGetOutput) []string {
	var names []string
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Outcome == "failed" {
				names = append(names, j.Name)
			}
		}
	}
	return names
}

func watchFinalResult(ctx context.Context, state runGetOutput, runID string, elapsed time.Duration) error {
	status := deriveDisplayStatus(state)
	switch status {
	case "succeeded":
		iostream.ErrPrintf(ctx, "%s Run %s succeeded (%s)\n",
			iostream.SymbolOK(ctx), runID, formatElapsed(elapsed))
		return nil
	case "canceled":
		iostream.ErrPrintf(ctx, "Run %s was cancelled (%s)\n", runID, formatElapsed(elapsed))
		return clierrors.New("run.cancelled", "Run cancelled",
			fmt.Sprintf("Run %s was cancelled.", runID)).
			WithExitCode(clierrors.ExitCancelled)
	default:
		iostream.ErrPrintf(ctx, "%s Run %s failed (%s)\n",
			iostream.SymbolFail(ctx), runID, formatElapsed(elapsed))
		return clierrors.New("run.failed", "Run failed",
			fmt.Sprintf("Run %s failed.", runID)).
			WithSuggestions(failedJobLogSuggestions(state)...).
			WithExitCode(clierrors.ExitGeneralError)
	}
}

func failedJobLogSuggestions(state runGetOutput) []string {
	var suggestions []string
	for _, wf := range state.Workflows {
		for _, j := range wf.Jobs {
			if j.Outcome == "failed" {
				suggestions = append(suggestions,
					fmt.Sprintf("View logs for failed job %q: circleci job get <job-id>", j.Name))
			}
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
