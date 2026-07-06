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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/job"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/workflow"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

const (
	statusCanceled     = "canceled"
	maxWorkflowFetches = 8

	// defaultBranchGuess is the branch assumed for latest-run lookup when
	// --project is given explicitly but --branch is not. The local checkout's
	// current branch is meaningless for a (possibly different) project, so rather
	// than consulting git we guess the common default branch name.
	defaultBranchGuess = "main"
)

func newGetCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get [<run-id>]",
		Short: "Get a run's status",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<run-id>%[1]s is optional and is the UUID of the run to look up. When
				omitted, the latest run is resolved from the project and branch
				inferred from the current git repository's remote and checked-out
				branch (override with %[1]s--project%[1]s and %[1]s--branch%[1]s). With
				%[1]s--project%[1]s set, the branch defaults to main unless %[1]s--branch%[1]s is given.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Display the status of a CircleCI run and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch. When
			--project is given, the branch defaults to main (not the local checkout,
			which is meaningless for a different project) unless --branch is set.

			Pass a run UUID to look up a specific run.

			JSON fields: id, phase, outcome, current_outcome, branch, tag, revision,
			             created_at, errors[].type/message,
			             workflows[].id/name/phase/outcome/current_outcome/duration/
			             jobs[].id/name/phase/outcome/current_outcome/type
		`),
		Example: heredoc.Doc(`
			# Get the latest run for the current branch
			$ circleci run get

			# Get a run by UUID
			$ circleci run get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci run get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used for latest-run lookup")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to the current branch, or main when --project is set)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runGetOutput struct {
	ID             uuid.UUID        `json:"id"`
	Phase          string           `json:"phase"`
	Outcome        string           `json:"outcome,omitempty"`
	CurrentOutcome string           `json:"current_outcome,omitempty"`
	Branch         string           `json:"branch,omitempty"`
	Tag            string           `json:"tag,omitempty"`
	Revision       string           `json:"revision,omitempty"`
	CreatedAt      string           `json:"created_at"`
	Errors         []errorOutput    `json:"errors,omitempty"`
	Workflows      []workflowOutput `json:"workflows"`
}

type errorOutput struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type workflowOutput struct {
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	Phase          string      `json:"phase"`
	Outcome        string      `json:"outcome,omitempty"`
	CurrentOutcome string      `json:"current_outcome,omitempty"`
	Duration       string      `json:"duration,omitempty"`
	Jobs           []jobOutput `json:"jobs"`
}

type jobOutput struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Phase          string    `json:"phase"`
	Outcome        string    `json:"outcome,omitempty"`
	CurrentOutcome string    `json:"current_outcome,omitempty"`
	Type           string    `json:"type,omitempty"`
}

