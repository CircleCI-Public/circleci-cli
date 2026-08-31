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

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
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
			"help:arguments": heredoc.Docf(`
				%[1]s<run-id>%[1]s is optional: a run UUID (as shown by %[1]scircleci run list --json%[1]s)
				or a run number (as shown by %[1]scircleci run list%[1]s).

				When omitted, the latest run for the current branch is watched.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Monitor a CircleCI run and block until it reaches a terminal state. Without
			arguments, watches the latest run for the current branch. A terminal gets a
			live table of workflows and jobs; piped or in CI, each change prints a line.

			Exit code reflects the result: 0 succeeded, 1 failed, 6 cancelled, 7 the run's
			config was rejected (a dynamic-config continuation included), 8 timed out.
			With --sha, polls up to 2 minutes for the run to appear — useful after a push.
		`),
		Example: heredoc.Doc(`
			# Watch the latest run on the current branch
			$ circleci run watch

			# Push and watch in one step
			$ git push && circleci run watch --sha $(git rev-parse HEAD)

			# Watch by UUID (e.g. from 'run list --json')
			$ circleci run watch 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Watch with a longer timeout
			$ circleci run watch --timeout 30m

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
	var (
		id  uuid.UUID
		err error
	)
	isUUID := false
	if len(args) == 1 {
		id, err = uuid.Parse(args[0])
		isUUID = err == nil
	}

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

	var r *apiclient.RunV3

	switch {
	case isUUID:
		r, err = client.GetRunV3(ctx, id)
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
			return apiErr(err, p.ID.String())
		}

	case sha != "":
		r, err = waitForRunBySHA(ctx, client, projectSlug, branch, sha)
		if err != nil {
			return err
		}

	default:
		proj, pErr := client.GetProjectBySlug(ctx, projectSlug)
		if pErr != nil {
			return apiErr(pErr, projectSlug)
		}
		now := time.Now().UTC()
		runs, sErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID.String()},
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

	cmdutil.TrackKnownID(ctx, cmdutil.KeyRunID, r.ID)

	displayBranch := r.Branch
	if displayBranch == "" {
		displayBranch = branch
	}

	// An interactive terminal gets the live table, which names the run in its own
	// header. Everywhere else — piped, redirected, or CI — the run is announced on
	// one line and progress is reported a line at a time.
	if iostream.IsInteractive(ctx) {
		return watchInteractive(ctx, client, r.ID, displayBranch, timeout, failFast)
	}

	iostream.ErrPrintf(ctx, "Watching run %s (%s)\n\n", r.ID, displayBranch)

	return watchUntilDone(ctx, client, r.ID, timeout, failFast)
}

// waitForRunBySHA searches for a run matching the given commit SHA via V3 search,
// polling every 5 seconds for up to shaWaitDuration() if not immediately found.
func waitForRunBySHA(ctx context.Context, client *apiclient.Client, projectSlug, branch, sha string) (*apiclient.RunV3, error) {
	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return nil, apiErr(err, projectSlug)
	}

	waitDur := shaWaitDuration()
	deadline := time.Now().Add(waitDur)
	interval := 5 * time.Second
	printed := false

	filter := fmt.Sprintf("pipeline.git.revision == %q", sha)
	if branch != "" {
		filter += fmt.Sprintf(" and pipeline.git.branch == %q", branch)
	}

	for {
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID.String()},
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

// watchInteractive runs the watch as a bubbletea program (see
// ui.RunWatchFlowModel): a live table of workflows and jobs on stderr, redrawn
// in place, that ends itself when the run does. The program only collects the
// outcome; the summary line and exit code are decided here, by the same
// functions the non-interactive path uses, so both agree on what a failed run
// looks like.
func watchInteractive(ctx context.Context, client *apiclient.Client, runID uuid.UUID, branch string, timeout time.Duration, failFast bool) error {
	model := ui.NewRunWatchFlow(ctx, ui.RunWatchFlowOptions{
		RunID:    runID,
		Branch:   branch,
		Color:    iostream.ColorEnabled(ctx),
		Animate:  iostream.SpinnerEnabled(ctx),
		FailFast: failFast,
		Timeout:  timeout,
		Fetch: func(ctx context.Context) (ui.RunWatchState, error) {
			state, err := fetchWatchState(ctx, client, runID)
			if err != nil {
				return ui.RunWatchState{}, err
			}
			return watchState(state), nil
		},
	})

	final, err := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	).Run()
	if err != nil {
		// A SIGINT that arrived as a signal rather than a keystroke, or a
		// cancelled context, ends the program with one of bubbletea's terminal
		// errors. That is the user stopping the watch, not the watch failing.
		if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) ||
			errors.Is(err, context.Canceled) {
			return watchInterrupted()
		}
		return clierrors.New("run.watch_display_failed", "Failed to display the run",
			err.Error()).WithExitCode(clierrors.ExitGeneralError)
	}

	res := final.(ui.RunWatchFlowModel).Result()

	// Separate the table the program left on screen from the summary line.
	iostream.ErrPrintf(ctx, "\n")

	switch {
	case res.Cancelled:
		return watchInterrupted()
	case res.Err != nil:
		return clierrors.New("api.error", "API error while watching run", res.Err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	case res.TimedOut:
		return watchTimedOut(runID, timeout)
	case res.FailFast:
		return watchFailFastResult(ctx, res.State, runID, res.Elapsed)
	default:
		return watchFinalResult(ctx, res.State, runID, res.Elapsed)
	}
}

// watchUntilDone polls the given run until all workflows reach a terminal state
// or the timeout elapses, printing one line per observed change. This is the
// non-interactive path: no cursor movement, so the output survives being piped
// to a file or a CI log.
func watchUntilDone(ctx context.Context, client *apiclient.Client, runID uuid.UUID, timeout time.Duration, failFast bool) error {
	deadline := time.Now().Add(timeout)
	start := time.Now()

	var prevFingerprint string
	notedPending := false
	pollInterval := ui.RunWatchPollInterval

	for {
		raw, err := fetchWatchState(ctx, client, runID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return watchInterrupted()
			}
			return clierrors.New("api.error", "API error while watching run", err.Error()).
				WithExitCode(clierrors.ExitAPIError)
		}

		state := watchState(raw)
		elapsed := time.Since(start)

		if fingerprint := watchFingerprint(state); fingerprint != prevFingerprint {
			printWatchLine(ctx, state, elapsed)
			prevFingerprint = fingerprint
		}

		// A dynamic-config run goes quiet here: its setup workflow has ended and the
		// continued workflow does not exist yet, so nothing further prints until one
		// appears — or, if the continued config is rejected, until the run ends
		// carrying the error. Say so once, so the wait does not read as a hung watch.
		if !state.Done && state.AllWorkflowsEnded && !notedPending {
			iostream.ErrPrintf(ctx, "All workflows have ended; waiting for the run itself to finish.\n")
			notedPending = true
		}

		switch {
		case state.Done:
			return watchFinalResult(ctx, state, runID, elapsed)
		case failFast && len(state.FailedJobs()) > 0:
			return watchFailFastResult(ctx, state, runID, elapsed)
		case time.Now().After(deadline):
			return watchTimedOut(runID, timeout)
		}

		if err := sleepOrCancel(ctx, pollInterval); err != nil {
			return watchInterrupted()
		}
		if pollInterval < ui.RunWatchMaxPollInterval {
			pollInterval += ui.RunWatchPollInterval
		}
	}
}

// watchState adapts one poll of run state to the rows the watch table draws,
// plus the two questions both watch paths ask of it: is the run over, and how
// did it end. The status glyph and word come from the single-width
// PhaseOutcomeSymbol/PhaseOutcomeText pair rather than PhaseOutcomeStatus, whose
// emoji shortcodes only render when passed through markdown.
func watchState(state runGetOutput) ui.RunWatchState {
	out := ui.RunWatchState{
		Workflows:         make([]ui.RunWatchWorkflow, 0, len(state.Workflows)),
		Errors:            watchErrors(state.Errors),
		Done:              runEnded(state),
		AllWorkflowsEnded: allWorkflowsEnded(state.Workflows),
		Outcome:           deriveDisplayStatus(state),
	}
	for _, wf := range state.Workflows {
		row := ui.RunWatchWorkflow{
			Name:     wf.Name,
			Symbol:   apiclient.PhaseOutcomeSymbol(wf.Phase, wf.Outcome, wf.CurrentOutcome),
			Status:   apiclient.PhaseOutcomeText(wf.Phase, wf.Outcome, wf.CurrentOutcome),
			Duration: wf.Duration,
			Jobs:     make([]ui.RunWatchJob, 0, len(wf.Jobs)),
		}
		for _, j := range wf.Jobs {
			row.Jobs = append(row.Jobs, ui.RunWatchJob{
				ID:     j.ID,
				Name:   j.Name,
				Symbol: apiclient.PhaseOutcomeSymbol(j.Phase, j.Outcome, j.CurrentOutcome),
				Status: apiclient.PhaseOutcomeText(j.Phase, j.Outcome, j.CurrentOutcome),
				Type:   j.Type,
				Failed: j.Outcome == "failed",
			})
		}
		out.Workflows = append(out.Workflows, row)
	}
	return out
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

func watchTimedOut(runID uuid.UUID, timeout time.Duration) *clierrors.CLIError {
	return clierrors.New("run.timeout", "Watch timed out",
		fmt.Sprintf("Run %s did not complete within %s.", runID, timeout)).
		WithExitCode(clierrors.ExitTimeout)
}

// fetchWatchState retrieves the current run state including all workflows
// and their jobs, reusing buildOutput from get.go.
func fetchWatchState(ctx context.Context, client *apiclient.Client, runID uuid.UUID) (runGetOutput, error) {
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

// runEnded reports whether the run itself has finished, and is the only thing
// the watch treats as the run being over. Its workflows are not: a dynamic-config
// run whose setup workflow has ended is still going — the continued workflow does
// not exist yet, and if its config is rejected it never will — so a watch that
// stopped at "every workflow has ended" would report the setup workflow's success
// as the run's, and never mention the continuation being rejected. A run that
// failed before producing any workflow has none to read either way.
func runEnded(r runGetOutput) bool {
	return r.Phase == apiclient.PhaseEnded
}

// allWorkflowsEnded reports that the run has produced at least one workflow and
// every one of them has ended. That is not the run being over (see runEnded); it
// is what the watch says so on screen for, since the table stops changing there
// while the run carries on.
func allWorkflowsEnded(workflows []workflowOutput) bool {
	if len(workflows) == 0 {
		return false
	}
	for _, wf := range workflows {
		if wf.Phase != apiclient.PhaseEnded {
			return false
		}
	}
	return true
}

// watchErrors adapts a run's own errors — a config that would not compile, a
// continuation that was rejected — to the watch state's error type.
func watchErrors(errs []errorOutput) []ui.RunWatchError {
	out := make([]ui.RunWatchError, 0, len(errs))
	for _, e := range errs {
		out = append(out, ui.RunWatchError{Type: e.Type, Message: e.Message})
	}
	return out
}

// watchFingerprint summarises the statuses in a poll, so the non-interactive
// path can print a line only when something actually moved.
func watchFingerprint(state ui.RunWatchState) string {
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

func printWatchLine(ctx context.Context, state ui.RunWatchState, elapsed time.Duration) {
	parts := make([]string, 0, len(state.Workflows))
	for _, wf := range state.Workflows {
		parts = append(parts, fmt.Sprintf("%s=%s", wf.Name, wf.Status))
	}
	iostream.ErrPrintf(ctx, "[%s]  %s\n", ui.FormatElapsed(elapsed), strings.Join(parts, "  "))
}

func watchFailFastResult(ctx context.Context, state ui.RunWatchState, runID uuid.UUID, elapsed time.Duration) error {
	names := failedJobNames(state)
	iostream.ErrPrintf(ctx, "%s Run %s has failing job(s): %s — exiting (%s)\n",
		iostream.SymbolFail(ctx), runID, strings.Join(names, ", "), ui.FormatElapsed(elapsed))
	return clierrors.New("run.failed", "Run failed",
		fmt.Sprintf("Run %s has %d failing job(s); exiting due to --failfast.", runID, len(names))).
		WithSuggestions(failedJobLogSuggestions(state)...).
		WithExitCode(clierrors.ExitGeneralError)
}

func failedJobNames(state ui.RunWatchState) []string {
	failed := state.FailedJobs()
	names := make([]string, 0, len(failed))
	for _, j := range failed {
		names = append(names, j.Name)
	}
	return names
}

func watchFinalResult(ctx context.Context, state ui.RunWatchState, runID uuid.UUID, elapsed time.Duration) error {
	switch state.Outcome {
	case "succeeded":
		iostream.ErrPrintf(ctx, "%s Run %s succeeded (%s)\n",
			iostream.SymbolOK(ctx), runID, ui.FormatElapsed(elapsed))
		return nil
	case "canceled":
		iostream.ErrPrintf(ctx, "Run %s was cancelled (%s)\n", runID, ui.FormatElapsed(elapsed))
		return clierrors.New("run.cancelled", "Run cancelled",
			fmt.Sprintf("Run %s was cancelled.", runID)).
			WithExitCode(clierrors.ExitCancelled)
	default:
		iostream.ErrPrintf(ctx, "%s Run %s failed (%s)\n",
			iostream.SymbolFail(ctx), runID, ui.FormatElapsed(elapsed))
		return clierrors.New("run.failed", "Run failed", runFailureMessage(runID, state)).
			WithSuggestions(runFailureSuggestions(state)...).
			WithExitCode(runFailureExitCode(state))
	}
}

// runFailureMessage explains a finished run that did not succeed. A run's own
// errors carry the explanation when it has any: they are the failures that belong
// to the run rather than to one of its jobs, and the case that arrives here with
// nothing else to show is a dynamic-config run whose continued config was
// rejected — its setup workflow succeeded, no job failed, and the API reports the
// run itself as succeeded, so the error it carries is the only account of what
// went wrong.
func runFailureMessage(runID uuid.UUID, state ui.RunWatchState) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "Run %s failed.", runID)
	for _, e := range state.Errors {
		_, _ = fmt.Fprintf(&b, "\n\n%s", runErrorLine(e))
	}
	return b.String()
}

// runErrorLine renders one run error as "<type> error: <message>", or as the
// message alone when the API gave the error no type.
func runErrorLine(e ui.RunWatchError) string {
	msg := strings.TrimSpace(e.Message)
	if e.Type == "" {
		return "error: " + msg
	}
	return e.Type + " error: " + msg
}

// runFailureSuggestions is what to do next about a failed run: validate the
// config behind a config error — which for a dynamic-config run is the config the
// setup workflow generated, not the one in the repository — and then fetch the
// logs of each failed job.
func runFailureSuggestions(state ui.RunWatchState) []string {
	var suggestions []string
	if hasConfigError(state) {
		suggestions = append(suggestions,
			"Validate the config locally: circleci config validate .circleci/config.yml",
			"For dynamic config, validate the config the setup job generates, not .circleci/config.yml",
		)
	}
	return append(suggestions, failedJobLogSuggestions(state)...)
}

// runFailureExitCode separates a run the platform would not accept from one that
// ran and failed. A config error means nothing was wrong with the run's jobs —
// there was no valid config to build them from — so it exits like the other
// validation failures in the CLI rather than as a general error.
func runFailureExitCode(state ui.RunWatchState) int {
	if hasConfigError(state) && len(state.FailedJobs()) == 0 {
		return clierrors.ExitValidationFail
	}
	return clierrors.ExitGeneralError
}

// hasConfigError reports whether any of the run's own errors is a config error —
// the type the API attaches to a config it would not compile, including the
// continued config of a dynamic-config run.
func hasConfigError(state ui.RunWatchState) bool {
	for _, e := range state.Errors {
		if e.Type == "config" {
			return true
		}
	}
	return false
}

// failedJobLogSuggestions builds one runnable suggestion per failed job. The
// job UUID is already in the state we just rendered, so it is interpolated
// into the command rather than left as a placeholder the user has to resolve.
// A job that arrived without an ID falls back to the placeholder — a nil UUID
// would produce a command that looks copy-pasteable but cannot work.
func failedJobLogSuggestions(state ui.RunWatchState) []string {
	failed := state.FailedJobs()
	suggestions := make([]string, 0, len(failed))
	for _, j := range failed {
		id := "<job-id>"
		if j.ID != uuid.Nil {
			id = j.ID.String()
		}
		suggestions = append(suggestions,
			fmt.Sprintf("View logs for failed job %q: circleci job get %s", j.Name, id))
	}
	return suggestions
}
