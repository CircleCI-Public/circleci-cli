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
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
	"github.com/CircleCI-Public/circleci-cli/clikit/ui/theme"
)

// Poll cadence for the watch flow. The first poll follows RunWatchPollInterval
// and each subsequent one waits a step longer, up to RunWatchMaxPollInterval —
// a run that has been going for twenty minutes rarely needs second-by-second
// attention, and the API does not need the traffic. The clock redraws at least
// once a second regardless, so the display never looks frozen between polls.
const (
	RunWatchPollInterval    = 5 * time.Second
	RunWatchMaxPollInterval = 30 * time.Second

	// runWatchClockTick is the longest a clock tick ever waits. A tick usually
	// comes sooner: it is aimed at whichever figure on screen changes next (see
	// clockDelay).
	runWatchClockTick = time.Second
)

// Table column widths, in cells. A name longer than its column is truncated
// rather than allowed to push the status column out of line. The job column is
// two narrower than the workflow one to pay for the deeper indent, so both
// levels' status glyphs land in the same column while the names still nest.
const (
	runWatchWorkflowNameCol = 30
	runWatchJobNameCol      = 28
	runWatchStatusCol       = 10
)

// The footer's poll meter: how many cells it is drawn in, and the filled and
// empty cell. Six cells is enough motion to read as a countdown on the tightest
// interval without turning the footer into a progress bar.
const (
	runWatchMeterCells = 6
	runWatchMeterFull  = "▰"
	runWatchMeterEmpty = "▱"
)

// RunWatchJob is one job row in the watch table. Symbol is the uncolored status
// glyph (the flow colors it when color is enabled) and Status the matching word,
// kept apart so the word can be padded into a fixed-width column without the
// glyph's width throwing the alignment off. Failed marks a job whose outcome is
// a failure, which is what --failfast trips on and what the final error's
// suggestions are built from.
type RunWatchJob struct {
	ID     uuid.UUID
	Name   string
	Symbol string
	Status string
	Type   string
	Failed bool
}

// RunWatchWorkflow is one workflow row and the job rows nested under it.
type RunWatchWorkflow struct {
	Name     string
	Symbol   string
	Status   string
	Duration string
	Jobs     []RunWatchJob
}

// RunWatchState is one poll's worth of run state: the rows to draw, whether
// every workflow has reached a terminal phase, and the run's derived display
// status once it has. Like the run-get item types this mirrors what the API
// client returns rather than embedding it, keeping this package independent of
// internal/apiclient — the caller's Fetch callback does the conversion.
type RunWatchState struct {
	Workflows []RunWatchWorkflow
	Done      bool
	Outcome   string
}

// FailedJobs returns every job across every workflow whose outcome is a failure,
// in the order they appear in the table.
func (s RunWatchState) FailedJobs() []RunWatchJob {
	var failed []RunWatchJob
	for _, wf := range s.Workflows {
		for _, j := range wf.Jobs {
			if j.Failed {
				failed = append(failed, j)
			}
		}
	}
	return failed
}

// RunWatchResult is the outcome of a completed RunWatchFlowModel run, read via
// Result() after tea.Program.Run() returns. Exactly one of Cancelled, TimedOut,
// FailFast, Err or "the run finished" is true; in the last case State.Outcome
// says how it finished. State is always the most recent poll, so the caller can
// report failed jobs whichever way the flow ended.
type RunWatchResult struct {
	State     RunWatchState
	Elapsed   time.Duration
	Cancelled bool
	TimedOut  bool
	FailFast  bool
	Err       error
}

// RunWatchFlowOptions configures a RunWatchFlowModel. Fetch is the only required
// field: it returns the run's current state and is called once on start and then
// on every poll.
type RunWatchFlowOptions struct {
	RunID  uuid.UUID
	Branch string

	Color   bool
	Animate bool

	// FailFast ends the watch as soon as any job has failed, without waiting for
	// the rest of the run.
	FailFast bool

	// Timeout ends the watch once this much time has elapsed. Zero means no
	// timeout.
	Timeout time.Duration

	// PollInterval and MaxPollInterval override the default poll cadence. Zero
	// means RunWatchPollInterval / RunWatchMaxPollInterval.
	PollInterval    time.Duration
	MaxPollInterval time.Duration

	Fetch func(ctx context.Context) (RunWatchState, error)
}