func runGet(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string, jsonOut bool) error {
	// With no run ID and an interactive terminal, walk the user through a series
	// of pickers (run → workflow → job) instead of silently resolving the latest
	// run. JSON output stays non-interactive so scripting is unaffected.
	if len(args) == 0 && !jsonOut && iostream.IsInteractive(ctx) {
		return runGetInteractive(ctx, client, projectSlug, branch)
	}

	var r *apiclient.RunV3

	if len(args) == 1 {
		id, err := uuid.Parse(args[0])
		if err != nil {
			return err
		}

		r, err = client.GetRunV3(ctx, id)
		if err != nil {
			return apiErr(err, id.String())
		}
	} else {
		effectiveBranch := branch
		if projectSlug == "" {
			// No --project: infer both the project and (when not supplied) the
			// branch from the current git repository.
			info, err := gitremote.Detect()
			if err != nil {
				return cmdutil.GitDetectErr(err, "Or provide a run UUID: circleci run get <uuid>")
			}
			projectSlug = info.Slug
			if effectiveBranch == "" {
				effectiveBranch = info.Branch
			}
		} else if effectiveBranch == "" {
			// --project was given explicitly; the local branch is meaningless for a
			// (possibly different) project, so default to "main" rather than the
			// checked-out branch.
			effectiveBranch = defaultBranchGuess
		}

		proj, err := client.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}

		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching latest run for %s on branch %s", projectSlug, effectiveBranch))
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID.String()},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildRunFilter(effectiveBranch, ""),
			Limit:      1,
		})
		sp.Stop()
		if searchErr != nil {
			return apiErr(searchErr, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		if len(runs) == 0 {
			return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		r = &runs[0]
	}

	return displayRun(ctx, client, r, jsonOut)
}

// displayRun fetches a run's workflows and their jobs, then renders the run
// summary as JSON or markdown. This is the shared output path for both the
// direct "circleci run get" invocation and the interactive picker's
// "see all workflows" choice, so both produce identical output.
func displayRun(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3, jsonOut bool) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, r.ID)
	if err != nil {
		// The workflows API can 404 for a run that exists (e.g. workflows
		// not yet materialised) — still show the run, with no workflows.
		if !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return apiErr(err, r.ID.String())
		}
		workflows = nil
	}

	wfJobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(maxWorkflowFetches)
		g.Go(func() error {
			jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
			if err != nil {
				return apiErr(err, wf.ID.String())
			}
			wfJobs[i] = jobs
			return nil
		})
		err = g.Wait()
		if err != nil {
			return err
		}
	}

	out := buildOutput(r, workflows, wfJobs)

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.RunURL(appURL, r.ID)

	printRun(ctx, out, u)
	return nil
}

// createdWindow resolves the explicit [from, to] creation-time window for an
// active "Created" filter, measured from now. Both bounds are always set —
// including a 90-day floor on the lower bound for an "older than" query — so the
// runs endpoints never fall back to their implicit (~14-day) default window. That
// matters most for "my runs", where an implicit lower bound also narrows the
// recently-active project set and can hide every matching run (see
// RUN_DATE_RANGES.md). Call only when created.Active().
//
// The relative-age buckets are all well under 90 days, so the older-than window
// [now-90d, now-duration] is always a valid, non-empty range.
func createdWindow(created ui.RunCreatedFilter, now time.Time) (from, to time.Time) {
	cut := now.Add(-created.Duration)
	if created.Newer {
		return cut, now // newer than the cut, up to now
	}
	return now.AddDate(0, 0, -90), cut // older than the cut, floored at 90 days
}

