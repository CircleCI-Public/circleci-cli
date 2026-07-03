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

package ui

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/termrender"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// switchScopeBinding is the run picker's "switch trigger" shortcut, built from
// the platform key binding: shift+tab normally, or plain Tab on Windows, where
// the ConPTY/ultraviolet input stack drops shift+tab (the modifier is discarded
// and CSI Z yields no key event at all). Its help label names that key.
var switchScopeBinding = func() key.Binding {
	k := components.KeyShiftTab
	if runtime.GOOS == "windows" {
		k = components.KeyTab
	}
	return key.NewBinding(key.WithKeys(k.Keys()...), key.WithHelp(k.Keys()[0], "change trigger"))
}()

// switchScopeKeyLabel names the platform switch-trigger key for help prose.
var switchScopeKeyLabel = switchScopeBinding.Help().Key

// stepPagerKeys is the footer key hint set for the step-output / test-message
// pager. Unlike the markdown pager, esc goes back to the picker rather than
// quitting.
var stepPagerKeys = []key.Binding{
	components.BindScroll,
	components.BindTopBottom,
	components.BindSearch,
	components.BindBack,
}

// runGetHelpMarkdown is the content of the "?" keyboard-shortcut overlay offered
// on the run-get pickers. It is rendered to styled, width-wrapped text by the
// caller-supplied RenderMarkdown and shown in a scrollable frame. The scope
// toggle names the platform key (shift+tab, or Tab on Windows), matching the
// binding the run picker actually uses.
var runGetHelpMarkdown = `# Keyboard shortcuts

| Key | Action |
| --- | --- |
| ` + "`esc`" + ` | Go back to the previous screen (quit from the first one) |
| ` + "`ctrl+c`" + ` | Quit the app immediately |

## Moving around a picker

| Key | Action |
| --- | --- |
| ` + "`↑` / `↓`" + ` (or ` + "`k` / `j`" + `) | Move the cursor up / down |
| ` + "`PgUp` / `PgDn`" + ` | Jump a page at a time |
| ` + "`g` / `G`" + ` | Jump to the first / last item |
| ` + "`enter`" + ` | Open the highlighted item |
| ` + "`r`" + ` | Refresh the current data |
| ` + "`esc`" + ` | Return to the previous picker |

## The run picker

This has some additional keys:

| Key | Action |
| --- | --- |
| ` + "`" + switchScopeKeyLabel + "`" + ` | Switch trigger — cycle branches and your runs |
| ` + "`s`" + ` | Cycle the status filter |
| ` + "`S`" + ` | Clear the status filter (back to all statuses) |

## Pager

When paging through markdown, step or test output:

| Key | Action |
| --- | --- |
| ` + "`↑` / `↓`" + ` | Scroll one line |
| ` + "`g` / `G`" + ` | Jump to top / bottom |
| ` + "`/`" + ` | Search (then ` + "`n` / `N`" + ` for next / previous match) |
| ` + "`esc`" + ` | Return to the previous screen |
`

// Labels for the "show everything at this level" option that heads each of the
// workflow and job pickers.
const (
	runGetAllWorkflowsLabel = "See all workflows (run summary)"
	runGetAllJobsLabel      = "All jobs in workflow (workflow summary)"
	runGetJobReportLabel    = "Job report (summary)"
	runGetJobOutputLabel    = "Full job report (including step output)"
	runGetFailedTestsLabel  = "Failed tests"

	// runGetMetaCount is the number of leading job-summary options (job report,
	// full output report, failed tests). They sit on the first picker after the
	// job: the step picker for a single-execution job, or the execution picker
	// otherwise.
	runGetMetaCount = 3

	// runGetMetaGlyph fills the icon column for the leading "see all" / "all
	// jobs" summary options. They carry no status, so rather than leave a blank
	// gap they get a muted "list" mark that reads as "show everything here" and
	// keeps the column aligned with the status-bearing rows below.
	runGetMetaGlyph = "≡"

	// runGetFailedTestsGlyph marks the "failed tests" summary option. It is a
	// muted question mark — a state-neutral "did the tests pass?" prompt — rather
	// than a status symbol, since the option is offered whether or not any test
	// actually failed.
	runGetFailedTestsGlyph = "?"
)

// RunGetAction is the terminal choice the user reached in the run-get flow.
type RunGetAction int

const (
	// RunGetActionCancel means the user quit (esc on the first picker, ctrl+c
	// anywhere) without choosing what to display.
	RunGetActionCancel RunGetAction = iota
	// RunGetActionShowRun displays the whole run (all its workflows).
	RunGetActionShowRun
	// RunGetActionShowWorkflow displays a single workflow (all its jobs).
	RunGetActionShowWorkflow
	// RunGetActionShowJob displays a single job.
	RunGetActionShowJob
	// RunGetActionShowJobOutput displays the full per-step output report for a
	// job (the equivalent of "circleci job output list").
	RunGetActionShowJobOutput
)

// RunGetItem is one selectable row: a display label, an optional status symbol
// (uncolored — the flow colors it when color is enabled), and the UUID it maps
// to.
type RunGetItem struct {
	Label string
	Icon  string
	ID    uuid.UUID
	// Errors, set only for run rows, are the run's config/setup errors. When the
	// run is selected they are shown beneath the workflow picker's title so a run
	// that produced no workflows (e.g. a config that failed to compile) explains
	// itself rather than presenting an empty list.
	Errors []RunGetError
}

// RunGetError is a single run-level error (type + message) surfaced under the
// workflow picker title.
type RunGetError struct {
	Type    string
	Message string
}

// RunGetStepItem is one selectable job step. Steps have no UUID; they are
// addressed by their parallel-execution index and step number.
type RunGetStepItem struct {
	Label     string
	Icon      string
	Execution int
	StepNum   int
}

// RunGetTestItem is one selectable failed test: a display label, a status
// symbol, and the test's message shown in the pager when the row is picked.
type RunGetTestItem struct {
	Label   string
	Icon    string
	Message string
}

// RunGetExecution is one parallel execution of a job, carrying its steps. When a
// job's parallelism is greater than one the flow inserts an execution picker
// before the step picker; with a single execution that picker is skipped.
type RunGetExecution struct {
	Label string
	Icon  string
	Index int
	Steps []RunGetStepItem
}

// RunGetResult is the outcome of a completed RunGetFlowModel run, read via
// Result() after tea.Program.Run() returns.
type RunGetResult struct {
	Action     RunGetAction
	RunID      uuid.UUID
	WorkflowID uuid.UUID
	JobID      uuid.UUID
	// Err is set when a mid-flow fetch (workflows, jobs or steps) failed; Action
	// is RunGetActionCancel in that case.
	Err error
}

