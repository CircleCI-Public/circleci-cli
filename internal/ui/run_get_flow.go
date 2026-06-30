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
	"context"
	"fmt"
	"runtime"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// switchScopeKey is the run picker's "switch branch" key, with its display
// label. Non-Windows uses shift+tab; on Windows the ConPTY/ultraviolet input
// stack drops shift+tab (the modifier is discarded and CSI Z yields no key
// event at all), so plain Tab is bound instead.
var switchScopeKey, switchScopeKeyLabel = func() (string, string) {
	if runtime.GOOS == "windows" {
		return components.KeyTab, "tab"
	}
	return components.KeyShiftTab, "shift+tab"
}()

// Labels for the "show everything at this level" option that heads each of the
// workflow and job pickers.
const (
	runGetAllWorkflowsLabel = "See all workflows (run summary)"
	runGetAllJobsLabel      = "All jobs in workflow (workflow summary)"
	runGetJobReportLabel    = "Job report (summary)"
	runGetJobOutputLabel    = "Full job report (including step output)"

	// runGetMetaCount is the number of leading job-summary options (job report,
	// full output report). They sit on the first picker after the job: the step
	// picker for a single-execution job, or the execution picker otherwise.
	runGetMetaCount = 2

	// runGetBackHint replaces the default footer on the workflow, job and step
	// pickers, where esc goes back a step rather than quitting and r re-fetches.
	runGetBackHint = "(↑/↓ to move, enter to select, r to refresh, esc to go back)"

	// runGetMetaGlyph fills the icon column for the leading "see all" / "all
	// jobs" summary options. They carry no status, so rather than leave a blank
	// gap they get a muted "list" mark that reads as "show everything here" and
	// keeps the column aligned with the status-bearing rows below.
	runGetMetaGlyph = "≡"
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
	// RunGetActionShowStep displays the output of a single job step, identified
	// by JobID + Execution + StepNum.
	RunGetActionShowStep
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
}

// RunGetStepItem is one selectable job step. Steps have no UUID; they are
// addressed by their parallel-execution index and step number.
type RunGetStepItem struct {
	Label     string
	Icon      string
	Execution int
	StepNum   int
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
	// Execution and StepNum are set only for RunGetActionShowStep (the chosen
	// step's parallel execution and step number).
	Execution int
	StepNum   int
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
	Color           bool
	// Animate reports whether the loading spinner should animate. Pass false when
	// CIRCLE_SPINNER_DISABLED is set (or the session is non-interactive) so the
	// loading line stays static instead of repainting.
	Animate bool

	// CurrentBranch is the branch the initial Runs were fetched for.
	// DefaultBranch is the project's default branch. When the two differ, the run
	// picker offers a shift+tab toggle between them, re-fetching via FetchRuns.
	// When DefaultBranch is empty or equal to CurrentBranch, the toggle is hidden.
	CurrentBranch string
	DefaultBranch string
	FetchRuns     func(ctx context.Context, branch string) ([]RunGetItem, error)
}

// runScope is one entry in the run picker's shift+tab cycle: a branch filter
// ("" means all branches) with the label shown in the picker title and loading
// line, and the note shown when the scope has no runs.
type runScope struct {
	branch    string // "" = all branches
	label     string // title/loading wording, e.g. "main branch", "all branches"
	emptyNote string // shown when the scope has no runs
}

// titleName is the bracket-inner text for the picker title: the bare branch
// name for a branch scope, or "all branches" for the unfiltered scope.
func (s runScope) titleName() string {
	if s.branch == "" {
		return "all branches"
	}
	return s.branch
}