// runGetInteractive drives the interactive picker flow used when "circleci run
// get" is run in a terminal with no run ID:
//
//  1. Pick from the 10 most recent runs on the current branch.
//  2. Pick a workflow, or "see all workflows" to print the run summary
//     (identical to "circleci run get <id>").
//  3. For a chosen workflow, pick a job, or "all jobs in workflow" to print the
//     workflow summary (identical to "circleci workflow get <id>"). Picking a
//     job prints the job summary (identical to "circleci job get <id>").
//
// Each terminal choice reuses the existing command's output code so the
// rendered markdown matches its non-interactive counterpart exactly. The
// pickers are cancellable with esc/ctrl+c, which returns nil (no output).
func runGetInteractive(ctx context.Context, client *apiclient.Client, projectSlug, branch string) error {
	effectiveBranch := branch
	var (
		defaultBranch string
		myRunsOnly    bool
	)
	// When --project is given explicitly the local checkout is meaningless for a
	// (possibly different) project, so git is not consulted at all: the branch
	// defaults to "main" when not supplied, and the default-branch toggle stays
	// off. Otherwise the git remote supplies the project and (when not supplied)
	// the branch and default branch. When no project could be inferred (not inside
	// a repository) fall back to a "my runs" only flow — the authenticated user's
	// runs across all projects — rather than erroring: project- and branch-scoped
	// runs need a project, but "my runs" does not.
	if projectSlug != "" {
		if effectiveBranch == "" {
			effectiveBranch = defaultBranchGuess
		}
	} else {
		info, err := gitremote.Detect()
		if err != nil {
			myRunsOnly = true
		} else {
			projectSlug = info.Slug
			if effectiveBranch == "" {
				effectiveBranch = info.Branch
			}
			defaultBranch = info.DefaultBranch
		}
	}

	// The project is resolved only for the project-scoped flow; the "my runs"
	// fallback lists across all projects and needs no project lookup.
	var projectID string
	if !myRunsOnly {
		proj, err := client.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}
		projectID = proj.ID.String()
	}

	// fetchRuns lists the 10 most recent runs for a branch as picker items,
	// optionally narrowed to a pipeline status and a created-age window. It is used
	// for the initial load and re-run on each branch-scope toggle, status-filter or
	// created-filter change. Unused in the "my runs" only flow (there is no project
	// to scope to).
	fetchRuns := func(ctx context.Context, br, status string, created ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
		now := time.Now().UTC()
		// The default window is the last 90 days; a created filter narrows it to
		// runs older or newer than its relative age (still floored at 90 days).
		from, to := now.AddDate(0, 0, -90), now
		if created.Active() {
			from, to = createdWindow(created, now)
		}
		runs, err := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{projectID},
			From:       from,
			To:         to,
			Filter:     apiclient.BuildRunFilter(br, status),
			Limit:      10,
		})
		if err != nil {
			return nil, apiErr(err, fmt.Sprintf("%s@%s", projectSlug, br))
		}
		return runItems(runs), nil
	}

	// fetchMyRuns lists the authenticated user's recent runs across all projects
	// (optionally narrowed to a pipeline status), backing the run picker's "my
	// runs" scope. Unlike fetchRuns it is neither project- nor branch-scoped (the
	// counterpart to "circleci my runs"). Because it spans projects, each row folds
	// its project (the "org/repo" slug from the run's repository URL) into the ref
	// bracket as "[project:branch]".
	fetchMyRuns := func(ctx context.Context, status string, created ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
		// Only send explicit bounds when a created filter is active. With no filter,
		// leave from/to nil so the endpoint applies its own default window (as
		// "circleci my runs" does). An active filter must always send an explicit
		// lower bound: an "older than" query would otherwise inherit the endpoint's
		// implicit ~14-day default from, which also collapses the recently-active
		// project set to nothing and returns no runs (see RUN_DATE_RANGES.md).
		params := apiclient.MyRunsParams{Limit: 10, Status: status}
		if created.Active() {
			from, to := createdWindow(created, time.Now().UTC())
			params.From, params.To = &from, &to
		}
		runs, err := client.ListMyRunsV3(ctx, params)
		if err != nil {
			return nil, apiErr(err, "your runs")
		}
		return runItemsWithProjects(runs, true), nil
	}

	var (
		items []ui.RunGetItem
		err   error
	)
	if myRunsOnly {
		sp := iostream.Spinner(ctx, true, "Fetching your recent runs")
		items, err = fetchMyRuns(ctx, "", ui.RunCreatedFilter{})
		sp.Stop()
	} else {
		sp := iostream.Spinner(ctx, true, fmt.Sprintf("Fetching recent runs for %s on branch %s", projectSlug, effectiveBranch))
		items, err = fetchRuns(ctx, effectiveBranch, "", ui.RunCreatedFilter{})
		sp.Stop()
	}
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if myRunsOnly {
			return apiErr(fmt.Errorf("no runs found"), "your runs")
		}
		return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
	}

	model := ui.NewRunGetFlow(ctx, ui.RunGetFlowOptions{
		Runs:             items,
		Color:            iostream.ColorEnabled(ctx),
		Animate:          iostream.SpinnerEnabled(ctx),
		CurrentBranch:    effectiveBranch,
		DefaultBranch:    defaultBranch,
		MyRunsOnly:       myRunsOnly,
		FetchRuns:        fetchRuns,
		FetchMyRuns:      fetchMyRuns,
		StatusFilters:    runStatusFilters,
		FetchWorkflows:   workflowItems(client),
		FetchJobs:        jobItems(client),
		FetchExecutions:  executionItems(client),
		FetchStepStdout:  stepStdout(client),
		FetchStepStderr:  stepStderr(client),
		FetchFailedTests: failedTestItems(client),
		RenderMarkdown: func(md string, width int) string {
			return iostream.RenderMarkdownAt(ctx, md, width)
		},
	})

	final, err := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	).Run()
	if err != nil {
		return err
	}

	res := final.(ui.RunGetFlowModel).Result()
	if res.Err != nil {
		return res.Err
	}

	// The picker only collected the choice; the matching summary is printed now,
	// after the program has exited, reusing each command's existing output code.
	switch res.Action {
	case ui.RunGetActionShowRun:
		// Fetch the run by ID rather than reusing the picker list: a branch toggle
		// may have left the chosen run out of the currently loaded slice.
		r, err := client.GetRunV3(ctx, res.RunID)
		if err != nil {
			return apiErr(err, res.RunID.String())
		}
		return displayRun(ctx, client, r, false)
	case ui.RunGetActionShowWorkflow:
		return workflow.Get(ctx, client, res.WorkflowID.String(), false)
	case ui.RunGetActionShowJob:
		return job.Get(ctx, client, res.JobID.String(), false)
	case ui.RunGetActionShowJobOutput:
		return showJobOutput(ctx, client, res.JobID)
	case ui.RunGetActionCancel:
		return nil
	default:
		return nil
	}
}