// RunGetFlowOptions configures a RunGetFlowModel. The fetch callbacks keep this
// program decoupled from the API client: the caller supplies the already-built
// run list and closures that return the next level's items on demand.
type RunGetFlowOptions struct {
	Runs            []RunGetItem
	FetchWorkflows  func(ctx context.Context, runID uuid.UUID) ([]RunGetItem, error)
	FetchJobs       func(ctx context.Context, workflowID uuid.UUID) ([]RunGetItem, error)
	FetchExecutions func(ctx context.Context, jobID uuid.UUID) ([]RunGetExecution, error)
	// FetchStepStdout reads a step's stdout from byte offset, returning the new
	// bytes (raw, ANSI intact) and whether stdout has finished. The pager polls
	// this until terminal. FetchStepStderr reads the step's full stderr once
	// stdout terminates (stdout always completes first).
	FetchStepStdout func(ctx context.Context, jobID uuid.UUID, execution, stepNum int, offset int64) (data []byte, terminal bool, err error)
	FetchStepStderr func(ctx context.Context, jobID uuid.UUID, execution, stepNum int) ([]byte, error)
	// FetchFailedTests lists a job's failed tests for the "failed tests" picker.
	// Each item carries the message shown in the pager when the test is picked.
	FetchFailedTests func(ctx context.Context, jobID uuid.UUID) ([]RunGetTestItem, error)
	Color            bool
	// Animate reports whether the loading spinner should animate. Pass false when
	// CIRCLE_SPINNER_DISABLED is set (or the session is non-interactive) so the
	// loading line stays static instead of repainting.
	Animate bool

	// CurrentBranch is the branch the initial Runs were fetched for.
	// DefaultBranch is the project's default branch. When the two differ, the run
	// picker offers a shift+tab toggle between them, re-fetching via FetchRuns.
	// When DefaultBranch is empty or equal to CurrentBranch, the toggle is hidden.
	// The status argument to FetchRuns is the active status filter (the "s" key),
	// a pipeline.status value ("" = every status).
	CurrentBranch string
	DefaultBranch string
	FetchRuns     func(ctx context.Context, branch, status string) ([]RunGetItem, error)
	// FetchMyRuns lists the authenticated user's recent runs across all projects
	// (the counterpart to "circleci my runs"). When set, the run picker's
	// shift+tab cycle gains a "my runs" scope that fetches via this callback
	// rather than by branch; when nil the scope is omitted. status is the active
	// status filter, as for FetchRuns.
	FetchMyRuns func(ctx context.Context, status string) ([]RunGetItem, error)
	// StatusFilters are the pipeline statuses the "s" key cycles through, in
	// order. The picker prepends an "all statuses" (no filter) entry, so pressing
	// "s" cycles no-filter → each status → back. When empty, the "s" action is
	// omitted.
	StatusFilters []RunStatusFilter

	// RenderMarkdown renders markdown as styled text wrapped to width columns,
	// backing the "?" keyboard-shortcut help overlay the pickers offer. The flow
	// supplies its own help markdown; the caller only provides the renderer (which
	// keeps the ui package decoupled from glamour). When nil, the "?" key is inert
	// and no help hint is shown.
	RenderMarkdown func(md string, width int) string
}

// runScope is one entry in the run picker's shift+tab cycle: a branch filter
// ("" means all branches) with the label shown in the picker title and loading
// line. A myRuns scope is not branch-filtered at all — it lists the user's runs
// across all projects via FetchMyRuns instead of FetchRuns.
type runScope struct {
	branch string // "" = all branches (ignored when myRuns)
	myRuns bool   // list the authenticated user's runs across all projects
	label  string // title/loading wording, e.g. "main branch", "all branches"
}

// titleName is the bracket-inner text for the picker title: the bare branch
// name for a branch scope, "all branches" for the unfiltered scope, or
// "my runs" for the cross-project user scope.
func (s runScope) titleName() string {
	switch {
	case s.myRuns:
		return "my runs"
	case s.branch == "":
		return "all branches"
	default:
		return s.branch
	}
}

// where is the location phrase used in the "no runs" footer note, e.g.
// "on main", "on any branch", or "in your runs".
func (s runScope) where() string {
	switch {
	case s.myRuns:
		return "in your runs"
	case s.branch == "":
		return "on any branch"
	default:
		return "on " + s.branch
	}
}

// buildRunScopes assembles the toggle cycle: the current branch, then the
// default branch when it is known and distinct, then "all branches", and
// finally "my runs" when includeMyRuns is set (i.e. FetchMyRuns is wired). The
// cycle always includes all-branches so a toggle is offered even when there is
// only one branch to name.
func buildRunScopes(current, defaultBranch string, includeMyRuns bool) []runScope {
	branchScope := func(b string) runScope {
		return runScope{branch: b, label: b + " branch"}
	}
	scopes := []runScope{branchScope(current)}
	if defaultBranch != "" && defaultBranch != current {
		scopes = append(scopes, branchScope(defaultBranch))
	}
	scopes = append(scopes, runScope{label: "all branches"})
	if includeMyRuns {
		scopes = append(scopes, runScope{myRuns: true, label: "your runs"})
	}
	return scopes
}

// RunStatusFilter is one selectable pipeline-status filter offered by the run
// picker's "s" key: the API pipeline.status value and the label shown to the
// user. The caller supplies the list (from apiclient status constants) so the
// ui package stays decoupled from the API client.
type RunStatusFilter struct {
	Value string // pipeline.status value
	Label string // title/footer wording, e.g. "failed", "needs approval"
}

// buildStatusCycle prepends the "all statuses" (no-filter) entry to the caller's
// selectable statuses, so pressing "s" cycles no-filter → each status → back.
// With no selectable statuses the cycle is just the no-filter entry and the "s"
// action is suppressed (see statusFilterEnabled).
func buildStatusCycle(selectable []RunStatusFilter) []RunStatusFilter {
	return append([]RunStatusFilter{{Label: "all statuses"}}, selectable...)
}

// runsEmptyNote is the transient footer note shown when a scope+status
// combination has no runs, e.g. "No runs found on main" or "No failed runs in
// your runs".
func runsEmptyNote(scope runScope, status RunStatusFilter) string {
	if status.Value == "" {
		return "No runs found " + scope.where()
	}
	return "No " + status.Label + " runs " + scope.where()
}

type runGetStage int

const (
	runGetStageRunSelect runGetStage = iota
	runGetStageLoadingRuns
	runGetStageLoadingWorkflows
	runGetStageWorkflowSelect
	runGetStageLoadingJobs
	runGetStageJobSelect
	runGetStageLoadingExecutions
	runGetStageExecutionSelect
	runGetStageStepSelect
	runGetStageLoadingStep
	runGetStageStepPager
	runGetStageLoadingTests
	runGetStageTestSelect
	runGetStageHelp
	runGetStageDone
)