// async message types carrying poll results and clock ticks into the Update loop.
type (
	runWatchStateMsg struct {
		state RunWatchState
		err   error
	}
	// runWatchClockMsg carries the generation of the chain that armed it, so a
	// tick from a superseded schedule is dropped rather than acted on.
	runWatchClockMsg struct{ gen int }
)

// runWatchKeys is the footer key hint set: poll again now, or stop watching.
// The table refreshes itself, so refresh is there for the widening poll interval
// alone — by the end of a long run the next update can be half a minute away,
// and the footer's countdown says so. ctrl+c and esc also quit but go
// unadvertised — the footer names the shortest form of each action, as the
// pickers' footers do.
var runWatchKeys = []key.Binding{components.BindRefresh, components.BindQuit}

// RunWatchFlowModel is the bubbletea program behind `circleci run watch` in an
// interactive terminal: a live table of the run's workflows and their jobs, each
// row carrying a status glyph, redrawn in place as the run progresses.
//
// It polls through the caller-supplied Fetch callback — first immediately, then
// on a widening interval — and ends itself when every workflow has reached a
// terminal phase, when --failfast trips on a failed job, or when the timeout
// elapses. r polls again straight away; q, esc and ctrl+c stop watching (the run
// itself keeps going). After Run() returns, read the outcome with Result(); the
// caller prints the final summary line and decides the exit code.
//
// The program is deliberately inline rather than full-screen: the final frame is
// the completed table, and the caller's summary line prints directly beneath it.
type RunWatchFlowModel struct {
	ctx  context.Context
	opts RunWatchFlowOptions

	width  int
	start  time.Time
	spin   spinner.Model
	state  RunWatchState
	loaded bool

	// The poll schedule. wait is the interval the countdown on screen is measured
	// against and nextWait the one after it — the backoff is applied when a wait
	// is scheduled, not while it runs, so the first wait is the base interval and
	// the meter's span always matches the figure beside it. nextPoll is when the
	// wait expires, and is zero while a fetch is in flight.
	wait     time.Duration
	nextWait time.Duration
	nextPoll time.Time
	polling  bool

	// clockGen identifies the live clock chain. Arming a clock supersedes the
	// outstanding one, and a tick that arrives from an older generation is
	// ignored — so anything that changes the schedule can re-arm freely without
	// leaving a second chain running behind it. That is the bug this flow shipped
	// with: a poll timer abandoned by "r" fired anyway, and from then on the run
	// refreshed at twice the cadence its own countdown showed.
	clockGen int

	done   bool
	result RunWatchResult
}

// NewRunWatchFlow returns a RunWatchFlowModel ready to pass to tea.NewProgram.
func NewRunWatchFlow(ctx context.Context, opts RunWatchFlowOptions) RunWatchFlowModel {
	if opts.PollInterval <= 0 {
		opts.PollInterval = RunWatchPollInterval
	}
	if opts.MaxPollInterval <= 0 {
		opts.MaxPollInterval = RunWatchMaxPollInterval
	}
	return RunWatchFlowModel{
		ctx:      ctx,
		opts:     opts,
		start:    time.Now(),
		spin:     components.NewSpinner(opts.Color),
		nextWait: opts.PollInterval,
		// Init's first act is to fetch, so the model starts mid-poll.
		polling: true,
	}
}

// Result returns the final outcome. Only valid after tea.Program.Run() returns.
func (m RunWatchFlowModel) Result() RunWatchResult { return m.result }

func (m RunWatchFlowModel) Init() tea.Cmd {
	// Init cannot alter the model, so the first clock is armed at the generation
	// the model already carries; every later arm supersedes it.
	clock := tea.Tick(m.clockDelay(), func(time.Time) tea.Msg { return runWatchClockMsg{gen: m.clockGen} })
	cmds := []tea.Cmd{m.cmdFetch(), clock}
	if m.opts.Animate {
		cmds = append(cmds, m.spin.Tick)
	}
	return tea.Batch(cmds...)
}