// buildRunScopes assembles the toggle cycle: the current branch, then the
// default branch when it is known and distinct, then "all branches". The cycle
// always ends with all-branches so a toggle is offered even when there is only
// one branch to name.
func buildRunScopes(current, defaultBranch string) []runScope {
	branchScope := func(b string) runScope {
		return runScope{branch: b, label: b + " branch", emptyNote: "No runs found on " + b}
	}
	scopes := []runScope{branchScope(current)}
	if defaultBranch != "" && defaultBranch != current {
		scopes = append(scopes, branchScope(defaultBranch))
	}
	scopes = append(scopes, runScope{label: "all branches", emptyNote: "No runs found on any branch"})
	return scopes
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
//  5. Pick a step, or one of two summaries — "job report" (→ RunGetActionShowJob)
//     or the full per-step output report (→ RunGetActionShowJobOutput). Picking
//     a step yields RunGetActionShowStep; the cursor starts on the first failed
//     step.
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

	spin         spinner.Model
	loadingLabel string

	// height is the latest terminal height, passed to each picker so long lists
	// (e.g. many steps) scroll instead of overflowing the screen.
	height int

	// Remembered cursors so moving back redisplays a picker where it was left.
	runCursor       int
	workflowCursor  int
	jobCursor       int
	executionCursor int

	// runs is the run list currently shown by the first picker. It starts as
	// opts.Runs (the CurrentBranch runs) and is replaced when the user cycles to
	// another scope with shift+tab. scopes is the ordered cycle of branch filters
	// (current branch, default branch, all branches); activeBranch is the branch
	// runs currently holds ("" meaning all branches). toggleNote carries a
	// transient footer message (e.g. when a scope has no runs).
	runs         []RunGetItem
	scopes       []runScope
	activeBranch string
	toggleNote   string

	// Fetched data for the current selections, parallel to the pickers (the
	// workflow/job/step pickers are offset by their leading summary options).
	workflows  []RunGetItem
	jobs       []RunGetItem
	executions []RunGetExecution
	steps      []RunGetStepItem

	runID      uuid.UUID
	workflowID uuid.UUID
	jobID      uuid.UUID
	execution  int // chosen parallel execution index (0 when single-execution)

	// restoreStep is set when an executions re-fetch was triggered by refreshing
	// the step picker; onExecutions then re-enters the same execution's steps
	// rather than routing back through the execution picker.
	restoreStep bool

	result RunGetResult
}