// RunGetFlowModel is a single multi-stage bubbletea program that drives the
// interactive "circleci run get" flow by composing components.SelectModel and a
// spinner:
//
//  1. Pick a run from the recent list.
//  2. Pick a workflow, or "see all workflows" (→ RunGetActionShowRun).
//  3. Pick a job, or "all jobs in workflow" (→ RunGetActionShowWorkflow).
//  4. For a job with parallelism > 1, pick an execution (skipped otherwise).
//  5. Pick a step, or one of three summaries — "job report" (→ RunGetActionShowJob),
//     the full per-step output report (→ RunGetActionShowJobOutput), or "failed
//     tests", which opens a further picker of the job's failed tests. The cursor
//     starts on the first failed step. Picking a step opens its output in an
//     in-flow pager (r refreshes, esc returns to the step picker) rather than
//     ending the program.
//  6. From the failed-tests picker, picking a test opens its message in the same
//     pager (esc returns to the test picker).
//
// Between selections the next level's items are fetched off the Update loop via
// a tea.Cmd, with a spinner shown meanwhile. esc moves back one step (on the
// first picker it quits); ctrl+c quits anywhere. After Run() returns, read the
// outcome with Result(); the caller then prints the corresponding summary.
type RunGetFlowModel struct {
	ctx  context.Context
	opts RunGetFlowOptions

	stage runGetStage

	runSelect       components.SelectModel
	workflowSelect  components.SelectModel
	jobSelect       components.SelectModel
	executionSelect components.SelectModel
	stepSelect      components.SelectModel
	testSelect      components.SelectModel

	spin         spinner.Model
	loadingLabel string

	// width and height are the latest terminal size. height is passed to each
	// picker so long lists scroll instead of overflowing; both size the step
	// output pager.
	width  int
	height int

	// Remembered cursors so moving back redisplays a picker where it was left.
	runCursor       int
	workflowCursor  int
	jobCursor       int
	executionCursor int

	// runs is the run list currently shown by the first picker. It starts as
	// opts.Runs (the CurrentBranch runs) and is replaced when the user cycles to
	// another scope (shift+tab) or status filter (s); it is empty when the active
	// scope+status has no runs (the picker then shows an empty-state placeholder).
	// scopes is the ordered cycle of scopes (current branch, default branch, all
	// branches, and optionally "my runs"); activeScopeIdx is the index into scopes
	// of the scope runs currently holds. statusFilters is the status-filter cycle
	// (an "all statuses" entry followed by opts.StatusFilters); statusIdx is the
	// index into it of the active filter (0 = all statuses).
	runs           []RunGetItem
	scopes         []runScope
	activeScopeIdx int
	statusFilters  []RunStatusFilter
	statusIdx      int

	// Fetched data for the current selections, parallel to the pickers (the
	// workflow/job/step pickers are offset by their leading summary options).
	workflows  []RunGetItem
	jobs       []RunGetItem
	executions []RunGetExecution
	steps      []RunGetStepItem
	tests      []RunGetTestItem

	// testCursor remembers the failed-test picker's cursor of the test opened in
	// the pager, so returning (esc) resumes on it. testReturnStage records which
	// picker offered the "failed tests" option (execution or step), so esc from
	// the test picker returns there.
	testCursor      int
	testReturnStage runGetStage

	// runErrors are the selected run's config/setup errors, captured when the run
	// is picked and rendered under the workflow picker's title.
	runErrors []RunGetError

	runID      uuid.UUID
	workflowID uuid.UUID
	jobID      uuid.UUID
	execution  int // chosen parallel execution index (0 when single-execution)
	stepNum    int // chosen step number, for the output pager
	// stepCursor remembers the step picker's cursor index of the step opened in
	// the pager, so returning from the pager (esc) resumes on it. -1 means "no
	// remembered step" — a fresh entry into the step picker defaults to the first
	// failed step instead.
	stepCursor int

	// restoreStep is set when an executions re-fetch was triggered by refreshing
	// the step picker; onExecutions then re-enters the same execution's steps
	// rather than routing back through the execution picker.
	restoreStep bool

	// Step output pager state. pager (a components.PagerModel) scrolls the selected
	// step's output, streamed raw (ANSI intact) so colors survive, and provides the
	// less-style "/" search. pagerBuf accumulates stdout (then stderr); pagerOffset
	// is the next stdout byte to request; pagerTerminal marks stdout finished;
	// pagerStderrDone marks stderr appended. pagerFetching guards against
	// overlapping fetches; pagerEpoch invalidates polls/fetches from a superseded
	// stream (e.g. when leaving the pager).
	pager           components.PagerModel
	pagerBuf        []byte
	pagerOffset     int64
	pagerTerminal   bool
	pagerStderrDone bool
	pagerFetching   bool
	pagerEpoch      int
	// pagerReturnStage is the picker esc returns to from the pager: the step
	// picker for streamed step output, or the failed-test picker for a test
	// message.
	pagerReturnStage runGetStage

	// help is the "?" keyboard-shortcut overlay (a scrollable markdown frame);
	// helpReturnStage is the picker esc/q returns to when it is dismissed. help is
	// the zero value (unusable) when RenderMarkdown is nil, in which case "?" is
	// inert.
	help            components.HelpModel
	helpReturnStage runGetStage

	result RunGetResult
}

// async message types carrying fetch results back into the Update loop.
type (
	runGetRunsMsg struct {
		items     []RunGetItem
		scopeIdx  int
		statusIdx int
		err       error
	}
	runGetWorkflowsMsg struct {
		items []RunGetItem
		err   error
	}
	runGetJobsMsg struct {
		items []RunGetItem
		err   error
	}
	runGetExecutionsMsg struct {
		items []RunGetExecution
		err   error
	}
	runGetTestsMsg struct {
		items []RunGetTestItem
		err   error
	}
	runGetStepStdoutMsg struct {
		epoch    int
		data     []byte
		terminal bool
		err      error
	}
	runGetStepStderrMsg struct {
		epoch int
		data  []byte
		err   error
	}
	runGetStepPollMsg struct {
		epoch int
	}
)

// NewRunGetFlow returns a RunGetFlowModel ready to pass to tea.NewProgram.
func NewRunGetFlow(ctx context.Context, opts RunGetFlowOptions) RunGetFlowModel {
	m := RunGetFlowModel{
		ctx:           ctx,
		opts:          opts,
		stage:         runGetStageRunSelect,
		spin:          components.NewSpinner(opts.Color),
		pager:         components.NewPager().WithKeys(stepPagerKeys...),
		runs:          opts.Runs,
		scopes:        buildRunScopes(opts.CurrentBranch, opts.DefaultBranch, opts.FetchMyRuns != nil),
		statusFilters: buildStatusCycle(opts.StatusFilters),
		// activeScopeIdx and statusIdx start at 0: the current branch is always the
		// first scope, and "all statuses" the first filter.
		stepCursor: -1,
	}
	if opts.RenderMarkdown != nil {
		m.help = components.NewHelp(func(w int) string {
			return opts.RenderMarkdown(runGetHelpMarkdown, w)
		})
	}
	m.runSelect = m.newRunSelect()
	return m
}

// Result returns the final outcome. Only valid after tea.Program.Run() returns.
func (m RunGetFlowModel) Result() RunGetResult { return m.result }

func (m RunGetFlowModel) Init() tea.Cmd { return nil }

func (m RunGetFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Remember the terminal size for pickers built on later stages and for the
	// pager, and let it fall through to the active stage (below) so a live resize
	// re-windows it.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Only animate while a fetch is in flight; otherwise let the spinner
		// stop ticking so selection stages don't repaint needlessly.
		if m.loading() {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	case runGetRunsMsg:
		return m.onRuns(msg)
	case runGetWorkflowsMsg:
		return m.onWorkflows(msg)
	case runGetJobsMsg:
		return m.onJobs(msg)
	case runGetExecutionsMsg:
		return m.onExecutions(msg)
	case runGetTestsMsg:
		return m.onTests(msg)
	case runGetStepStdoutMsg:
		return m.onStepStdout(msg)
	case runGetStepStderrMsg:
		return m.onStepStderr(msg)
	case runGetStepPollMsg:
		return m.onStepPoll(msg)
	}

	switch m.stage {
	case runGetStageRunSelect:
		return m.updateRunSelect(msg)
	case runGetStageWorkflowSelect:
		return m.updateWorkflowSelect(msg)
	case runGetStageJobSelect:
		return m.updateJobSelect(msg)
	case runGetStageExecutionSelect:
		return m.updateExecutionSelect(msg)
	case runGetStageStepSelect:
		return m.updateStepSelect(msg)
	case runGetStageTestSelect:
		return m.updateTestSelect(msg)
	case runGetStageStepPager:
		return m.updateStepPager(msg)
	case runGetStageHelp:
		return m.updateHelp(msg)
	case runGetStageLoadingRuns, runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingExecutions, runGetStageLoadingStep, runGetStageLoadingTests:
		// ctrl+c can still abort while a fetch is in flight.
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeyCtrlC) {
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		}
	case runGetStageDone:
		// Quitting; no further input is processed.
	}
	return m, nil
}

// loadingCmd pairs a fetch command with the spinner tick so the loading screen
// animates while it runs. When animation is disabled (CIRCLE_SPINNER_DISABLED /
// non-interactive) the tick is omitted and the loading line stays static.
func (m RunGetFlowModel) loadingCmd(fetch tea.Cmd) tea.Cmd {
	if m.opts.Animate {
		return tea.Batch(m.spin.Tick, fetch)
	}
	return fetch
}