// workflowItems returns a fetch closure for the run-get picker: it lists a
// run's workflows as selectable items labelled with a status symbol and name. A
// 404 means the run exists but has no workflows yet, returned as an empty list
// so the picker still offers the run summary.
func workflowItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
	return func(ctx context.Context, runID uuid.UUID) ([]ui.RunGetItem, error) {
		wfs, err := client.GetRunWorkflowsV3(ctx, runID)
		if err != nil {
			if httpcl.HasStatusCode(err, http.StatusNotFound) {
				return nil, nil
			}
			return nil, apiErr(err, runID.String())
		}
		items := make([]ui.RunGetItem, len(wfs))
		for i, w := range wfs {
			items[i] = ui.RunGetItem{
				ID:    w.ID,
				Icon:  apiclient.PhaseOutcomeSymbol(w.Phase, w.Outcome, w.CurrentOutcome),
				Label: w.Name,
			}
		}
		return items, nil
	}
}

// jobItems returns a fetch closure for the run-get picker: it lists a
// workflow's jobs as selectable items labelled with a status symbol and name.
func jobItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
	return func(ctx context.Context, workflowID uuid.UUID) ([]ui.RunGetItem, error) {
		jobs, err := client.GetWorkflowJobsV3(ctx, workflowID)
		if err != nil {
			return nil, apiErr(err, workflowID.String())
		}
		items := make([]ui.RunGetItem, len(jobs))
		for i, j := range jobs {
			items[i] = ui.RunGetItem{
				ID:    j.ID,
				Icon:  apiclient.PhaseOutcomeSymbol(j.Phase, j.Outcome, j.CurrentOutcome),
				Label: j.Name,
			}
		}
		return items, nil
	}
}

