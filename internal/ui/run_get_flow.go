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

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// Labels for the "show everything at this level" option that heads each of the
// workflow and job pickers.
const (
	runGetAllWorkflowsLabel = "See all workflows (run summary)"
	runGetAllJobsLabel      = "All jobs in workflow (workflow summary)"
	runGetJobReportLabel    = "Job report (summary)"
	runGetJobOutputLabel    = "Full job report (including step output)"

	// runGetStepOffset is the number of leading meta options (job report, full
	// output report) before the steps in the step picker.
	runGetStepOffset = 2

	// runGetBackHint replaces the default footer on the workflow, job and step
	// pickers, where esc goes back a step rather than quitting.
	runGetBackHint = "(↑/↓ to move, enter to select, esc to go back)"

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

// RunGetResult is the outcome of a completed RunGetFlowModel run, read via
// Result() after tea.Program.Run() returns.
type RunGetResult struct {
	Action     RunGetAction
	RunID      uuid.UUID
	WorkflowID uuid.UUID
	JobID      uuid.UUID
	// Execution and StepNum are set only for RunGetActionShowStep.
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
	Runs           []RunGetItem
	FetchWorkflows func(ctx context.Context, runID uuid.UUID) ([]RunGetItem, error)
	FetchJobs      func(ctx context.Context, workflowID uuid.UUID) ([]RunGetItem, error)
	FetchSteps     func(ctx context.Context, jobID uuid.UUID) ([]RunGetStepItem, error)
	Color          bool
}

type runGetStage int

const (
	runGetStageRunSelect runGetStage = iota
	runGetStageLoadingWorkflows
	runGetStageWorkflowSelect
	runGetStageLoadingJobs
	runGetStageJobSelect
	runGetStageLoadingSteps
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
//  4. Pick a step, or one of two summaries — "job report" (→ RunGetActionShowJob)
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

	runSelect      components.SelectModel
	workflowSelect components.SelectModel
	jobSelect      components.SelectModel
	stepSelect     components.SelectModel

	spin         spinner.Model
	loadingLabel string

	// Remembered cursors so moving back redisplays a picker where it was left.
	runCursor      int
	workflowCursor int
	jobCursor      int

	// Fetched data for the current selections, parallel to the pickers (offset
	// by one for the leading "see all" / "job report" option).
	workflows []RunGetItem
	jobs      []RunGetItem
	steps     []RunGetStepItem

	runID      uuid.UUID
	workflowID uuid.UUID
	jobID      uuid.UUID

	result RunGetResult
}

// async message types carrying fetch results back into the Update loop.
type (
	runGetWorkflowsMsg struct {
		items []RunGetItem
		err   error
	}
	runGetJobsMsg struct {
		items []RunGetItem
		err   error
	}
	runGetStepsMsg struct {
		items []RunGetStepItem
		err   error
	}
)

// NewRunGetFlow returns a RunGetFlowModel ready to pass to tea.NewProgram.
func NewRunGetFlow(ctx context.Context, opts RunGetFlowOptions) RunGetFlowModel {
	m := RunGetFlowModel{
		ctx:   ctx,
		opts:  opts,
		stage: runGetStageRunSelect,
		spin:  components.NewSpinner(opts.Color),
	}
	m.runSelect = m.newRunSelect()
	return m
}

// Result returns the final outcome. Only valid after tea.Program.Run() returns.
func (m RunGetFlowModel) Result() RunGetResult { return m.result }

func (m RunGetFlowModel) Init() tea.Cmd { return nil }

func (m RunGetFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case runGetWorkflowsMsg:
		return m.onWorkflows(msg)
	case runGetJobsMsg:
		return m.onJobs(msg)
	case runGetStepsMsg:
		return m.onSteps(msg)
	}

	switch m.stage {
	case runGetStageRunSelect:
		return m.updateRunSelect(msg)
	case runGetStageWorkflowSelect:
		return m.updateWorkflowSelect(msg)
	case runGetStageJobSelect:
		return m.updateJobSelect(msg)
	case runGetStageStepSelect:
		return m.updateStepSelect(msg)
	case runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingSteps:
		// ctrl+c can still abort while a fetch is in flight.
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == components.KeyCtrlC {
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		}
	case runGetStageDone:
		// Quitting; no further input is processed.
	}
	return m, nil
}

func (m RunGetFlowModel) loading() bool {
	return m.stage == runGetStageLoadingWorkflows ||
		m.stage == runGetStageLoadingJobs ||
		m.stage == runGetStageLoadingSteps
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
		}
	}

	updated, cmd := m.runSelect.Update(msg)
	m.runSelect = updated.(components.SelectModel)
	if !m.runSelect.Done() {
		return m, cmd
	}

	m.runCursor = m.runSelect.Selected()
	m.runID = m.opts.Runs[m.runCursor].ID
	m.stage = runGetStageLoadingWorkflows
	m.loadingLabel = "Fetching workflows"
	return m, tea.Batch(m.spin.Tick, m.cmdFetchWorkflows())
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
	return m, tea.Batch(m.spin.Tick, m.cmdFetchJobs())
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
	m.stage = runGetStageLoadingSteps
	m.loadingLabel = "Fetching steps"
	return m, tea.Batch(m.spin.Tick, m.cmdFetchSteps())
}