func (m RunGetFlowModel) loading() bool {
	return m.stage == runGetStageLoadingRuns ||
		m.stage == runGetStageLoadingWorkflows ||
		m.stage == runGetStageLoadingJobs ||
		m.stage == runGetStageLoadingExecutions ||
		m.stage == runGetStageLoadingStep ||
		m.stage == runGetStageLoadingTests
}

// quit records the result and switches to the done stage, whose empty View
// clears the final picker from the screen before the program exits. Without
// this the last picker's collapsed line would linger above the printed summary
// (intermediate pickers are already overwritten by the next stage's frame).
func (m RunGetFlowModel) quit(res RunGetResult) (tea.Model, tea.Cmd) {
	m.result = res
	m.stage = runGetStageDone
	return m, tea.Quit
}

// updateRunSelect handles the first picker. esc and ctrl+c both quit, since
// there is no earlier step to return to.
func (m RunGetFlowModel) updateRunSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC, components.KeyEsc):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, switchScopeBinding):
			if len(m.scopes) >= 2 {
				next := (m.activeScopeIdx + 1) % len(m.scopes)
				m.stage = runGetStageLoadingRuns
				m.loadingLabel = "Fetching runs for " + m.scopes[next].label
				return m, m.loadingCmd(m.cmdFetchRuns(next, m.statusIdx))
			}
			return m, nil
		case key.Matches(k, components.BindStatus):
			if m.statusFilterEnabled() {
				return m.fetchStatus((m.statusIdx + 1) % len(m.statusFilters))
			}
			return m, nil
		case key.Matches(k, components.KeyStatusClear):
			// Jump straight back to "all statuses" (index 0); a no-op when already
			// there.
			if m.statusFilterEnabled() && m.statusIdx != 0 {
				return m.fetchStatus(0)
			}
			return m, nil
		case key.Matches(k, components.BindRefresh):
			m.stage = runGetStageLoadingRuns
			m.loadingLabel = "Refreshing runs"
			return m, m.loadingCmd(m.cmdFetchRuns(m.activeScopeIdx, m.statusIdx))
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageRunSelect)
		}
	}

	updated, cmd := m.runSelect.Update(msg)
	m.runSelect = updated.(components.SelectModel)
	if !m.runSelect.Done() {
		return m, cmd
	}

	if len(m.runs) == 0 {
		// The empty-state placeholder is the only row; there is nothing to open.
		// Rebuild the picker to clear its "chosen" flag and stay put.
		m.runSelect = m.newRunSelect()
		return m, nil
	}

	m.runCursor = m.runSelect.Selected()
	m.runID = m.runs[m.runCursor].ID
	m.runErrors = m.runs[m.runCursor].Errors
	m.stage = runGetStageLoadingWorkflows
	m.loadingLabel = "Fetching workflows"
	return m, m.loadingCmd(m.cmdFetchWorkflows())
}

// onRuns handles a completed re-fetch (a shift+tab scope toggle, an "s" status
// change, or an "r" refresh). On error it quits; otherwise it swaps in the new
// runs and commits the scope and status that produced them, with the cursor
// reset. An empty result is committed too (the picker shows an empty-state
// placeholder) so cycling never gets stuck — the next toggle advances from the
// just-committed filter rather than retrying the same empty one.
func (m RunGetFlowModel) onRuns(msg runGetRunsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.runs = msg.items
	m.activeScopeIdx = msg.scopeIdx
	m.statusIdx = msg.statusIdx
	m.runCursor = 0
	m.runSelect = m.newRunSelect()
	m.stage = runGetStageRunSelect
	return m, nil
}

// activeScope is the scope whose runs are currently shown.
func (m RunGetFlowModel) activeScope() runScope {
	return m.scopes[m.activeScopeIdx]
}

// statusFilterEnabled reports whether the "s" status-filter action is available:
// true only when the caller supplied at least one selectable status (the cycle
// then holds the "all statuses" entry plus those).
func (m RunGetFlowModel) statusFilterEnabled() bool {
	return len(m.statusFilters) > 1
}

// fetchStatus begins loading the active scope's runs under the status filter at
// idx, with a loading label naming the status ("all statuses" reads as plain
// "Fetching runs").
func (m RunGetFlowModel) fetchStatus(idx int) (tea.Model, tea.Cmd) {
	m.stage = runGetStageLoadingRuns
	if s := m.statusFilters[idx]; s.Value != "" {
		m.loadingLabel = "Fetching " + s.Label + " runs"
	} else {
		m.loadingLabel = "Fetching runs"
	}
	return m, m.loadingCmd(m.cmdFetchRuns(m.activeScopeIdx, idx))
}

func (m RunGetFlowModel) onWorkflows(msg runGetWorkflowsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.workflows = msg.items
	m.workflowCursor = 0
	m.workflowSelect = m.newWorkflowSelect()
	m.stage = runGetStageWorkflowSelect
	return m, nil
}

// updateWorkflowSelect handles the second picker. esc returns to the run
// picker; ctrl+c quits. Selecting the leading option shows the whole run.
func (m RunGetFlowModel) updateWorkflowSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			m.runSelect = m.newRunSelect()
			m.stage = runGetStageRunSelect
			return m, nil
		case key.Matches(k, components.BindRefresh):
			m.stage = runGetStageLoadingWorkflows
			m.loadingLabel = "Refreshing workflows"
			return m, m.loadingCmd(m.cmdFetchWorkflows())
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageWorkflowSelect)
		}
	}

	updated, cmd := m.workflowSelect.Update(msg)
	m.workflowSelect = updated.(components.SelectModel)
	if !m.workflowSelect.Done() {
		return m, cmd
	}

	sel := m.workflowSelect.Selected()
	if sel == 0 { // "see all workflows"
		return m.quit(RunGetResult{Action: RunGetActionShowRun, RunID: m.runID})
	}
	m.workflowCursor = sel
	m.workflowID = m.workflows[sel-1].ID
	m.stage = runGetStageLoadingJobs
	m.loadingLabel = "Fetching jobs"
	return m, m.loadingCmd(m.cmdFetchJobs())
}

func (m RunGetFlowModel) onJobs(msg runGetJobsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.jobs = msg.items
	m.jobSelect = m.newJobSelect()
	m.stage = runGetStageJobSelect
	return m, nil
}

// updateJobSelect handles the job picker. esc returns to the workflow picker;
// ctrl+c quits. The leading option shows the whole workflow; picking a job
// advances to the step picker.
func (m RunGetFlowModel) updateJobSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			m.workflowSelect = m.newWorkflowSelect()
			m.stage = runGetStageWorkflowSelect
			return m, nil
		case key.Matches(k, components.BindRefresh):
			m.stage = runGetStageLoadingJobs
			m.loadingLabel = "Refreshing jobs"
			return m, m.loadingCmd(m.cmdFetchJobs())
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageJobSelect)
		}
	}

	updated, cmd := m.jobSelect.Update(msg)
	m.jobSelect = updated.(components.SelectModel)
	if !m.jobSelect.Done() {
		return m, cmd
	}

	sel := m.jobSelect.Selected()
	if sel == 0 { // "all jobs in workflow"
		return m.quit(RunGetResult{Action: RunGetActionShowWorkflow, WorkflowID: m.workflowID})
	}
	m.jobCursor = sel
	m.jobID = m.jobs[sel-1].ID
	m.stage = runGetStageLoadingExecutions
	m.loadingLabel = "Fetching steps"
	return m, m.loadingCmd(m.cmdFetchExecutions())
}