// async message types carrying fetch results back into the Update loop.
type (
	runGetRunsMsg struct {
		items  []RunGetItem
		branch string
		err    error
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
)

// NewRunGetFlow returns a RunGetFlowModel ready to pass to tea.NewProgram.
func NewRunGetFlow(ctx context.Context, opts RunGetFlowOptions) RunGetFlowModel {
	m := RunGetFlowModel{
		ctx:          ctx,
		opts:         opts,
		stage:        runGetStageRunSelect,
		spin:         components.NewSpinner(opts.Color),
		runs:         opts.Runs,
		scopes:       buildRunScopes(opts.CurrentBranch, opts.DefaultBranch),
		activeBranch: opts.CurrentBranch,
	}
	m.runSelect = m.newRunSelect()
	return m
}

// Result returns the final outcome. Only valid after tea.Program.Run() returns.
func (m RunGetFlowModel) Result() RunGetResult { return m.result }

func (m RunGetFlowModel) Init() tea.Cmd { return nil }

func (m RunGetFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Remember the terminal height for pickers built on later stages, and let it
	// fall through to the active picker (below) so a live resize re-windows it.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
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
	case runGetStageLoadingRuns, runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingExecutions:
		// ctrl+c can still abort while a fetch is in flight.
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == components.KeyCtrlC {
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
		m.stage == runGetStageLoadingExecutions
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
		switch k.String() {
		case components.KeyCtrlC, components.KeyEsc:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case switchScopeKey:
			if len(m.scopes) >= 2 {
				next := m.nextScope()
				m.toggleNote = ""
				m.stage = runGetStageLoadingRuns
				m.loadingLabel = "Fetching runs for " + next.label
				return m, m.loadingCmd(m.cmdFetchRuns(next.branch))
			}
			return m, nil
		case components.KeyR:
			m.toggleNote = ""
			m.stage = runGetStageLoadingRuns
			m.loadingLabel = "Refreshing runs"
			return m, m.loadingCmd(m.cmdFetchRuns(m.activeBranch))
		}
	}

	updated, cmd := m.runSelect.Update(msg)
	m.runSelect = updated.(components.SelectModel)
	if !m.runSelect.Done() {
		return m, cmd
	}

	m.runCursor = m.runSelect.Selected()
	m.runID = m.runs[m.runCursor].ID
	m.stage = runGetStageLoadingWorkflows
	m.loadingLabel = "Fetching workflows"
	return m, m.loadingCmd(m.cmdFetchWorkflows())
}

// onRuns handles a completed shift+tab scope toggle. On error it quits; when the
// scope has no runs it keeps the current list and shows a footer note; otherwise
// it swaps in the scope's runs with the cursor reset.
func (m RunGetFlowModel) onRuns(msg runGetRunsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	if len(msg.items) == 0 {
		m.toggleNote = m.scopeForBranch(msg.branch).emptyNote
	} else {
		m.runs = msg.items
		m.activeBranch = msg.branch
		m.runCursor = 0
		m.toggleNote = ""
	}
	m.runSelect = m.newRunSelect()
	m.stage = runGetStageRunSelect
	return m, nil
}

// currentScopeIdx is the index in scopes of the scope whose runs are showing.
func (m RunGetFlowModel) currentScopeIdx() int {
	for i, s := range m.scopes {
		if s.branch == m.activeBranch {
			return i
		}
	}
	return 0
}

// activeScope is the scope whose runs are currently shown.
func (m RunGetFlowModel) activeScope() runScope {
	return m.scopes[m.currentScopeIdx()]
}

// nextScope is the scope a shift+tab would cycle to, wrapping past the end.
func (m RunGetFlowModel) nextScope() runScope {
	return m.scopes[(m.currentScopeIdx()+1)%len(m.scopes)]
}

// scopeForBranch returns the scope matching a branch filter (for its wording).
func (m RunGetFlowModel) scopeForBranch(branch string) runScope {
	for _, s := range m.scopes {
		if s.branch == branch {
			return s
		}
	}
	return runScope{}
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
		switch k.String() {
		case components.KeyCtrlC:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case components.KeyEsc:
			m.runSelect = m.newRunSelect()
			m.stage = runGetStageRunSelect
			return m, nil
		case components.KeyR:
			m.stage = runGetStageLoadingWorkflows
			m.loadingLabel = "Refreshing workflows"
			return m, m.loadingCmd(m.cmdFetchWorkflows())
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
		switch k.String() {
		case components.KeyCtrlC:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case components.KeyEsc:
			m.workflowSelect = m.newWorkflowSelect()
			m.stage = runGetStageWorkflowSelect
			return m, nil
		case components.KeyR:
			m.stage = runGetStageLoadingJobs
			m.loadingLabel = "Refreshing jobs"
			return m, m.loadingCmd(m.cmdFetchJobs())
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

// enterStepSelect scopes the step picker to one execution and shows it.
func (m RunGetFlowModel) enterStepSelect(exec RunGetExecution) RunGetFlowModel {
	m.execution = exec.Index
	m.steps = exec.Steps
	m.stepSelect = m.newStepSelect()
	m.stage = runGetStageStepSelect
	return m
}

// updateExecutionSelect handles the execution picker, shown only when a job has
// parallelism > 1. Its leading options are the job summaries; the remaining
// rows are executions. esc returns to the job picker; ctrl+c quits.
func (m RunGetFlowModel) updateExecutionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case components.KeyCtrlC:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case components.KeyEsc:
			m.jobSelect = m.newJobSelect()
			m.stage = runGetStageJobSelect
			return m, nil
		case components.KeyR:
			m.stage = runGetStageLoadingExecutions
			m.loadingLabel = "Refreshing steps"
			return m, m.loadingCmd(m.cmdFetchExecutions())
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
		switch k.String() {
		case components.KeyCtrlC:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case components.KeyEsc:
			if len(m.executions) > 1 {
				m.executionSelect = m.newExecutionSelect()
				m.stage = runGetStageExecutionSelect
			} else {
				m.jobSelect = m.newJobSelect()
				m.stage = runGetStageJobSelect
			}
			return m, nil
		case components.KeyR:
			// Steps come from the job's executions; re-fetch them and re-enter the
			// current execution's steps (restoreStep) instead of bouncing back to
			// the execution picker.
			m.restoreStep = true
			m.stage = runGetStageLoadingExecutions
			m.loadingLabel = "Refreshing steps"
			return m, m.loadingCmd(m.cmdFetchExecutions())
		}
	}

	updated, cmd := m.stepSelect.Update(msg)
	m.stepSelect = updated.(components.SelectModel)
	if !m.stepSelect.Done() {
		return m, cmd
	}

	sel := m.stepSelect.Selected()
	if meta := m.stepMetaCount(); meta > 0 {
		switch sel {
		case 0: // "job report" — short summary
			return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
		case 1: // "full output report" — every step's output
			return m.quit(RunGetResult{Action: RunGetActionShowJobOutput, JobID: m.jobID})
		}
		sel -= meta
	}
	step := m.steps[sel]
	return m.quit(RunGetResult{
		Action:    RunGetActionShowStep,
		JobID:     m.jobID,
		Execution: step.Execution,
		StepNum:   step.StepNum,
	})
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
	case runGetStageLoadingRuns, runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingExecutions:
		label := theme.HelperStyle.Render(m.loadingLabel)
		if m.opts.Animate {
			label = m.spin.View() + " " + label
		}
		return tea.NewView(label)
	case runGetStageDone:
		// Empty final frame so the last picker is cleared before the program
		// exits and the summary prints in its place.
		return tea.NewView("")
	}
	return tea.NewView("")
}

// --- picker builders ---

func (m RunGetFlowModel) newRunSelect() components.SelectModel {
	// Name the active scope in the title so it is clear which branch's runs are
	// listed (e.g. "Select a run [main]" or "… [all branches]"), bracketed to
	// match the per-row "[branch]" labels and colored so it stands out from the
	// rest of the (bold, uncolored) title.
	prompt := "Select a run"
	if len(m.scopes) >= 2 {
		scope := "[" + m.activeScope().titleName() + "]"
		if m.opts.Color {
			scope = theme.SecondaryStyle.Render(scope)
		}
		prompt = "Select a run " + scope
	}
	return components.NewSelectModel(prompt, itemLabels(m.runs)).
		WithIcons(m.itemIcons(m.runs)).
		WithCursor(m.runCursor).
		WithHint(m.runSelectHint()).
		WithHeight(m.height)
}

// runSelectHint is the footer for the run picker. The first picker quits on esc,
// so it keeps the default movement hints and, when a scope toggle is available,
// advertises the switch shortcut (shift+tab, or Tab on Windows; the active scope
// is named in the title, not here). A transient toggleNote (e.g. "No runs found
// on …") is prefixed when set.
func (m RunGetFlowModel) runSelectHint() string {
	hint := "(↑/↓ to move, enter to select, r to refresh, esc to quit)"
	if len(m.scopes) >= 2 {
		hint = fmt.Sprintf("(↑/↓ to move, enter to select, r to refresh, %s to switch branch, esc to quit)", switchScopeKeyLabel)
	}
	if m.toggleNote != "" {
		hint = m.toggleNote + "  " + hint
	}
	return hint
}

func (m RunGetFlowModel) newWorkflowSelect() components.SelectModel {
	labels := append([]string{runGetAllWorkflowsLabel}, itemLabels(m.workflows)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.workflows)...)
	return components.NewSelectModel("Select a workflow", labels).
		WithIcons(icons).
		WithCursor(m.workflowCursor).
		WithHint(runGetBackHint).
		WithHeight(m.height)
}