func (m RunWatchFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyPressMsg:
		return m.onKey(msg)

	case runWatchStateMsg:
		return m.onState(msg)

	case runWatchClockMsg:
		return m.onClock(msg)

	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		s, cmd := m.spin.Update(msg)
		m.spin = s
		return m, cmd
	}
	return m, nil
}

// onClock advances the display and, when the wait has expired, starts the next
// poll. Polling rides on the clock rather than a timer of its own, so the poll
// can only ever happen once the countdown on screen has actually run out.
func (m RunWatchFlowModel) onClock(msg runWatchClockMsg) (tea.Model, tea.Cmd) {
	if m.done || msg.gen != m.clockGen {
		return m, nil
	}
	if m.timedOut() {
		m.result.TimedOut = true
		return m.quit()
	}
	if m.waitExpired() {
		next, fetch := m.beginPoll()
		next, clock := next.armClock()
		return next, tea.Batch(fetch, clock)
	}
	next, clock := m.armClock()
	return next, clock
}

// waitExpired reports whether the scheduled wait has run out. A fetch already in
// flight (nextPoll zero) has nothing to wait for.
func (m RunWatchFlowModel) waitExpired() bool {
	return !m.polling && !m.nextPoll.IsZero() && !time.Now().Before(m.nextPoll)
}

// onKey handles the two things a watcher can do: poll again now, or stop
// watching. Quitting is not an error — the run carries on in CircleCI — but the
// caller reports it as a cancellation so a script cannot mistake an abandoned
// watch for a successful run.
func (m RunWatchFlowModel) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, components.KeyCtrlC, components.KeyEsc, components.BindQuit):
		m.result.Cancelled = true
		return m.quit()
	case key.Matches(msg, components.BindRefresh):
		if m.done || m.polling {
			return m, nil
		}
		// A manual refresh restarts the backoff: the user is watching closely
		// again, so go back to the tightest cadence.
		m.nextWait = m.opts.PollInterval
		next, fetch := m.beginPoll()
		next, clock := next.armClock()
		return next, tea.Batch(fetch, clock)
	}
	return m, nil
}

// beginPoll marks a fetch as in flight — the countdown has nothing left to count
// down to — and returns the command that runs it.
func (m RunWatchFlowModel) beginPoll() (RunWatchFlowModel, tea.Cmd) {
	m.polling = true
	m.nextPoll = time.Time{}
	return m, m.cmdFetch()
}

// onState folds a completed poll into the model and decides whether the watch is
// over. A fetch failure ends the flow: watch has no partial state worth showing
// once the API stops answering, and a caller that kept polling through errors
// would hide an expired token behind a spinner.
func (m RunWatchFlowModel) onState(msg runWatchStateMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.result.Cancelled = true
		} else {
			m.result.Err = msg.err
		}
		return m.quit()
	}

	m.state = msg.state
	m.loaded = true
	m.polling = false
	m.result.State = msg.state

	switch {
	case msg.state.Done:
		return m.quit()
	case m.opts.FailFast && len(msg.state.FailedJobs()) > 0:
		m.result.FailFast = true
		return m.quit()
	case m.timedOut():
		m.result.TimedOut = true
		return m.quit()
	}

	m.wait = m.nextWait
	m.nextPoll = time.Now().Add(m.wait)
	m.nextWait = min(m.nextWait+m.opts.PollInterval, m.opts.MaxPollInterval)
	// Re-arm rather than leave it to the outstanding tick, which was aimed at a
	// schedule that predates this wait: a wait shorter than the time left on that
	// tick would otherwise not be noticed until it landed.
	next, clock := m.armClock()
	return next, clock
}

// quit stamps the elapsed time onto the result and ends the program. The view
// keeps rendering the table, so the final frame bubbletea flushes on exit is the
// finished run — the caller's summary line then prints beneath it.
func (m RunWatchFlowModel) quit() (tea.Model, tea.Cmd) {
	m.done = true
	m.result.Elapsed = m.elapsed()
	return m, tea.Quit
}

func (m RunWatchFlowModel) elapsed() time.Duration { return time.Since(m.start) }

func (m RunWatchFlowModel) timedOut() bool {
	return m.opts.Timeout > 0 && m.elapsed() >= m.opts.Timeout
}

// --- commands ---