func (m RunGetFlowModel) onExecutions(msg runGetExecutionsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.executions = msg.items

	// Refreshing the step picker re-enters the execution the user was viewing,
	// if it still exists, rather than routing back through the execution picker.
	if m.restoreStep {
		m.restoreStep = false
		for _, exec := range m.executions {
			if exec.Index == m.execution {
				return m.enterStepSelect(exec), nil
			}
		}
	}

	switch len(m.executions) {
	case 0:
		// No resolvable steps — skip straight to the job report rather than show
		// an empty picker.
		return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
	case 1:
		// Single execution: no execution picker, go straight to its steps.
		return m.enterStepSelect(m.executions[0]), nil
	default:
		m.executionCursor = m.firstFailedExecutionCursor()
		m.executionSelect = m.newExecutionSelect()
		m.stage = runGetStageExecutionSelect
		return m, nil
	}
}

// enterStepSelect scopes the step picker to one execution and shows it. This is
// a fresh entry (from the execution picker or a single-execution job), so the
// cursor defaults to the first failed step rather than a remembered one.
func (m RunGetFlowModel) enterStepSelect(exec RunGetExecution) RunGetFlowModel {
	m.execution = exec.Index
	m.steps = exec.Steps
	m.stepCursor = -1
	m.stepSelect = m.newStepSelect()
	m.stage = runGetStageStepSelect
	return m
}

// enterFailedTests begins loading the job's failed tests for the further test
// picker, remembering which picker (returnStage) offered the option so esc from
// the test picker routes back there.
func (m RunGetFlowModel) enterFailedTests(returnStage runGetStage) (tea.Model, tea.Cmd) {
	m.testReturnStage = returnStage
	m.testCursor = 0
	m.stage = runGetStageLoadingTests
	m.loadingLabel = "Fetching failed tests"
	return m, m.loadingCmd(m.cmdFetchTests())
}

func (m RunGetFlowModel) onTests(msg runGetTestsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.tests = msg.items
	m.testCursor = 0
	m.testSelect = m.newTestSelect()
	m.stage = runGetStageTestSelect
	return m, nil
}

// updateTestSelect handles the failed-test picker. esc returns to the picker
// that offered the option (execution or step); ctrl+c quits; r re-fetches.
// Picking a test opens its message in the pager. When the job recorded no failed
// tests the picker shows a single placeholder row that simply goes back.
func (m RunGetFlowModel) updateTestSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			return m.returnFromTestSelect(), nil
		case key.Matches(k, components.BindRefresh):
			m.stage = runGetStageLoadingTests
			m.loadingLabel = "Refreshing failed tests"
			return m, m.loadingCmd(m.cmdFetchTests())
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageTestSelect)
		}
	}

	updated, cmd := m.testSelect.Update(msg)
	m.testSelect = updated.(components.SelectModel)
	if !m.testSelect.Done() {
		return m, cmd
	}

	if len(m.tests) == 0 {
		// The placeholder row is the only entry; there is nothing to open.
		return m.returnFromTestSelect(), nil
	}
	sel := m.testSelect.Selected()
	m.testCursor = sel
	m.openTestMessage(m.tests[sel].Message)
	return m, nil
}

// returnFromTestSelect rebuilds and shows whichever picker offered the failed-
// tests option: the execution picker for a parallel job, else the step picker.
func (m RunGetFlowModel) returnFromTestSelect() RunGetFlowModel {
	if m.testReturnStage == runGetStageExecutionSelect {
		m.executionSelect = m.newExecutionSelect()
		m.stage = runGetStageExecutionSelect
	} else {
		m.stepSelect = m.newStepSelect()
		m.stage = runGetStageStepSelect
	}
	return m
}

// openTestMessage loads a test's message into the pager as static content (no
// streaming) and shows it from the top, with esc set to return to the test
// picker. The message is shown raw so any ANSI colors survive, matching the
// step-output pager.
func (m *RunGetFlowModel) openTestMessage(message string) {
	m.pagerEpoch++
	m.pagerFetching = false
	m.pagerTerminal = true
	m.pagerStderrDone = true
	m.pagerBuf = []byte(message)
	m.pagerOffset = int64(len(message))
	m.pagerReturnStage = runGetStageTestSelect
	// Fresh content: drop any search carried over from a previously viewed step,
	// then show it from the top.
	m.pager = m.pager.ResetSearch()
	m.syncPager()
	m.pager = m.pager.GotoTop()
	m.stage = runGetStageStepPager
}

// updateExecutionSelect handles the execution picker, shown only when a job has
// parallelism > 1. Its leading options are the job summaries; the remaining
// rows are executions. esc returns to the job picker; ctrl+c quits.
func (m RunGetFlowModel) updateExecutionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			m.jobSelect = m.newJobSelect()
			m.stage = runGetStageJobSelect
			return m, nil
		case key.Matches(k, components.BindRefresh):
			m.stage = runGetStageLoadingExecutions
			m.loadingLabel = "Refreshing steps"
			return m, m.loadingCmd(m.cmdFetchExecutions())
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageExecutionSelect)
		}
	}

	updated, cmd := m.executionSelect.Update(msg)
	m.executionSelect = updated.(components.SelectModel)
	if !m.executionSelect.Done() {
		return m, cmd
	}

	sel := m.executionSelect.Selected()
	switch sel {
	case 0: // "job report" — short summary, all executions
		return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
	case 1: // "full output report" — every step's output, all executions
		return m.quit(RunGetResult{Action: RunGetActionShowJobOutput, JobID: m.jobID})
	case 2: // "failed tests" — open the failed-test picker
		return m.enterFailedTests(runGetStageExecutionSelect)
	}
	m.executionCursor = sel
	return m.enterStepSelect(m.executions[sel-runGetMetaCount]), nil
}

// updateStepSelect handles the step picker. esc returns to the execution picker
// when there was one, else to the job picker; ctrl+c quits. The job-summary
// options lead the picker only for a single-execution job; for a parallel job
// they live on the execution picker instead, so here it is steps only.
func (m RunGetFlowModel) updateStepSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			if len(m.executions) > 1 {
				m.executionSelect = m.newExecutionSelect()
				m.stage = runGetStageExecutionSelect
			} else {
				m.jobSelect = m.newJobSelect()
				m.stage = runGetStageJobSelect
			}
			return m, nil
		case key.Matches(k, components.BindRefresh):
			// Steps come from the job's executions; re-fetch them and re-enter the
			// current execution's steps (restoreStep) instead of bouncing back to
			// the execution picker.
			m.restoreStep = true
			m.stage = runGetStageLoadingExecutions
			m.loadingLabel = "Refreshing steps"
			return m, m.loadingCmd(m.cmdFetchExecutions())
		case key.Matches(k, components.BindHelp):
			return m.openHelp(runGetStageStepSelect)
		}
	}

	updated, cmd := m.stepSelect.Update(msg)
	m.stepSelect = updated.(components.SelectModel)
	if !m.stepSelect.Done() {
		return m, cmd
	}

	picked := m.stepSelect.Selected()
	sel := picked
	if meta := m.stepMetaCount(); meta > 0 {
		switch sel {
		case 0: // "job report" — short summary
			return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
		case 1: // "full output report" — every step's output
			return m.quit(RunGetResult{Action: RunGetActionShowJobOutput, JobID: m.jobID})
		case 2: // "failed tests" — open the failed-test picker
			return m.enterFailedTests(runGetStageStepSelect)
		}
		sel -= meta
	}
	// A chosen step streams its output into the pager (rather than quitting the
	// flow), so esc can return here. Remember the picker index so esc resumes on
	// this step, then start the stream.
	step := m.steps[sel]
	m.execution = step.Execution
	m.stepNum = step.StepNum
	m.stepCursor = picked
	m.pagerReturnStage = runGetStageStepSelect
	m.stage = runGetStageLoadingStep
	m.loadingLabel = "Fetching step output"
	return m, m.loadingCmd(m.startStepStream())
}