func (m RunGetFlowModel) newJobSelect() components.SelectModel {
	labels := append([]string{runGetAllJobsLabel}, itemLabels(m.jobs)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.jobs)...)
	return components.NewSelectModel("Select a job", labels).
		WithIcons(icons).
		WithCursor(m.jobCursor).
		WithHint(runGetBackHint).
		WithHeight(m.height)
}

func (m RunGetFlowModel) newExecutionSelect() components.SelectModel {
	labels := make([]string, 0, len(m.executions)+runGetMetaCount)
	icons := make([]string, 0, len(m.executions)+runGetMetaCount)
	labels = append(labels, runGetJobReportLabel, runGetJobOutputLabel)
	icons = append(icons, m.metaIcon(), m.metaIcon())
	for _, e := range m.executions {
		labels = append(labels, e.Label)
		icons = append(icons, colorizeStatusIcon(e.Icon, m.opts.Color))
	}
	return components.NewSelectModel("Select an execution", labels).
		WithIcons(icons).
		WithCursor(m.executionCursor).
		WithHint(runGetBackHint).
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
		labels = append(labels, runGetJobReportLabel, runGetJobOutputLabel)
		icons = append(icons, m.metaIcon(), m.metaIcon())
	}
	for _, s := range m.steps {
		labels = append(labels, s.Label)
		icons = append(icons, colorizeStatusIcon(s.Icon, m.opts.Color))
	}
	return components.NewSelectModel("Select a step", labels).
		WithIcons(icons).
		WithCursor(m.firstFailedStepCursor()).
		WithHint(runGetBackHint).
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
	if m.opts.Color {
		return theme.HelperStyle.Render(runGetMetaGlyph)
	}
	return runGetMetaGlyph
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

func (m RunGetFlowModel) cmdFetchRuns(branch string) tea.Cmd {
	ctx, fn := m.ctx, m.opts.FetchRuns
	return func() tea.Msg {
		items, err := fn(ctx, branch)
		return runGetRunsMsg{items: items, branch: branch, err: err}
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