// executionItems returns a fetch closure for the run-get picker: it lists a
// job's parallel executions, each carrying its steps. The flow shows the
// execution picker only when there is more than one; otherwise it goes straight
// to the single execution's steps.
func executionItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetExecution, error) {
	return func(ctx context.Context, jobID uuid.UUID) ([]ui.RunGetExecution, error) {
		j, err := client.GetJobV3(ctx, jobID)
		if err != nil {
			return nil, cmdutil.APIErr(err, jobID.String(), "job.not_found", "No job found for %q.")
		}
		// Right-align the index so the durations line up when indices differ in
		// width (e.g. "Execution  9" vs "Execution 10").
		var idxW int
		for _, exec := range j.Executions {
			idxW = max(idxW, len(strconv.Itoa(exec.Index)))
		}

		execs := make([]ui.RunGetExecution, len(j.Executions))
		for i, exec := range j.Executions {
			label := fmt.Sprintf("Execution %*d", idxW, exec.Index)
			if d, ok := executionDuration(exec); ok {
				label += " - " + formatElapsed(d)
			}
			execs[i] = ui.RunGetExecution{
				Index: exec.Index,
				Icon:  executionIcon(exec),
				Label: label,
				Steps: stepRows(exec),
			}
		}
		return execs, nil
	}
}

// stepStdout returns the run-get step pager's stdout reader: a ranged fetch from
// byte offset that reports whether stdout has terminated, so the pager can poll
// until it completes. Output is returned raw (ANSI intact) for colored display.
func stepStdout(client *apiclient.Client) func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error) {
	return func(ctx context.Context, jobID uuid.UUID, execution, stepNum int, offset int64) ([]byte, bool, error) {
		data, terminal, err := client.GetJobStdoutRange(ctx, jobID, execution, stepNum, offset)
		if err != nil {
			return nil, false, cmdutil.APIErr(err, fmt.Sprintf("step %d of job %s", stepNum, jobID),
				"job.output_not_found", "No output found for %s.")
		}
		return data, terminal, nil
	}
}

// stepStderr returns the run-get step pager's stderr reader, fetched once stdout
// terminates. A missing stderr (404) is treated as empty rather than an error.
func stepStderr(client *apiclient.Client) func(context.Context, uuid.UUID, int, int) ([]byte, error) {
	return func(ctx context.Context, jobID uuid.UUID, execution, stepNum int) ([]byte, error) {
		data, err := client.GetJobStderr(ctx, jobID, execution, stepNum)
		if err != nil {
			if httpcl.HasStatusCode(err, http.StatusNotFound) {
				return nil, nil
			}
			return nil, cmdutil.APIErr(err, fmt.Sprintf("step %d of job %s", stepNum, jobID),
				"job.output_not_found", "No output found for %s.")
		}
		return data, nil
	}
}

// failedTestItems returns a fetch closure for the run-get failed-test picker: it
// streams the job's test results and keeps the failures, each carrying its
// message for the pager. The message is passed through verbatim (ANSI intact) so
// the pager can colorize it just like step output.
func failedTestItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetTestItem, error) {
	return func(ctx context.Context, jobID uuid.UUID) ([]ui.RunGetTestItem, error) {
		var items []ui.RunGetTestItem
		err := client.StreamJobTests(ctx, jobID, func(tr apiclient.TestResult) {
			if tr.Result != "failure" {
				return
			}
			label := tr.Name
			if tr.Classname != "" {
				label = fmt.Sprintf("%s (%s)", tr.Name, tr.Classname)
			}
			items = append(items, ui.RunGetTestItem{
				Icon:    "✗",
				Label:   label,
				Message: tr.Message,
			})
		})
		if err != nil {
			return nil, cmdutil.APIErr(err, jobID.String(),
				"job.tests_not_found", "No test results found for job %q.")
		}
		return items, nil
	}
}

// executionDuration is an execution's wall-clock time: from the earliest step
// start to the latest step end. The bool is false until at least one step has
// finished, so a still-running execution shows no duration.
func executionDuration(exec apiclient.JobV3Execution) (time.Duration, bool) {
	var start, end time.Time
	for _, s := range exec.Steps {
		if start.IsZero() || s.StartedAt.Before(start) {
			start = s.StartedAt
		}
		if s.StoppedAt != nil && s.StoppedAt.After(end) {
			end = *s.StoppedAt
		}
	}
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start), true
}