// startStepStream resets the pager buffers and begins a new stdout stream from
// offset 0 under a fresh epoch (which invalidates any in-flight fetch or pending
// poll from a previous stream). It returns the first fetch command.
func (m *RunGetFlowModel) startStepStream() tea.Cmd {
	m.pagerEpoch++
	m.pagerBuf = nil
	m.pagerOffset = 0
	m.pagerTerminal = false
	m.pagerStderrDone = false
	m.pagerFetching = true
	// Fresh stream: drop any search carried over from a previously viewed step.
	m.pager = m.pager.ResetSearch()
	return m.cmdFetchStepStdout(m.pagerEpoch)
}

// onStepStdout appends a stdout chunk to the buffer and refreshes the pager.
// While stdout has not terminated it schedules the next 2s poll; once it has, it
// kicks off the one-shot stderr fetch. Stale chunks (from a superseded stream)
// are dropped.
func (m RunGetFlowModel) onStepStdout(msg runGetStepStdoutMsg) (tea.Model, tea.Cmd) {
	if msg.epoch != m.pagerEpoch {
		return m, nil
	}
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	m.pagerFetching = false
	m.pagerBuf = append(m.pagerBuf, msg.data...)
	m.pagerOffset += int64(len(msg.data))
	m.pagerTerminal = msg.terminal
	m.showPager()

	if !m.pagerTerminal {
		return m, m.cmdStepPoll(m.pagerEpoch)
	}
	// stdout finished; fetch stderr once and append it.
	if !m.pagerStderrDone && m.opts.FetchStepStderr != nil {
		m.pagerFetching = true
		return m, m.cmdFetchStepStderr(m.pagerEpoch)
	}
	return m, nil
}

// onStepStderr appends the step's stderr once stdout has finished. A stderr
// error is non-fatal — the stdout already shown stays — so it is ignored.
func (m RunGetFlowModel) onStepStderr(msg runGetStepStderrMsg) (tea.Model, tea.Cmd) {
	if msg.epoch != m.pagerEpoch {
		return m, nil
	}
	m.pagerFetching = false
	m.pagerStderrDone = true
	if msg.err == nil && len(msg.data) > 0 {
		m.pagerBuf = append(m.pagerBuf, msg.data...)
		m.showPager()
	}
	return m, nil
}

// onStepPoll is the 2s heartbeat: it requests the next stdout chunk from the
// current offset, unless the stream has moved on (stale epoch), a fetch is
// already running, or stdout has terminated.
func (m RunGetFlowModel) onStepPoll(msg runGetStepPollMsg) (tea.Model, tea.Cmd) {
	if msg.epoch != m.pagerEpoch || m.pagerFetching || m.pagerTerminal {
		return m, nil
	}
	m.pagerFetching = true
	return m, m.cmdFetchStepStdout(m.pagerEpoch)
}

// showPager loads the accumulated raw output into the pager. New output follows
// the bottom only when the view was already at the bottom, so a reader who has
// scrolled up is not yanked down (the pager handles that in
// SetContentFollowingTail).
func (m *RunGetFlowModel) showPager() {
	if m.width > 0 && m.height > 0 {
		m.pager = m.pager.SetSize(m.width, m.height)
	}
	m.pager = m.pager.SetContentFollowingTail(m.pagerContent())
	m.stage = runGetStageStepPager
}

// syncPager loads the accumulated output into the pager, preserving the scroll
// position. Used when the content is set once (a test message) rather than
// streamed.
func (m *RunGetFlowModel) syncPager() {
	if m.width > 0 && m.height > 0 {
		m.pager = m.pager.SetSize(m.width, m.height)
	}
	m.pager = m.pager.SetContent(m.pagerContent())
}

// pagerContent is the string shown in the pager: the accumulated raw output
// replayed through a terminal model (so carriage-return / cursor-movement
// redraws collapse to their final state) with colors preserved, or a
// placeholder while nothing has arrived yet.
//
// Raw output carries the redraws a terminal resolves in place: apt, Docker and
// friends repaint progress with "\r" and cursor moves. lipgloss, which renders
// the pager, instead treats each "\r" (and other control) as a line break, so a
// single logical line explodes into many visual rows the viewport never budgeted
// for — inflating the frame and pushing the footer off the bottom of the screen.
// termrender.RenderStyled replays the stream into the text a human would have
// seen, keeping SGR styling intact so the output stays colored.
func (m RunGetFlowModel) pagerContent() string {
	if len(m.pagerBuf) == 0 {
		return theme.HelperStyle.Render("(no output yet)")
	}
	var buf strings.Builder
	if err := termrender.RenderStyled(&buf, bytes.NewReader(m.pagerBuf)); err != nil {
		return string(m.pagerBuf) // fall back to the raw bytes on a render error
	}
	return buf.String()
}

