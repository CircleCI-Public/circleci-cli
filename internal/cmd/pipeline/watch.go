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

package pipeline

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// defaultSHAWaitDuration is the maximum time to wait for a pipeline matching a
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
	)

	cmd := &cobra.Command{
		Use:   "watch [<pipeline-number>]",
		Short: "Watch a pipeline until it completes",
		Long: heredoc.Doc(`
			Monitor a CircleCI pipeline and block until it reaches a terminal state.

			Exit code reflects the result:
			  0 = all workflows succeeded
			  1 = one or more workflows failed
			  6 = pipeline was cancelled
			  8 = timed out before pipeline completed

			Without arguments, watches the latest pipeline for the current branch.
			Pass a pipeline number to watch a specific pipeline.

			With --sha, searches the pipeline list for a pipeline matching that
			commit. If not yet found, polls for up to 2 minutes — useful when run
			immediately after git push.
		`),
		Example: heredoc.Doc(`
			# Watch the latest pipeline on the current branch
			$ circleci pipeline watch

			# Push and watch in one step
			$ git push && circleci pipeline watch --sha $(git rev-parse HEAD)

			# Watch a specific pipeline number
			$ circleci pipeline watch 75

			# Watch a pipeline on a different branch
			$ circleci pipeline watch --branch main

			# Watch with a longer timeout
			$ circleci pipeline watch --timeout 60m
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runWatch(ctx, client, streams, args, projectSlug, branch, sha, timeout)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to watch (defaults to current branch)")
	cmd.Flags().StringVar(&sha, "sha", "", "Watch pipeline for this commit SHA; polls up to 2m if not yet created")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Maximum time to wait for pipeline completion")

	return cmd
}

func runWatch(ctx context.Context, client *apiclient.Client, streams iostream.Streams, args []string, projectSlug, branch, sha string, timeout time.Duration) error {
	// Resolve project and branch from git if not fully specified.
	needsGit := projectSlug == "" || (branch == "" && sha == "" && len(args) == 0)
	if needsGit {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify --project and --branch explicitly",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if branch == "" {
			branch = info.Branch
		}
	}

	// Find the pipeline to watch.
	var pipeline *apiclient.Pipeline
	var err error

	switch {
	case len(args) == 1:
		number, _ := strconv.ParseInt(args[0], 10, 64)
		pipeline, err = client.GetPipelineByNumber(ctx, projectSlug, number)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, args[0]))
		}

	case sha != "":
		pipeline, err = waitForPipelineBySHA(ctx, streams, client, projectSlug, branch, sha)
		if err != nil {
			return err
		}

	default:
		pipeline, err = client.GetLatestPipeline(ctx, projectSlug, branch)
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", projectSlug, branch))
		}
	}

	branch = pipeline.ProjectSlug // use slug from the found pipeline
	if pipeline.VCS != nil && pipeline.VCS.Branch != "" {
		branch = pipeline.VCS.Branch
	}

	streams.ErrPrintf("Watching pipeline #%d (%s @ %s)\n\n", pipeline.Number, pipeline.ProjectSlug, branch)

	return watchUntilDone(ctx, streams, client, pipeline.ID, pipeline.Number, timeout)
}

// waitForPipelineBySHA searches for a pipeline matching the given commit SHA,
// polling every 5 seconds for up to shaWaitDuration() if not immediately found.
func waitForPipelineBySHA(ctx context.Context, streams iostream.Streams, client *apiclient.Client, projectSlug, branch, sha string) (*apiclient.Pipeline, error) {
	waitDur := shaWaitDuration()
	deadline := time.Now().Add(waitDur)
	interval := 5 * time.Second
	printed := false

	for {
		pipelines, err := client.ListPipelines(ctx, projectSlug, branch, 10)
		if err != nil {
			return nil, apiErr(err, projectSlug)
		}
		for i := range pipelines {
			p := &pipelines[i]
			if p.VCS != nil && strings.HasPrefix(p.VCS.Revision, sha) {
				return p, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, clierrors.New("pipeline.sha_not_found", "Pipeline not found",
				fmt.Sprintf("No pipeline found for commit %s in %s after %s.", sha, projectSlug, waitDur)).
				WithSuggestions(
					"Verify the push triggered a pipeline in CircleCI",
					"Check the SHA is correct: git rev-parse HEAD",
				).
				WithExitCode(clierrors.ExitNotFound)
		}

		if !printed {
			streams.ErrPrintf("Waiting for pipeline for commit %s...\n", sha)
			printed = true
		}
		time.Sleep(interval)
	}
}

// watchUntilDone polls the given pipeline until all workflows reach a terminal
// state or the timeout elapses.
func watchUntilDone(ctx context.Context, streams iostream.Streams, client *apiclient.Client, pipelineID string, pipelineNumber int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	start := time.Now()
	tty := streams.IsTerminal()

	var prevLines int
	var prevFingerprint string
	pollInterval := 5 * time.Second

	for {
		state, err := fetchWatchState(ctx, client, pipelineID)
		if err != nil {
			return clierrors.New("api.error", "API error while watching pipeline", err.Error()).
				WithExitCode(clierrors.ExitAPIError)
		}

		elapsed := time.Since(start)
		fingerprint := watchFingerprint(state)
		changed := fingerprint != prevFingerprint

		if tty {
			// Erase previous table, redraw with updated elapsed.
			if prevLines > 0 {
				_, _ = fmt.Fprintf(streams.Err, "\033[%dA\033[J", prevLines)
			}
			prevLines = printWatchTable(streams, state, elapsed)
		} else if changed {
			printWatchLine(streams, state, elapsed)
		}
		prevFingerprint = fingerprint

		if allWorkflowsDone(state.Workflows) {
			if tty && prevLines > 0 {
				// Final redraw without the elapsed ticker line.
				_, _ = fmt.Fprintf(streams.Err, "\033[%dA\033[J", prevLines)
				printWatchTableFinal(streams, state)
				streams.ErrPrintf("\n")
			}
			return watchFinalResult(streams, state, pipelineNumber, elapsed)
		}

		if time.Now().After(deadline) {
			streams.ErrPrintf("\n")
			return clierrors.New("pipeline.timeout", "Watch timed out",
				fmt.Sprintf("Pipeline #%d did not complete within %s.", pipelineNumber, timeout)).
				WithExitCode(clierrors.ExitTimeout)
		}

		time.Sleep(pollInterval)
		// Ramp from 5s to 30s over the first few polls.
		if pollInterval < 30*time.Second {
			pollInterval += 5 * time.Second
		}
	}
}

// fetchWatchState retrieves the current pipeline state including all workflows
// and their jobs, reusing buildOutput from get.go.
func fetchWatchState(ctx context.Context, client *apiclient.Client, pipelineID string) (pipelineGetOutput, error) {
	p, err := client.GetPipeline(ctx, pipelineID)
	if err != nil {
		return pipelineGetOutput{}, err
	}
	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return pipelineGetOutput{}, err
	}
	wfJobs := make([][]apiclient.WorkflowJob, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return pipelineGetOutput{}, err
		}
		wfJobs[i] = jobs
	}
	return buildOutput(p, workflows, wfJobs), nil
}

// allWorkflowsDone returns true when every workflow is in a terminal state.
// Returns false when there are no workflows yet (pipeline still starting up).
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
func watchFingerprint(state pipelineGetOutput) string {
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
func printWatchTable(streams iostream.Streams, state pipelineGetOutput, elapsed time.Duration) int {
	lines := 0
	for _, wf := range state.Workflows {
		streams.ErrPrintf("  %-28s  %s\n", wf.Name, wf.Status)
		lines++
		for _, j := range wf.Jobs {
			if j.Number > 0 {
				streams.ErrPrintf("    %-30s  %-12s  #%d\n", j.Name, j.Status, j.Number)
			} else {
				streams.ErrPrintf("    %-30s  %s\n", j.Name, j.Status)
			}
			lines++
		}
	}
	streams.ErrPrintf("\n  Elapsed: %s\n", formatElapsed(elapsed))
	lines += 2
	return lines
}

// printWatchTableFinal renders the final workflow/job table without the elapsed line.
func printWatchTableFinal(streams iostream.Streams, state pipelineGetOutput) {
	for _, wf := range state.Workflows {
		streams.ErrPrintf("  %-28s  %s\n", wf.Name, wf.Status)
		for _, j := range wf.Jobs {
			if j.Number > 0 {
				streams.ErrPrintf("    %-30s  %-12s  #%d\n", j.Name, j.Status, j.Number)
			} else {
				streams.ErrPrintf("    %-30s  %s\n", j.Name, j.Status)
			}
		}
	}
}

// printWatchLine emits a single-line status update for non-TTY output.
func printWatchLine(streams iostream.Streams, state pipelineGetOutput, elapsed time.Duration) {
	var parts []string
	for _, wf := range state.Workflows {
		parts = append(parts, fmt.Sprintf("%s=%s", wf.Name, wf.Status))
	}
	streams.ErrPrintf("[%s]  %s\n", formatElapsed(elapsed), strings.Join(parts, "  "))
}

// watchFinalResult prints the outcome and returns an appropriate error (or nil).
func watchFinalResult(streams iostream.Streams, state pipelineGetOutput, number int64, elapsed time.Duration) error {
	switch state.Status {
	case "success":
		streams.ErrPrintf("%s Pipeline #%d succeeded (%s)\n",
			streams.Symbol("✓", "OK:"), number, formatElapsed(elapsed))
		return nil
	case "canceled":
		streams.ErrPrintf("Pipeline #%d was cancelled (%s)\n", number, formatElapsed(elapsed))
		return clierrors.New("pipeline.cancelled", "Pipeline cancelled",
			fmt.Sprintf("Pipeline #%d was cancelled.", number)).
			WithExitCode(clierrors.ExitCancelled)
	default:
		streams.ErrPrintf("%s Pipeline #%d failed (%s)\n",
			streams.Symbol("✗", "FAIL:"), number, formatElapsed(elapsed))
		return clierrors.New("pipeline.failed", "Pipeline failed",
			fmt.Sprintf("Pipeline #%d failed.", number)).
			WithExitCode(clierrors.ExitGeneralError)
	}
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