// stepRows builds the selectable step rows for one execution, e.g.
// "101 - 1m4s - run tests [exit: 1]". The number column is right-aligned and the
// duration column left-aligned, each sized to the widest value in the execution
// and followed by " - ".
func stepRows(exec apiclient.JobV3Execution) []ui.RunGetStepItem {
	type row struct {
		step     apiclient.JobV3Step
		num, dur string
	}
	rows := make([]row, len(exec.Steps))
	var numW, durW int
	for i, s := range exec.Steps {
		num := strconv.Itoa(s.Num)
		// A step with no stop time is still running (or never started); show "~"
		// in the duration column rather than a blank gap.
		dur := "~"
		if s.StoppedAt != nil {
			dur = formatElapsed(s.StoppedAt.Sub(s.StartedAt))
		}
		numW = max(numW, len(num))
		durW = max(durW, len(dur))
		rows[i] = row{step: s, num: num, dur: dur}
	}

	items := make([]ui.RunGetStepItem, len(rows))
	for i, r := range rows {
		label := fmt.Sprintf("%*s - %-*s - %s", numW, r.num, durW, r.dur, r.step.Name)
		if r.step.ExitCode != nil {
			label += fmt.Sprintf(" [exit: %d]", *r.step.ExitCode)
		}
		items[i] = ui.RunGetStepItem{
			Icon:      apiclient.PhaseOutcomeSymbol(r.step.Phase, r.step.Outcome, ""),
			Label:     label,
			Execution: exec.Index,
			StepNum:   r.step.Num,
		}
	}
	return items
}

// executionIcon derives a status symbol for a whole execution: failed if any of
// its steps failed or errored, otherwise succeeded.
func executionIcon(exec apiclient.JobV3Execution) string {
	for _, s := range exec.Steps {
		switch apiclient.PhaseOutcomeSymbol(s.Phase, s.Outcome, "") {
		case "✗", "!":
			return "✗"
		}
	}
	return "✓"
}

// showJobOutput prints the full per-step output report for every execution of a
// job. The "full output report" summary is job-level, so a parallel job shows
// all executions rather than only one — job.OutputList renders one at a time.
func showJobOutput(ctx context.Context, client *apiclient.Client, jobID uuid.UUID) error {
	j, err := client.GetJobV3(ctx, jobID)
	if err != nil {
		return cmdutil.APIErr(err, jobID.String(), "job.not_found", "No job found for %q.")
	}
	for _, exec := range j.Executions {
		if err := job.OutputList(ctx, client, jobID, exec.Index, job.DefaultStepOutputTail, false); err != nil {
			return err
		}
	}
	return nil
}

func buildOutput(r *apiclient.RunV3, workflows []apiclient.WorkflowV3, wfJobs [][]apiclient.WorkflowJobV3) runGetOutput {
	wflows := make([]workflowOutput, len(workflows))
	for i, w := range workflows {
		jobs := make([]jobOutput, 0, len(wfJobs[i]))
		for _, j := range wfJobs[i] {
			jobs = append(jobs, jobOutput{
				ID:             j.ID,
				Name:           j.Name,
				Phase:          j.Phase,
				Outcome:        j.Outcome,
				CurrentOutcome: j.CurrentOutcome,
				Type:           j.Type,
			})
		}
		var dur string
		if w.EndedAt != nil {
			dur = formatElapsed(w.EndedAt.Sub(w.CreatedAt))
		}
		wflows[i] = workflowOutput{
			ID:             w.ID,
			Name:           w.Name,
			Phase:          w.Phase,
			Outcome:        w.Outcome,
			CurrentOutcome: w.CurrentOutcome,
			Duration:       dur,
			Jobs:           jobs,
		}
	}

	revision := r.Revision
	if len(revision) > 7 {
		revision = revision[:7]
	}

	errs := make([]errorOutput, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = errorOutput{Type: e.Type, Message: e.Message}
	}

	return runGetOutput{
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Tag:            r.Tag,
		Revision:       revision,
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		Errors:         errs,
		Workflows:      wflows,
	}
}