// updateStepPager drives the step-output pager. Scrolling, g/G jump-to-ends and
// the whole "/" search interaction are handled by the pager component; this
// method binds only the lifecycle keys the pager leaves alone: ctrl+c quits, and
// esc dismisses an active search or (with none) returns to the picker that opened
// the pager. Output streams in on its own (polled until terminal), so there is no
// reload key.
func (m RunGetFlowModel) updateStepPager(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Guard on Searching so esc/ctrl+c reach the "/" prompt (cancel) rather than
	// acting as lifecycle keys while a pattern is being typed.
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.pager.Searching() {
		switch {
		case key.Matches(k, components.KeyCtrlC):
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case key.Matches(k, components.KeyEsc):
			if m.pager.SearchActive() {
				m.pager = m.pager.ClearSearch()
				return m, nil
			}
			// Leave the pager: bump the epoch so any in-flight fetch or pending
			// poll is ignored, then return to whichever picker opened it. The
			// remembered cursor (stepCursor / testCursor) resumes on the opened row.
			m.pagerEpoch++
			m.pagerFetching = false
			if m.pagerReturnStage == runGetStageTestSelect {
				m.testSelect = m.newTestSelect()
				m.stage = runGetStageTestSelect
			} else {
				m.stepSelect = m.newStepSelect()
				m.stage = runGetStageStepSelect
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.pager, cmd = m.pager.Update(msg)
	return m, cmd
}

// helpEnabled reports whether the "?" keyboard-shortcut overlay is available:
// true only when the caller supplied a markdown renderer.
func (m RunGetFlowModel) helpEnabled() bool { return m.opts.RenderMarkdown != nil }

// openHelp shows the keyboard-shortcut overlay, remembering the picker
// (returnStage) esc/q returns to. It is a no-op when no renderer was supplied.
// The overlay is sized to the current terminal and reset (search dropped,
// scrolled to top) so a re-open starts clean.
func (m RunGetFlowModel) openHelp(returnStage runGetStage) (tea.Model, tea.Cmd) {
	if !m.helpEnabled() {
		return m, nil
	}
	m.helpReturnStage = returnStage
	if m.width > 0 && m.height > 0 {
		m.help = m.help.SetSize(m.width, m.height)
	}
	m.help = m.help.Reopen()
	m.stage = runGetStageHelp
	return m, nil
}

// updateHelp drives the help overlay. Scrolling and the "/" search are handled by
// the help component; esc/q (handled there too) dismiss it, at which point the
// flow routes back to whichever picker opened it. ctrl+c still quits the whole
// program, unless the "/" search prompt is capturing input.
func (m RunGetFlowModel) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeyCtrlC) && !m.help.Searching() {
		return m.quit(RunGetResult{Action: RunGetActionCancel})
	}
	var cmd tea.Cmd
	m.help, cmd = m.help.Update(msg)
	if m.help.Dismissed() {
		m.stage = m.helpReturnStage
	}
	return m, cmd
}

func (m RunGetFlowModel) View() tea.View {
	switch m.stage {
	case runGetStageRunSelect:
		return m.runSelect.View()
	case runGetStageWorkflowSelect:
		return m.workflowSelect.View()
	case runGetStageJobSelect:
		return m.jobSelect.View()
	case runGetStageExecutionSelect:
		return m.executionSelect.View()
	case runGetStageStepSelect:
		return m.stepSelect.View()
	case runGetStageTestSelect:
		return m.testSelect.View()
	case runGetStageLoadingRuns, runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingExecutions, runGetStageLoadingStep, runGetStageLoadingTests:
		label := theme.HelperStyle.Render(m.loadingLabel)
		if m.opts.Animate {
			label = m.spin.View() + " " + label
		}
		return tea.NewView(label)
	case runGetStageStepPager:
		return m.stepPagerView()
	case runGetStageHelp:
		return m.help.View()
	case runGetStageDone:
		// Empty final frame so the last picker is cleared before the program
		// exits and the summary prints in its place.
		return tea.NewView("")
	}
	return tea.NewView("")
}

// stepPagerView renders the step-output pager via the pager component (a
// scrollable viewport above a footer of scroll position, search state and key
// hints, on the alternate screen). While output is still streaming a "streaming…"
// indicator leads the footer.
func (m RunGetFlowModel) stepPagerView() tea.View {
	status := ""
	if !m.pagerTerminal {
		status = theme.AccentStyle.Render("streaming…") + "  "
	}
	return m.pager.View(status)
}

// --- picker builders ---

func (m RunGetFlowModel) newRunSelect() components.SelectModel {
	// Name the active scope and status filter in the title so it is clear which
	// runs are listed (e.g. "Select a run [main]", "… [all branches · failed]",
	// or "… [failed]" when there is only one scope). The bracket matches the
	// per-row "[branch]" labels and is colored so it stands out from the rest of
	// the (bold, uncolored) title.
	prompt := "Select a run"
	var parts []string
	if len(m.scopes) >= 2 {
		parts = append(parts, m.activeScope().titleName())
	}
	if status := m.statusFilters[m.statusIdx]; status.Value != "" {
		parts = append(parts, status.Label)
	}
	if len(parts) > 0 {
		scope := "[" + strings.Join(parts, " · ") + "]"
		if m.opts.Color {
			scope = theme.SecondaryStyle.Render(scope)
		}
		prompt = "Select a run " + scope
	}
	// A scope+status filter can match nothing; rather than a blank picker, show a
	// single placeholder row explaining the empty result (enter on it is a no-op).
	labels, icons := itemLabels(m.runs), m.itemIcons(m.runs)
	if len(m.runs) == 0 {
		labels = []string{"(" + runsEmptyNote(m.activeScope(), m.statusFilters[m.statusIdx]) + ")"}
		icons = []string{m.metaIcon()}
	}
	return components.NewSelectModel(prompt, labels).
		WithIcons(icons).
		WithCursor(m.runCursor).
		WithKeys(m.runSelectKeys()...).
		WithHeight(m.height)
}

// runSelectKeys is the footer for the run picker. The first picker quits on esc,
// so it advertises the refresh, status filter ("s") and — when a scope toggle is
// available — the trigger-switch shortcut (shift+tab, or Tab on Windows; the
// active scope and status are named in the title, not here).
func (m RunGetFlowModel) runSelectKeys() []key.Binding {
	keys := []key.Binding{components.BindMove, components.BindSelect, components.BindRefresh}
	if len(m.scopes) >= 2 {
		keys = append(keys, switchScopeBinding)
	}
	if m.statusFilterEnabled() {
		keys = append(keys, components.BindStatus)
	}
	if m.helpEnabled() {
		keys = append(keys, components.BindHelp)
	}
	keys = append(keys, components.BindQuitEsc)
	return keys
}

// backKeys is the footer for the workflow, job, execution, step and test
// pickers, where esc goes back a step. It advertises the "?" help overlay when a
// markdown renderer was supplied.
func (m RunGetFlowModel) backKeys() []key.Binding {
	keys := []key.Binding{components.BindMove, components.BindSelect, components.BindRefresh, components.BindBack}
	if m.helpEnabled() {
		keys = append(keys, components.BindHelp)
	}
	return keys
}

func (m RunGetFlowModel) newWorkflowSelect() components.SelectModel {
	labels := append([]string{runGetAllWorkflowsLabel}, itemLabels(m.workflows)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.workflows)...)
	return components.NewSelectModel("Select a workflow", labels).
		WithIcons(icons).
		WithNote(m.runErrorNote()).
		WithCursor(m.workflowCursor).
		WithKeys(m.backKeys()...).
		WithHeight(m.height)
}

// runErrorNote formats the selected run's errors for display under the workflow
// picker title: one "<type>: <message>" line per error, wrapped to the terminal
// width and tinted with the warning color when color is enabled. It is empty
// when the run had no errors, so the picker renders as before.
func (m RunGetFlowModel) runErrorNote() string {
	if len(m.runErrors) == 0 {
		return ""
	}
	lines := make([]string, len(m.runErrors))
	for i, e := range m.runErrors {
		line := e.Message
		if e.Type != "" {
			line = e.Type + ": " + e.Message
		}
		lines[i] = line
	}
	note := strings.Join(lines, "\n")
	style := theme.WarningStyle
	if !m.opts.Color {
		style = lipgloss.NewStyle()
	}
	// Wrap to the terminal width (leaving a small margin) so a long config error
	// spans multiple lines the picker can account for, rather than overflowing.
	if m.width > 4 {
		style = style.Width(m.width - 2)
	}
	return style.Render(note)
}

func (m RunGetFlowModel) newJobSelect() components.SelectModel {
	labels := append([]string{runGetAllJobsLabel}, itemLabels(m.jobs)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.jobs)...)
	return components.NewSelectModel("Select a job", labels).
		WithIcons(icons).
		WithCursor(m.jobCursor).
		WithKeys(m.backKeys()...).
		WithHeight(m.height)
}

func (m RunGetFlowModel) newExecutionSelect() components.SelectModel {
	labels := make([]string, 0, len(m.executions)+runGetMetaCount)
	icons := make([]string, 0, len(m.executions)+runGetMetaCount)
	labels = append(labels, runGetJobReportLabel, runGetJobOutputLabel, runGetFailedTestsLabel)
	icons = append(icons, m.metaIcon(), m.metaIcon(), m.failedTestsIcon())
	for _, e := range m.executions {
		labels = append(labels, e.Label)
		icons = append(icons, colorizeStatusIcon(e.Icon, m.opts.Color))
	}
	return components.NewSelectModel("Select an execution", labels).
		WithIcons(icons).
		WithCursor(m.executionCursor).
		WithKeys(m.backKeys()...).
		WithHeight(m.height)
}

// firstFailedExecutionCursor returns the picker index of the first failed/errored
// execution (offset past the leading summary options), so the cursor lands on
// the likely target. Falls back to the first summary option when none failed.
func (m RunGetFlowModel) firstFailedExecutionCursor() int {
	for i, e := range m.executions {
		if e.Icon == "✗" || e.Icon == "!" {
			return i + runGetMetaCount
		}
	}
	return 0
}

func (m RunGetFlowModel) newStepSelect() components.SelectModel {
	meta := m.stepMetaCount()
	labels := make([]string, 0, len(m.steps)+meta)
	icons := make([]string, 0, len(m.steps)+meta)
	if meta > 0 {
		labels = append(labels, runGetJobReportLabel, runGetJobOutputLabel, runGetFailedTestsLabel)
		icons = append(icons, m.metaIcon(), m.metaIcon(), m.failedTestsIcon())
	}
	for _, s := range m.steps {
		labels = append(labels, s.Label)
		icons = append(icons, colorizeStatusIcon(s.Icon, m.opts.Color))
	}
	// Resume on the remembered step (set when one was opened in the pager);
	// otherwise land on the first failed step.
	cursor := m.stepCursor
	if cursor < 0 {
		cursor = m.firstFailedStepCursor()
	}
	return components.NewSelectModel("Select a step", labels).
		WithIcons(icons).
		WithCursor(cursor).
		WithKeys(m.backKeys()...).
		WithHeight(m.height)
}