func (m RunWatchFlowModel) cmdFetch() tea.Cmd {
	ctx, fetch := m.ctx, m.opts.Fetch
	return func() tea.Msg {
		if fetch == nil {
			return runWatchStateMsg{}
		}
		state, err := fetch(ctx)
		return runWatchStateMsg{state: state, err: err}
	}
}

// armClock schedules the next clock tick and supersedes the outstanding one (see
// clockGen). Every path that ends a tick or moves the schedule arms a new clock;
// nothing else has to reason about how many are in flight.
func (m RunWatchFlowModel) armClock() (RunWatchFlowModel, tea.Cmd) {
	m.clockGen++
	gen := m.clockGen
	return m, tea.Tick(m.clockDelay(), func(time.Time) tea.Msg { return runWatchClockMsg{gen: gen} })
}

// clockDelay is how long to wait before the next redraw: until whichever figure
// on screen changes first — the elapsed clock ticking over, or the countdown
// losing a second. A flat one-second tick drifts against both (each cycle is a
// second plus whatever the loop took), and once it is out of phase with the
// deadline it is counting down to, the figure skips a second here and lingers on
// one there. Aiming each tick at the next boundary keeps every second on screen
// for a second, and lands the last one exactly on the poll.
func (m RunWatchFlowModel) clockDelay() time.Duration {
	delay := runWatchClockTick - m.elapsed()%runWatchClockTick
	if !m.nextPoll.IsZero() {
		if untilTick := time.Until(m.nextPoll) % runWatchClockTick; untilTick > 0 && untilTick < delay {
			delay = untilTick
		}
	}
	return delay
}

// --- view ---

func (m RunWatchFlowModel) View() tea.View {
	return components.WithWindowTitle(m.buildView(), components.FlowTitle("run watch"))
}

func (m RunWatchFlowModel) buildView() tea.View {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")

	if !m.loaded {
		b.WriteString(m.line("  " + m.spinnerPrefix() + m.muted("Fetching run status…")))
		return tea.NewView(b.String())
	}

	for _, wf := range m.state.Workflows {
		b.WriteString(m.line(m.workflowRow(wf)))
		for _, j := range wf.Jobs {
			b.WriteString(m.line(m.jobRow(j)))
		}
	}
	if footer := m.footer(); footer != "" {
		b.WriteString("\n" + footer)
	}
	return tea.NewView(b.String())
}

// header names the run being watched: the ID, so it can be pasted into another
// command, and the branch it belongs to in the secondary color the pickers use
// for a scope.
func (m RunWatchFlowModel) header() string {
	title := theme.TitleStyle.Render("Watching run " + m.opts.RunID.String())
	if m.opts.Branch == "" {
		return title
	}
	scope := "[" + m.opts.Branch + "]"
	if m.opts.Color {
		scope = theme.SecondaryStyle.Render(scope)
	}
	return title + " " + scope
}

// footer is the elapsed clock and the key hints, with a spinner leading them
// while the run is still going. The clock is followed by when the next poll
// lands, which is what makes the table legible as live — without it a run that
// has settled into the 30-second cadence looks stalled, and "r" looks like the
// only way to see anything new.
//
// Once the watch is over the whole footer goes: the keys are no longer live,
// nothing further is coming, and the summary line the caller prints underneath
// already carries the elapsed time. The final frame is the finished table alone.
func (m RunWatchFlowModel) footer() string {
	if m.done {
		return ""
	}
	return m.line("  "+m.spinnerPrefix()+
		m.muted("Elapsed "+FormatElapsed(m.elapsed())+" · "+m.nextPollNote())) +
		m.line("  "+components.Hints(runWatchKeys...))
}

// nextPollNote says when the run's status is next checked: a meter filling
// towards the next poll, or that one is happening right now. The meter carries
// the wait at a glance and the figure beside it makes the wait exact — the poll
// interval widens as the run goes on, so a bar alone would say "nearly there"
// without ever saying how long that is.
func (m RunWatchFlowModel) nextPollNote() string {
	remaining := ceilSeconds(time.Until(m.nextPoll))
	if m.polling || m.nextPoll.IsZero() || remaining <= 0 {
		return "updating…"
	}
	return fmt.Sprintf("next update %s %s", m.pollMeter(remaining), FormatElapsed(remaining))
}