// deriveDisplayStatus computes a meaningful overall display status from
// workflow phases and outcomes.
func deriveDisplayStatus(r runGetOutput) string {
	if len(r.Workflows) == 0 {
		return apiclient.PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == "failed" || wf.Outcome == "errored" || wf.CurrentOutcome == "failed" {
			return "failed"
		}
	}
	for _, wf := range r.Workflows {
		if wf.Phase != "ended" {
			return "running"
		}
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == statusCanceled {
			return statusCanceled
		}
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == "succeeded" {
			return "succeeded"
		}
	}
	return apiclient.PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
}

func printRun(ctx context.Context, r runGetOutput, u string) {
	var md strings.Builder
	md.WriteString("# Run\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", r.ID)
	if r.Branch != "" {
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
	}
	if r.Tag != "" {
		_, _ = fmt.Fprintf(&md, "- Tag: %s\n", r.Tag)
	}
	if r.Revision != "" {
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", r.Revision)
	}
	_, _ = fmt.Fprintf(&md, "- URL: <%s>\n", u)
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", deriveDisplayStatus(r))
	_, _ = fmt.Fprintf(&md, "- Created: %s\n", r.CreatedAt)

	if len(r.Errors) > 0 {
		md.WriteString("\n## Errors\n")
		for _, e := range r.Errors {
			_, _ = fmt.Fprintf(&md, "- **%s**: %s\n", e.Type, e.Message)
		}
	}
	md.WriteString("\n")

	if len(r.Workflows) == 0 {
		md.WriteString("No workflows found for this run.\n")
	} else {
		md.WriteString("## Workflows\n")
		for _, w := range r.Workflows {
			_, _ = fmt.Fprintf(&md, "### %s\n", w.Name)
			_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", w.ID)
			_, _ = fmt.Fprintf(&md, "- Status: %s\n", apiclient.PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome))
			if w.Duration != "" {
				_, _ = fmt.Fprintf(&md, "- Duration: %s\n", w.Duration)
			}
			md.WriteString("#### Jobs\n")
			mdTable := mdtable.New("Name", "Status", "Type", "ID")
			for _, j := range w.Jobs {
				mdTable.Row(j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type, "`"+j.ID.String()+"`")
			}
			md.WriteString(mdTable.Render())
		}
		md.WriteString("\n")
	}

	iostream.PrintMarkdown(ctx, md.String())
}

// runStatusFilters are the pipeline statuses the run picker's "s" key cycles
// through, in order (the picker prepends an "all statuses" entry). The values
// are apiclient pipeline.status tokens; the labels are the human wording.
// The icons reuse the picker's status glyphs (see colorizeStatusIcon in the ui
// package), so the filter dialog colors them the same as the run rows: ✓ green,
// ✗ red, ! yellow, ● blue, ○ / ⊘ muted.
var runStatusFilters = []ui.RunStatusFilter{
	{Value: apiclient.StatusCanceled, Label: "canceled", Icon: "⊘"},
	{Value: apiclient.StatusFailed, Label: "failed", Icon: "✗"},
	{Value: apiclient.StatusFailing, Label: "failing", Icon: "!"},
	{Value: apiclient.StatusNotRun, Label: "not run", Icon: "⊘"},
	{Value: apiclient.StatusQueued, Label: "queued", Icon: "○"},
	{Value: apiclient.StatusRunning, Label: "running", Icon: "●"},
	{Value: apiclient.StatusSuccess, Label: "success", Icon: "✓"},
}

// runItems maps recent runs to selectable picker items. The status symbol is
// supplied as an icon (the picker colors it and renders raw text, so it cannot
// use the emoji from PhaseOutcomeStatus); the label is e.g.
// "03d8295 [main] - 20 seconds ago".
func runItems(runs []apiclient.RunV3) []ui.RunGetItem {
	return runItemsWithProjects(runs, false)
}