// newTestSelect builds the failed-test picker. With no failed tests it shows a
// single placeholder row so the picker still renders (and esc/enter go back).
func (m RunGetFlowModel) newTestSelect() components.SelectModel {
	var labels, icons []string
	if len(m.tests) == 0 {
		labels = []string{"(no failed tests recorded)"}
		icons = []string{m.metaIcon()}
	} else {
		labels = make([]string, len(m.tests))
		icons = make([]string, len(m.tests))
		for i, t := range m.tests {
			labels[i] = t.Label
			icons[i] = colorizeStatusIcon(t.Icon, m.opts.Color)
		}
	}
	return components.NewSelectModel("Select a failed test", labels).
		WithIcons(icons).
		WithCursor(m.testCursor).
		WithKeys(m.backKeys()...).
		WithHeight(m.height)
}

// stepMetaCount is how many leading job-summary options the step picker carries:
// the two summaries when it is the first picker after the job (single
// execution), or zero when an execution picker already hosted them.
func (m RunGetFlowModel) stepMetaCount() int {
	if len(m.executions) > 1 {
		return 0
	}
	return runGetMetaCount
}

// firstFailedStepCursor returns the picker index of the first failed/errored
// step (offset past any leading summary options), so the cursor lands on the
// likely target. Falls back to the first option when none failed.
func (m RunGetFlowModel) firstFailedStepCursor() int {
	off := m.stepMetaCount()
	for i, s := range m.steps {
		if s.Icon == "✗" || s.Icon == "!" {
			return i + off
		}
	}
	return 0
}

// metaIcon is the glyph for the leading summary option, dimmed when color is on
// so it stays distinct from the status icons on the rows below.
func (m RunGetFlowModel) metaIcon() string {
	return m.mutedGlyph(runGetMetaGlyph)
}

// failedTestsIcon is the glyph for the "failed tests" summary option: a muted
// question mark, distinct from the plain list mark on the other summaries but
// still state-neutral (the option appears whether or not any test failed).
func (m RunGetFlowModel) failedTestsIcon() string {
	return m.mutedGlyph(runGetFailedTestsGlyph)
}

// mutedGlyph dims a summary-option glyph when color is on so it stays distinct
// from the status icons on the rows below.
func (m RunGetFlowModel) mutedGlyph(glyph string) string {
	if m.opts.Color {
		return theme.HelperStyle.Render(glyph)
	}
	return glyph
}

func itemLabels(items []RunGetItem) []string {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	return labels
}

// itemIcons renders each item's status symbol, colored per the theme when color
// is enabled. Disabled color yields the plain symbol, which still conveys status.
func (m RunGetFlowModel) itemIcons(items []RunGetItem) []string {
	icons := make([]string, len(items))
	for i, it := range items {
		icons[i] = colorizeStatusIcon(it.Icon, m.opts.Color)
	}
	return icons
}

// colorizeStatusIcon wraps a status symbol in its theme color. Unknown symbols
// (and all symbols when color is off) are returned unchanged.
func colorizeStatusIcon(symbol string, color bool) string {
	if !color || symbol == "" {
		return symbol
	}
	if style, ok := statusIconStyle(symbol); ok {
		return style.Render(symbol)
	}
	return symbol
}

// statusIconStyle maps a symbol from apiclient.PhaseOutcomeSymbol to a theme
// color. The bool is false for symbols that should stay uncolored.
func statusIconStyle(symbol string) (lipgloss.Style, bool) {
	switch symbol {
	case "✓":
		return theme.SuccessStyle, true // succeeded
	case "✗":
		return theme.ErrorStyle, true // failed
	case "!":
		return theme.WarningStyle, true // errored / timed out
	case "●":
		return theme.RunningStyle, true // running
	case "○", "⊘":
		return theme.HelperStyle, true // created/queued, canceled
	default:
		return lipgloss.Style{}, false
	}
}

// --- commands ---

// cmdFetchRuns fetches the runs for the scope at scopeIdx filtered by the status
// at statusIdx: the user's runs across all projects for a "my runs" scope (via
// FetchMyRuns), otherwise the branch-filtered runs (via FetchRuns). The scope
// and status indexes ride along on the result so onRuns can commit them once the
// fetch succeeds.
func (m RunGetFlowModel) cmdFetchRuns(scopeIdx, statusIdx int) tea.Cmd {
	ctx, scope := m.ctx, m.scopes[scopeIdx]
	status := m.statusFilters[statusIdx].Value
	fetchRuns, fetchMine := m.opts.FetchRuns, m.opts.FetchMyRuns
	return func() tea.Msg {
		var (
			items []RunGetItem
			err   error
		)
		if scope.myRuns {
			items, err = fetchMine(ctx, status)
		} else {
			items, err = fetchRuns(ctx, scope.branch, status)
		}
		return runGetRunsMsg{items: items, scopeIdx: scopeIdx, statusIdx: statusIdx, err: err}
	}
}

func (m RunGetFlowModel) cmdFetchWorkflows() tea.Cmd {
	ctx, fn, runID := m.ctx, m.opts.FetchWorkflows, m.runID
	return func() tea.Msg {
		items, err := fn(ctx, runID)
		return runGetWorkflowsMsg{items: items, err: err}
	}
}

func (m RunGetFlowModel) cmdFetchJobs() tea.Cmd {
	ctx, fn, wfID := m.ctx, m.opts.FetchJobs, m.workflowID
	return func() tea.Msg {
		items, err := fn(ctx, wfID)
		return runGetJobsMsg{items: items, err: err}
	}
}

func (m RunGetFlowModel) cmdFetchExecutions() tea.Cmd {
	ctx, fn, jobID := m.ctx, m.opts.FetchExecutions, m.jobID
	return func() tea.Msg {
		items, err := fn(ctx, jobID)
		return runGetExecutionsMsg{items: items, err: err}
	}
}

// cmdFetchTests lists the job's failed tests. A nil FetchFailedTests (e.g. an
// unwired test harness) yields an empty list rather than panicking.
func (m RunGetFlowModel) cmdFetchTests() tea.Cmd {
	ctx, fn, jobID := m.ctx, m.opts.FetchFailedTests, m.jobID
	return func() tea.Msg {
		if fn == nil {
			return runGetTestsMsg{}
		}
		items, err := fn(ctx, jobID)
		return runGetTestsMsg{items: items, err: err}
	}
}

func (m RunGetFlowModel) cmdFetchStepStdout(epoch int) tea.Cmd {
	ctx, fn := m.ctx, m.opts.FetchStepStdout
	jobID, execution, stepNum, offset := m.jobID, m.execution, m.stepNum, m.pagerOffset
	return func() tea.Msg {
		data, terminal, err := fn(ctx, jobID, execution, stepNum, offset)
		return runGetStepStdoutMsg{epoch: epoch, data: data, terminal: terminal, err: err}
	}
}

func (m RunGetFlowModel) cmdFetchStepStderr(epoch int) tea.Cmd {
	ctx, fn := m.ctx, m.opts.FetchStepStderr
	jobID, execution, stepNum := m.jobID, m.execution, m.stepNum
	return func() tea.Msg {
		data, err := fn(ctx, jobID, execution, stepNum)
		return runGetStepStderrMsg{epoch: epoch, data: data, err: err}
	}
}

// cmdStepPoll schedules the next stdout poll 2 seconds out, tagged with the
// stream epoch so a poll from a superseded stream is ignored on arrival.
func (m RunGetFlowModel) cmdStepPoll(epoch int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return runGetStepPollMsg{epoch: epoch}
	})
}