// ceilSeconds rounds a countdown up to the next whole second. Rounding to the
// nearest would drop the figure half a second early and finish the count with a
// second still to run — which reads as the timer skipping and the run refreshing
// ahead of its own countdown. Rounded up, "1s" means "up to a second left" and
// the count reaches zero exactly when the poll fires.
func ceilSeconds(d time.Duration) time.Duration {
	// Truncate rounds towards zero, so rounding a negative duration up would send
	// it away from zero — a poll that is already overdue would read as a second
	// still to run rather than as an update in progress.
	if d <= 0 {
		return 0
	}
	if truncated := d.Truncate(time.Second); truncated != d {
		return truncated + time.Second
	}
	return d
}

// pollMeter draws the wait for the next poll as a small bar that fills as it
// elapses. Solid and outline cells carry the reading on their own, so it survives
// with color off, and both stay outside the status-glyph vocabulary (see
// statusIconStyle) — a filling circle would read as a queued or running job,
// which is exactly what the rows above the footer use ○ and ● for.
func (m RunWatchFlowModel) pollMeter(remaining time.Duration) string {
	filled := 0
	if m.wait > 0 {
		// Rounded to the nearest cell rather than truncated, so the meter tracks
		// the wait evenly instead of trailing it by up to a cell.
		elapsed := m.wait - remaining
		filled = int((int64(runWatchMeterCells)*int64(elapsed) + int64(m.wait)/2) / int64(m.wait))
	}
	filled = min(max(filled, 0), runWatchMeterCells)
	return strings.Repeat(runWatchMeterFull, filled) +
		strings.Repeat(runWatchMeterEmpty, runWatchMeterCells-filled)
}

// workflowRow renders a workflow's name, status and duration. A workflow that
// has not finished has no duration yet, so the column is simply absent.
func (m RunWatchFlowModel) workflowRow(wf RunWatchWorkflow) string {
	return fmt.Sprintf("  %s  %s %s  %s",
		pad(wf.Name, runWatchWorkflowNameCol),
		colorizeStatusIcon(wf.Symbol, m.opts.Color),
		pad(wf.Status, runWatchStatusCol),
		wf.Duration)
}

// jobRow renders one job, indented under its workflow, with the job type in the
// trailing column.
func (m RunWatchFlowModel) jobRow(j RunWatchJob) string {
	return fmt.Sprintf("    %s  %s %s  %s",
		pad(j.Name, runWatchJobNameCol),
		colorizeStatusIcon(j.Symbol, m.opts.Color),
		pad(j.Status, runWatchStatusCol),
		j.Type)
}

// line trims a row's trailing padding, fits it to the terminal width, and
// terminates it. Truncation is ANSI-aware, so a row cut short at the right edge
// keeps its styling intact rather than leaking an escape sequence.
func (m RunWatchFlowModel) line(row string) string {
	row = strings.TrimRight(row, " ")
	if m.width > 0 {
		row = ansi.Truncate(row, m.width, "…")
	}
	return row + "\n"
}

// spinnerPrefix is the animated spinner and its trailing space, or nothing when
// the watch is over or animation is off (CIRCLE_SPINNER_DISABLED, or a
// non-interactive session).
func (m RunWatchFlowModel) spinnerPrefix() string {
	if m.done || !m.opts.Animate {
		return ""
	}
	return m.spin.View() + " "
}

// muted de-emphasizes footer text, leaving it plain when color is off.
func (m RunWatchFlowModel) muted(s string) string {
	if m.opts.Color {
		return theme.HelperStyle.Render(s)
	}
	return s
}

// pad fits s to exactly w cells: padded with spaces when short, truncated with an
// ellipsis when long, so a verbose job name cannot push the status column out of
// alignment. Width is measured in cells (not bytes or runes) so a name with
// wide glyphs still lines up.
func pad(s string, w int) string {
	if width := ansi.StringWidth(s); width <= w {
		return s + strings.Repeat(" ", w-width)
	}
	return ansi.Truncate(s, w, "…")
}

// FormatElapsed renders a watch duration as the compact "1h2m3s" form, dropping
// the units that are zero from the left. It is exported because `run watch`
// prints the same elapsed figure in its final summary line, after this program
// has exited, and the two must agree.
func FormatElapsed(d time.Duration) string {
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