// runItemsWithProjects maps runs to picker items. When withProject is set (the
// cross-project "my runs" scope), each run's project — its "org/repo" slug from
// the repository URL — is folded into the ref bracket as "[project:branch]";
// otherwise (a single-project branch scope) the bracket is just the branch/tag.
func runItemsWithProjects(runs []apiclient.RunV3, withProject bool) []ui.RunGetItem {
	items := make([]ui.RunGetItem, len(runs))
	for i := range runs {
		project := ""
		if withProject {
			project = cmdutil.RepoSlug(runs[i].RepositoryURL)
		}
		items[i] = ui.RunGetItem{
			ID:     runs[i].ID,
			Icon:   apiclient.PhaseOutcomeSymbol(runs[i].Phase, runs[i].Outcome, runs[i].CurrentOutcome),
			Label:  runItemLabel(&runs[i], project),
			Errors: runItemErrors(runs[i].Errors),
		}
	}
	return items
}

// runItemErrors adapts a run's API errors to the picker's error type, so the
// flow can show them under the workflow picker's title.
func runItemErrors(errs []apiclient.RunError) []ui.RunGetError {
	if len(errs) == 0 {
		return nil
	}
	out := make([]ui.RunGetError, len(errs))
	for i, e := range errs {
		out[i] = ui.RunGetError{Type: e.Type, Message: e.Message}
	}
	return out
}

// runItemLabel renders a run's picker label. Normally that is the short
// revision and ref (e.g. "03d8295 [main] - 20 seconds ago"). When project is set
// (the cross-project "my runs" scope) it leads the bracket as "[project:main]".
// A run that never resolved a commit — an errored run whose config could not be
// fetched, or one from an unknown trigger — carries no revision, branch or tag,
// which would leave the label blank but for the timestamp. For those, fall back
// to the run's first error, then its status word, so the row still says
// something.
func runItemLabel(r *apiclient.RunV3, project string) string {
	e := toListEntry(r)
	ref := e.Branch
	if ref == "" {
		ref = e.Tag
	}
	// Fold the project (when set) into the ref bracket: "[project:main]", or just
	// "[project]" for a run with no branch/tag.
	bracket := ref
	switch {
	case project != "" && ref != "":
		bracket = project + ":" + ref
	case project != "":
		bracket = project
	}
	var desc string
	switch {
	case e.Revision != "" && bracket != "":
		desc = fmt.Sprintf("%s [%s]", e.Revision, bracket)
	case e.Revision != "":
		desc = e.Revision
	case bracket != "":
		desc = "[" + bracket + "]"
	case len(r.Errors) > 0:
		desc = errorSummary(r.Errors[0])
	default:
		desc = apiclient.PhaseOutcomeText(r.Phase, r.Outcome, r.CurrentOutcome)
	}
	return desc + " - " + relativeTime(r.CreatedAt)
}

// errorSummary condenses a run error into a single short line for a picker row:
// its first sentence, capped in length, falling back to the error type when the
// message is empty.
func errorSummary(e apiclient.RunError) string {
	msg := strings.TrimSpace(e.Message)
	if i := strings.IndexAny(msg, "\r\n"); i >= 0 {
		msg = msg[:i]
	}
	if i := strings.Index(msg, ". "); i >= 0 {
		msg = msg[:i+1]
	}
	if msg == "" {
		msg = e.Type
	}
	const maxLen = 60
	if len(msg) > maxLen {
		msg = strings.TrimSpace(msg[:maxLen]) + "…"
	}
	return msg
}

// relativeTime renders how long ago t was in coarse, human-friendly units
// (e.g. "20 seconds ago", "3 minutes ago"). Future times clamp to "0 seconds
// ago" so clock skew never produces a negative count.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return pluralAgo(int(d.Seconds()), "second")
	case d < time.Hour:
		return pluralAgo(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return pluralAgo(int(d.Hours()), "hour")
	default:
		return pluralAgo(int(d.Hours()/24), "day")
	}
}

func pluralAgo(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}