func (m RunGetFlowModel) onSteps(msg runGetStepsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.quit(RunGetResult{Action: RunGetActionCancel, Err: msg.err})
	}
	// A job with no resolvable steps has nothing to pick — skip straight to the
	// job report rather than show a one-option picker.
	if len(msg.items) == 0 {
		return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
	}
	m.steps = msg.items
	m.stepSelect = m.newStepSelect()
	m.stage = runGetStageStepSelect
	return m, nil
}

// updateStepSelect handles the step picker. esc returns to the job picker;
// ctrl+c quits. The leading option shows the job report; picking a step shows
// that step's output.
func (m RunGetFlowModel) updateStepSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case components.KeyCtrlC:
			return m.quit(RunGetResult{Action: RunGetActionCancel})
		case components.KeyEsc:
			m.jobSelect = m.newJobSelect()
			m.stage = runGetStageJobSelect
			return m, nil
		}
	}

	updated, cmd := m.stepSelect.Update(msg)
	m.stepSelect = updated.(components.SelectModel)
	if !m.stepSelect.Done() {
		return m, cmd
	}

	switch m.stepSelect.Selected() {
	case 0: // "job report" — short summary
		return m.quit(RunGetResult{Action: RunGetActionShowJob, JobID: m.jobID})
	case 1: // "full output report" — every step's output
		return m.quit(RunGetResult{Action: RunGetActionShowJobOutput, JobID: m.jobID})
	}
	step := m.steps[m.stepSelect.Selected()-runGetStepOffset]
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
	case runGetStageStepSelect:
		return m.stepSelect.View()
	case runGetStageLoadingWorkflows, runGetStageLoadingJobs, runGetStageLoadingSteps:
		return tea.NewView(m.spin.View() + " " + theme.HelperStyle.Render(m.loadingLabel))
	case runGetStageDone:
		// Empty final frame so the last picker is cleared before the program
		// exits and the summary prints in its place.
		return tea.NewView("")
	}
	return tea.NewView("")
}

// --- picker builders ---

func (m RunGetFlowModel) newRunSelect() components.SelectModel {
	// The default hint ("…esc to quit") is correct here: the first picker quits.
	return components.NewSelectModel("Select a run", itemLabels(m.opts.Runs)).
		WithIcons(m.itemIcons(m.opts.Runs)).
		WithCursor(m.runCursor)
}

func (m RunGetFlowModel) newWorkflowSelect() components.SelectModel {
	labels := append([]string{runGetAllWorkflowsLabel}, itemLabels(m.workflows)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.workflows)...)
	return components.NewSelectModel("Select a workflow", labels).
		WithIcons(icons).
		WithCursor(m.workflowCursor).
		WithHint(runGetBackHint)
}

func (m RunGetFlowModel) newJobSelect() components.SelectModel {
	labels := append([]string{runGetAllJobsLabel}, itemLabels(m.jobs)...)
	icons := append([]string{m.metaIcon()}, m.itemIcons(m.jobs)...)
	return components.NewSelectModel("Select a job", labels).
		WithIcons(icons).
		WithCursor(m.jobCursor).
		WithHint(runGetBackHint)
}

func (m RunGetFlowModel) newStepSelect() components.SelectModel {
	labels := make([]string, 0, len(m.steps)+runGetStepOffset)
	icons := make([]string, 0, len(m.steps)+runGetStepOffset)
	labels = append(labels, runGetJobReportLabel, runGetJobOutputLabel)
	icons = append(icons, m.metaIcon(), m.metaIcon())
	for _, s := range m.steps {
		labels = append(labels, s.Label)
		icons = append(icons, colorizeStatusIcon(s.Icon, m.opts.Color))
	}
	return components.NewSelectModel("Select a step", labels).
		WithIcons(icons).
		WithCursor(m.firstFailedStepCursor()).
		WithHint(runGetBackHint)
}

// firstFailedStepCursor returns the picker index of the first failed/errored
// step (offset past the leading summary options), so the cursor lands on the
// likely target. Falls back to the first summary option when none failed.
func (m RunGetFlowModel) firstFailedStepCursor() int {
	for i, s := range m.steps {
		if s.Icon == "✗" || s.Icon == "!" {
			return i + runGetStepOffset
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
		return theme.AccentStyle, true // running
	case "○", "⊘":
		return theme.HelperStyle, true // created/queued, canceled
	default:
		return lipgloss.Style{}, false
	}
}

// --- commands ---

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

func (m RunGetFlowModel) cmdFetchSteps() tea.Cmd {
	ctx, fn, jobID := m.ctx, m.opts.FetchSteps, m.jobID
	return func() tea.Msg {
		items, err := fn(ctx, jobID)
		return runGetStepsMsg{items: items, err: err}
	}
}
