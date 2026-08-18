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

package ui_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// switchKey is the run picker's "switch branch" key, and switchLabel its footer
// label — platform-specific, matching the binding the flow uses (shift+tab
// normally, plain Tab on Windows where shift+tab is dropped).
var (
	switchKey = func() tea.KeyPressMsg {
		if runtime.GOOS == "windows" {
			return tea.KeyPressMsg{Code: tea.KeyTab}
		}
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	}()
	switchLabel = func() string {
		if runtime.GOOS == "windows" {
			return "tab"
		}
		return "shift+tab"
	}()

	keyR      = tea.KeyPressMsg{Code: 'r', Text: "r"}
	keyS      = tea.KeyPressMsg{Code: 's', Text: "s"}
	keyShiftS = tea.KeyPressMsg{Code: 'S', Text: "S"}
	keyDown   = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp     = tea.KeyPressMsg{Code: tea.KeyUp}
	keyEnt    = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc    = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyQ      = tea.KeyPressMsg{Code: 'q', Text: "q"}
	keyHelp   = tea.KeyPressMsg{Code: '?', Text: "?"}
	keySlash  = tea.KeyPressMsg{Code: '/', Text: "/"}
	keyRight  = tea.KeyPressMsg{Code: tea.KeyRight}
	keyLeft   = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyD      = tea.KeyPressMsg{Code: 'd', Text: "d"}
	keyO      = tea.KeyPressMsg{Code: 'o', Text: "o"}
	keyY      = tea.KeyPressMsg{Code: 'y', Text: "y"}
	keyN      = tea.KeyPressMsg{Code: 'n', Text: "n"}
)

// teaTimeout bounds every wait on a teatest program in this package. It is a
// liveness ceiling, not a latency budget: each wait returns the moment its
// condition is met, so raising it cannot slow a passing test — but a wait that
// is too tight fails a healthy program whenever the machine stalls. CI runs the
// whole tree with -race while the acceptance suite compiles and execs binaries
// on the same cores, and a 2s ceiling was tight enough to lose that race — on a
// step that takes ~10ms even with the CPUs saturated.
const teaTimeout = 10 * time.Second

// quitMsg tells flowHarness to end the program. The flow ignores unknown message
// types, so sending it does not perturb the model's state.
type quitMsg struct{}

// snapshotMsg asks flowHarness for the inner model's current frame, delivered on
// frame. The harness answers from inside the program loop, so a snapshot costs
// one message round-trip and — unlike quitting to read the final model — never
// waits on the renderer stopping and the terminal being restored. It also leaves
// the program running, so a test can snapshot and then keep driving the flow.
type snapshotMsg struct{ frame chan string }

// flowHarness drives a RunGetFlowModel as a standalone teatest program, serving
// snapshots and quitting on request without disturbing the inner model. (The
// flow's own quit paths switch to a "done" stage that renders an empty frame,
// which would defeat a snapshot.)
type flowHarness struct {
	m ui.RunGetFlowModel
}

func (h flowHarness) Init() tea.Cmd { return h.m.Init() }

func (h flowHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case quitMsg:
		return h, tea.Quit
	case snapshotMsg:
		msg.frame <- h.m.View().Content
		return h, nil
	}
	u, cmd := h.m.Update(msg)
	h.m = u.(ui.RunGetFlowModel)
	return h, cmd
}

func (h flowHarness) View() tea.View { return h.m.View() }

// startFlow runs a flow at a known terminal size and waits for the run picker.
// The program is ended on cleanup, since snapshots no longer end it themselves.
func startFlow(t *testing.T, m ui.RunGetFlowModel) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, flowHarness{m: m}, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() {
		tm.Send(quitMsg{})
		tm.WaitFinished(t, teatest.WithFinalTimeout(teaTimeout))
	})
	waitForOutput(t, tm, "Select a run")
	return tm
}

// waitForOutput blocks until the program's cumulative output contains s. It
// returns as soon as the substring appears, so fast assertions stay fast; the
// ceiling has to clear the streaming pager's 2s stdout poll.
func waitForOutput(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}, teatest.WithDuration(teaTimeout))
}

// flowFrame returns the inner model's current frame with its ANSI intact, for
// the few assertions about color and cursor-return handling.
func flowFrame(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	// Buffered so the program loop is never left blocked on a send, however this
	// test goroutine ends.
	frame := make(chan string, 1)
	tm.Send(snapshotMsg{frame: frame})
	select {
	case f := <-frame:
		return f
	case <-time.After(teaTimeout):
		t.Fatalf("no frame from the flow after %s", teaTimeout)
		return ""
	}
}

// flowSnapshot returns the inner model's current frame, ANSI stripped.
func flowSnapshot(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	return ansi.Strip(flowFrame(t, tm))
}

func runItem(label string) ui.RunGetItem {
	return ui.RunGetItem{ID: uuid.New(), Icon: "✓", Label: label}
}

// fetchByBranch returns a FetchRuns that maps a branch filter ("" = all
// branches) to its run list, returning an empty list for unmapped branches. The
// status argument is ignored (status filtering is covered separately).
func fetchByBranch(byBranch map[string][]ui.RunGetItem) func(context.Context, string, string, ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
	return func(_ context.Context, branch, _ string, _ ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
		return byBranch[branch], nil
	}
}

// newToggleFlow builds a run-get flow on branch "feature" with default branch
// "main". Animation is off so the loading command is the bare fetch (no spinner
// tick), keeping the program loop deterministic under teatest.
func newToggleFlow(fetch func(context.Context, string, string, ui.RunCreatedFilter) ([]ui.RunGetItem, error)) ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [feature] - 1 minute ago")},
		CurrentBranch: "feature",
		DefaultBranch: "main",
		FetchRuns:     fetch,
	})
}

// TestRunGetFlow_TitleNamesActiveScope shows the active scope, bracketed, in the
// picker title.
func TestRunGetFlow_TitleNamesActiveScope(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(nil)))
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a run [feature]"))
}

// TestRunGetFlow_FooterShortcuts confirms the footer advertises the refresh and
// trigger-switch shortcuts (the active branch is named in the title, not here).
func TestRunGetFlow_FooterShortcuts(t *testing.T) {
	v := flowSnapshot(t, startFlow(t, newToggleFlow(fetchByBranch(nil))))
	assert.Check(t, cmp.Contains(v, "r refresh"))
	assert.Check(t, cmp.Contains(v, switchLabel+" change trigger"))
}

// newHelpFlow builds a run-get flow with a markdown renderer wired, enabling the
// "?" help overlay. The renderer returns the markdown verbatim so tests can
// assert on its content without depending on glamour styling.
func newHelpFlow() ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:           []ui.RunGetItem{runItem("aaaaaaa [main] - 1 minute ago")},
		CurrentBranch:  "main",
		RenderMarkdown: func(md string, _ int) string { return md },
	})
}

// TestRunGetFlow_HelpOverlay confirms that "?" opens the keyboard-shortcut help,
// rendered in a rounded border to set it apart from the plain pickers.
func TestRunGetFlow_HelpOverlay(t *testing.T) {
	tm := startFlow(t, newHelpFlow())

	tm.Send(keyHelp)
	waitForOutput(t, tm, "Keyboard shortcuts")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "Keyboard shortcuts"))
	assert.Check(t, cmp.Contains(v, "╭")) // framed in a rounded border
}

// TestRunGetFlow_HelpOverlayReturnsOnEsc confirms esc dismisses the help overlay
// and returns to the picker it was opened from.
func TestRunGetFlow_HelpOverlayReturnsOnEsc(t *testing.T) {
	tm := startFlow(t, newHelpFlow())

	tm.Send(keyHelp)
	waitForOutput(t, tm, "Keyboard shortcuts")

	tm.Send(keyEsc)
	waitForOutput(t, tm, "Select a run")
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a run"))
}

// TestRunGetFlow_HelpOverlayClosesOnQ confirms that "q" also dismisses the help
// overlay and returns to the picker.
func TestRunGetFlow_HelpOverlayClosesOnQ(t *testing.T) {
	tm := startFlow(t, newHelpFlow())

	tm.Send(keyHelp)
	waitForOutput(t, tm, "Keyboard shortcuts")

	tm.Send(keyQ)
	waitForOutput(t, tm, "Select a run")
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a run"))
}

// TestRunGetFlow_HelpHintShownWhenEnabled confirms the run picker footer
// advertises "? help" when a markdown renderer is wired.
func TestRunGetFlow_HelpHintShownWhenEnabled(t *testing.T) {
	v := flowSnapshot(t, startFlow(t, newHelpFlow()))
	assert.Check(t, cmp.Contains(v, "? help"))
}

// TestRunGetFlow_HelpHintHiddenWhenDisabled confirms the "? help" hint is
// absent (and "?" inert) when no markdown renderer is supplied.
func TestRunGetFlow_HelpHintHiddenWhenDisabled(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(nil)))
	tm.Send(keyHelp) // inert without a renderer
	v := flowSnapshot(t, tm)
	assert.Check(t, !strings.Contains(v, "? help"))
	assert.Check(t, cmp.Contains(v, "Select a run")) // still on the picker
}

// TestRunGetFlow_WorkflowPickerShowsRunErrors verifies that selecting a run
// whose RunGetItem carries errors surfaces the error type and message under the
// workflow picker title — e.g. a config that failed to compile, which produced
// no workflows.
func TestRunGetFlow_WorkflowPickerShowsRunErrors(t *testing.T) {
	errRun := ui.RunGetItem{
		ID:    uuid.New(),
		Icon:  "⊘",
		Label: "No configuration was found - now",
		Errors: []ui.RunGetError{
			{Type: "config-fetch", Message: "No configuration was found in your project."},
		},
	}
	tm := startFlow(t, ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{errRun},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return nil, nil // the failed config produced no workflows
		},
	}))

	tm.Send(keyEnt) // select the errored run
	waitForOutput(t, tm, "Select a workflow")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "config-fetch: No configuration was found in your project."))
}

// TestRunGetFlow_ToggleCyclesScopes drives the switch key through the full cycle:
// current branch → default branch → all branches → back to current, swapping the
// run list each step. Each hop is a gated subtest whose WaitFor doubles as the
// assertion that the step landed (run rows are unique per scope, and rewritten
// in full, so their presence proves the toggle re-fetched and re-rendered that
// scope — titles share the "Select a run " prefix and are diff-rewritten in
// place, so they do not appear contiguously in the output stream). Gating stops
// the cycle at the first stalled hop rather than cascading misleading failures.
func TestRunGetFlow_ToggleCyclesScopes(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(map[string][]ui.RunGetItem{
		// The feature re-fetch returns a distinct row ("refetched") from the
		// initial list ("1 minute ago") so the wrap back to it has a unique token
		// to sync on — the original row is already in the output from startup.
		"feature": {runItem("aaaaaaa [feature] - refetched")},
		"main":    {runItem("bbbbbbb [main] - 2 minutes ago")},
		"":        {runItem("ccccccc [other] - 3 minutes ago")},
	})))

	assert.Assert(t, t.Run("feature → main", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "bbbbbbb [main]")
	}))
	assert.Assert(t, t.Run("main → all branches", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "ccccccc [other]")
	}))
	assert.Assert(t, t.Run("all branches → feature (wrap)", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "aaaaaaa [feature] - refetched")
	}))
}

// TestRunGetFlow_ToggleReachesMyRuns confirms that wiring FetchMyRuns appends a
// "my runs" scope to the shift+tab cycle: feature → main → all branches → my
// runs. The my-runs scope is fetched cross-project (via FetchMyRuns) rather than
// by branch, and is named "[my runs]" in the picker title.
func TestRunGetFlow_ToggleReachesMyRuns(t *testing.T) {
	tm := startFlow(t, ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [feature] - 1 minute ago")},
		CurrentBranch: "feature",
		DefaultBranch: "main",
		FetchRuns: fetchByBranch(map[string][]ui.RunGetItem{
			"feature": {runItem("aaaaaaa [feature] - 1 minute ago")},
			"main":    {runItem("bbbbbbb [main] - 2 minutes ago")},
			"":        {runItem("ccccccc [other] - 3 minutes ago")},
		}),
		FetchMyRuns: func(context.Context, string, ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{runItem("ddddddd [mine] - 4 minutes ago")}, nil
		},
	}))

	assert.Assert(t, t.Run("feature → main", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "bbbbbbb [main]")
	}))
	assert.Assert(t, t.Run("main → all branches", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "ccccccc [other]")
	}))
	assert.Assert(t, t.Run("all branches → my runs", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "ddddddd [mine]")
	}))
	assert.Assert(t, t.Run("names the my-runs scope in the title", func(t *testing.T) {
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a run [my runs]"))
	}))
}

// TestRunGetFlow_MyRunsOmittedWithoutFetch confirms that without FetchMyRuns the
// cycle stays branch-only (no "my runs" scope is added).
func TestRunGetFlow_MyRunsOmittedWithoutFetch(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(map[string][]ui.RunGetItem{
		// A distinct refetch row so the wrap back to feature has a unique token to
		// sync on (the initial row is already in the startup output).
		"feature": {runItem("aaaaaaa [feature] - refetched")},
		"main":    {runItem("bbbbbbb [main] - 2 minutes ago")},
		"":        {runItem("ccccccc [other] - 3 minutes ago")},
	})))

	// feature → main → all branches → wrap to feature (three scopes, no my runs).
	assert.Assert(t, t.Run("feature → main", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "bbbbbbb [main]")
	}))
	assert.Assert(t, t.Run("main → all branches", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "ccccccc [other]")
	}))
	assert.Assert(t, t.Run("all branches → feature (wrap, skipping my runs)", func(t *testing.T) {
		tm.Send(switchKey)
		waitForOutput(t, tm, "aaaaaaa [feature] - refetched")
	}))
}

// TestRunGetFlow_InitialScopeMyRuns confirms that InitialScope: ScopeMyRuns opens
// the picker on "my runs" while keeping all scopes available (i.e. the "change trigger"
// hint is visible).
func TestRunGetFlow_InitialScopeMyRuns(t *testing.T) {
	newFlow := func() ui.RunGetFlowModel {
		return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
			Runs:          []ui.RunGetItem{runItem("ddddddd [mine] - 1 minute ago")},
			CurrentBranch: "feature",
			DefaultBranch: "main",
			InitialScope:  ui.ScopeMyRuns,
			FetchRuns: fetchByBranch(map[string][]ui.RunGetItem{
				"feature": {runItem("abcdefg [feature] - 2 minutes ago")},
			}),
			FetchMyRuns: func(context.Context, string, ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
				return []ui.RunGetItem{runItem("ddddddd [mine] - 1 minute ago")}, nil
			},
		})
	}

	assert.Assert(t, t.Run("opens on my runs with other scopes available", func(t *testing.T) {
		v := flowSnapshot(t, startFlow(t, newFlow()))
		assert.Check(t, cmp.Contains(v, "Select a run [my runs]"))
		assert.Check(t, cmp.Contains(v, "ddddddd [mine]"))
		assert.Check(t, cmp.Contains(v, "change trigger")) // we can change out the my runs scope
	}))
	assert.Assert(t, t.Run("toggle from my runs reaches current branch", func(t *testing.T) {
		tm := startFlow(t, newFlow())
		tm.Send(switchKey) // my runs -> feature
		waitForOutput(t, tm, "abcdefg [feature]")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a run [feature]"))
	}))
}

// TestRunGetFlow_InitialScopeMyRunsNoProject confirms that when FetchRuns is nil
// (no project available), InitialScope is effectively forced to ScopeMyRuns and
// the shift+tab toggle is hidden (only one scope exists).
func TestRunGetFlow_InitialScopeMyRunsNoProject(t *testing.T) {
	tm := startFlow(t, ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs: []ui.RunGetItem{runItem("ddddddd [mine] - 1 minute ago")},
		FetchMyRuns: func(context.Context, string, ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{runItem("ddddddd [mine] - 1 minute ago")}, nil
		},
	}))

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "Select a run [my runs]"))
	assert.Check(t, cmp.Contains(v, "ddddddd [mine]"))
	// Only one scope. The change trigger hint should be absent.
	assert.Check(t, !strings.Contains(v, "change trigger"))
}

// TestRunGetFlow_ToggleNoRuns swaps in an empty-state placeholder (committing the
// empty scope, so the title names it) when the toggled-to scope has no runs.
// Committing the empty scope is what keeps cycling from getting stuck.
func TestRunGetFlow_ToggleNoRuns(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(map[string][]ui.RunGetItem{
		"feature": {runItem("aaaaaaa [feature] - 1 minute ago")},
		// "main" unmapped → empty result.
	})))

	assert.Assert(t, t.Run("toggle to a scope with no runs", func(t *testing.T) {
		tm.Send(switchKey) // feature → main (empty)
		waitForOutput(t, tm, "No runs found on main")
	}))

	assert.Assert(t, t.Run("shows the empty-state placeholder under the new scope", func(t *testing.T) {
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "(No runs found on main)"))
		assert.Check(t, cmp.Contains(v, "Select a run [main]"))
		assert.Check(t, !strings.Contains(v, "aaaaaaa [feature]"))
	}))
}

// TestRunGetFlow_RefreshRefetchesActiveScope confirms r re-fetches the active
// branch and swaps in the fresh list without changing scope.
func TestRunGetFlow_RefreshRefetchesActiveScope(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(map[string][]ui.RunGetItem{
		"feature": {runItem("zzzzzzz [feature] - just now")},
	})))

	tm.Send(keyR)
	waitForOutput(t, tm, "zzzzzzz [feature]")
}

// newStatusFlow builds a single-branch (main) run-get flow whose FetchRuns maps
// the active status filter to a run list, with the given selectable statuses on
// the "s" cycle. The initial list is the "all statuses" set.
func newStatusFlow(byStatus map[string][]ui.RunGetItem, statuses []ui.RunStatusFilter) ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          byStatus[""],
		CurrentBranch: "main",
		FetchRuns: func(_ context.Context, _, status string, _ ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
			return byStatus[status], nil
		},
		StatusFilters: statuses,
	})
}

// TestRunGetFlow_StatusFilterCycles drives the "s" key through the status cycle
// (all statuses → canceled → failed), swapping the run list each step, and
// confirms the active status is named in the picker title alongside the scope.
func TestRunGetFlow_StatusFilterCycles(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{
			"":         {runItem("aaaaaaa [main] - all")},
			"canceled": {runItem("bbbbbbb [main] - canceled")},
			"failed":   {runItem("ccccccc [main] - failed")},
		},
		[]ui.RunStatusFilter{
			{Value: "canceled", Label: "canceled"},
			{Value: "failed", Label: "failed"},
		},
	))

	assert.Assert(t, t.Run("all statuses → canceled", func(t *testing.T) {
		tm.Send(keyS)
		waitForOutput(t, tm, "bbbbbbb [main] - canceled")
	}))
	assert.Assert(t, t.Run("canceled → failed", func(t *testing.T) {
		tm.Send(keyS)
		waitForOutput(t, tm, "ccccccc [main] - failed")
	}))
	assert.Assert(t, t.Run("names scope and status in the title", func(t *testing.T) {
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a run [main · failed]"))
	}))
}

// TestRunGetFlow_StatusFilterNoRuns swaps in an empty-state placeholder and names
// the (committed) status in the title when the chosen status has no runs.
func TestRunGetFlow_StatusFilterNoRuns(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{
			"": {runItem("aaaaaaa [main] - all")},
			// "canceled" unmapped → empty result.
		},
		[]ui.RunStatusFilter{{Value: "canceled", Label: "canceled"}},
	))

	assert.Assert(t, t.Run("filter to a status with no runs", func(t *testing.T) {
		tm.Send(keyS)
		waitForOutput(t, tm, "No canceled runs on main")
	}))
	assert.Assert(t, t.Run("shows the empty-state placeholder, status in title", func(t *testing.T) {
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "(No canceled runs on main)"))
		assert.Check(t, cmp.Contains(v, "Select a run [main · canceled]"))
		assert.Check(t, !strings.Contains(v, "aaaaaaa [main] - all"))
	}))
}

// TestRunGetFlow_StatusFilterAdvancesPastEmpty is the regression test for the
// "stuck" bug: pressing "s" onto an empty status must still commit it, so a
// further "s" reaches the following status rather than retrying the empty one.
// With the bug the empty status is not committed, so the second "s" recomputes
// the same empty next and "failed" is never reached.
func TestRunGetFlow_StatusFilterAdvancesPastEmpty(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{
			"":       {runItem("aaaaaaa [main] - all")},
			"failed": {runItem("ccccccc [main] - failed")},
			// "canceled" unmapped → empty result.
		},
		[]ui.RunStatusFilter{
			{Value: "canceled", Label: "canceled"},
			{Value: "failed", Label: "failed"},
		},
	))

	assert.Assert(t, t.Run("all statuses → canceled (empty)", func(t *testing.T) {
		tm.Send(keyS)
		waitForOutput(t, tm, "No canceled runs on main")
	}))
	assert.Assert(t, t.Run("canceled → failed reaches runs (not stuck on canceled)", func(t *testing.T) {
		tm.Send(keyS)
		waitForOutput(t, tm, "ccccccc [main] - failed")
	}))
}

// TestRunGetFlow_StatusFilterResetsWithShiftS confirms capital "S" jumps straight
// back to "all statuses" from any status, rather than cycling one step. The
// all-statuses re-fetch returns a distinct row from the startup list so the
// reset has a unique token to sync on.
func TestRunGetFlow_StatusFilterResetsWithShiftS(t *testing.T) {
	byStatus := map[string][]ui.RunGetItem{
		"":       {runItem("ddddddd [main] - all-refetched")},
		"failed": {runItem("ccccccc [main] - failed")},
	}
	tm := startFlow(t, ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - startup")},
		CurrentBranch: "main",
		FetchRuns: func(_ context.Context, _, status string, _ ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
			return byStatus[status], nil
		},
		StatusFilters: []ui.RunStatusFilter{
			{Value: "canceled", Label: "canceled"},
			{Value: "failed", Label: "failed"},
		},
	}))

	assert.Assert(t, t.Run("cycle to failed", func(t *testing.T) {
		tm.Send(keyS) // all → canceled (empty)
		waitForOutput(t, tm, "No canceled runs on main")
		tm.Send(keyS) // canceled → failed
		waitForOutput(t, tm, "ccccccc [main] - failed")
	}))
	assert.Assert(t, t.Run("shift+S resets to all statuses", func(t *testing.T) {
		tm.Send(keyShiftS)
		// Sync on the contiguous revision token (the "- all-refetched" suffix is
		// spliced with insert-mode escapes in the diffed stream); assert the rest on
		// the fully-rendered snapshot.
		waitForOutput(t, tm, "ddddddd [main]")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "ddddddd [main] - all-refetched"))
		// The title drops the status now that the filter is cleared.
		assert.Check(t, cmp.Contains(v, "Select a run [main]"))
		assert.Check(t, !strings.Contains(v, "· failed"))
	}))
}

// TestRunGetFlow_StatusFilterOmittedWithoutFilters confirms that without
// StatusFilters the footer omits the "s" hint and pressing "s" is a no-op.
func TestRunGetFlow_StatusFilterOmittedWithoutFilters(t *testing.T) {
	tm := startFlow(t, newToggleFlow(fetchByBranch(map[string][]ui.RunGetItem{
		"feature": {runItem("aaaaaaa [feature] - 1 minute ago")},
	})))

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "aaaaaaa [feature]"))
	assert.Check(t, !strings.Contains(v, "s status"))
}

// newStepFlow builds a flow whose run → workflow → job → (single) execution
// chain leads to a step picker with two steps, the second failed. The cursor
// defaults to the failed step. Branch "main" keeps the run-picker title tidy.
func newStepFlow(
	stdout func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error),
	stderr func(context.Context, uuid.UUID, int, int) ([]byte, error),
) ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - now")},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✓", Label: "build"}}, nil
		},
		FetchJobs: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✗", Label: "test"}}, nil
		},
		FetchExecutions: func(context.Context, uuid.UUID) ([]ui.RunGetExecution, error) {
			return []ui.RunGetExecution{{Index: 0, Steps: []ui.RunGetStepItem{
				{Label: "checkout", Icon: "✓", Execution: 0, StepNum: 100},
				{Label: "run tests", Icon: "✗", Execution: 0, StepNum: 101},
			}}}, nil
		},
		FetchStepStdout: stdout,
		FetchStepStderr: stderr,
	})
}

// newTestsFlow builds a single-execution flow whose run → workflow → job chain
// leads to a step picker with one failed step, and whose FetchFailedTests
// returns the given failed tests for the "Failed tests" meta option.
func newTestsFlow(failed []ui.RunGetTestItem) ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - now")},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✓", Label: "build"}}, nil
		},
		FetchJobs: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✗", Label: "test"}}, nil
		},
		FetchExecutions: func(context.Context, uuid.UUID) ([]ui.RunGetExecution, error) {
			return []ui.RunGetExecution{{Index: 0, Steps: []ui.RunGetStepItem{
				{Label: "run tests", Icon: "✗", Execution: 0, StepNum: 101},
			}}}, nil
		},
		FetchFailedTests: func(context.Context, uuid.UUID) ([]ui.RunGetTestItem, error) {
			return failed, nil
		},
	})
}

// driveToFailedTestsPicker navigates run → workflow → job to the step picker,
// then selects the "Failed tests" meta option to open the failed-test picker.
// On the step picker the cursor lands on the failed step ("run tests"), which
// sits just below the four meta rows, so two keyUps reach "Failed tests" (past
// "Resource usage"). It stops after sending the select; callers wait on their own distinctive row (a
// test label, or the empty-state placeholder) since the title/rows arrive in one
// frame that a single WaitFor would consume whole.
func driveToFailedTestsPicker(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Send(keyEnt) // select the only run
	waitForOutput(t, tm, "See all workflows")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "build"
	waitForOutput(t, tm, "All jobs in workflow")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "test"
	waitForOutput(t, tm, "Failed tests")
	tm.Send(keyUp)  // failed step → "Resource usage"
	tm.Send(keyUp)  // → "Failed tests"
	tm.Send(keyEnt) // open the failed-test picker
}

// TestRunGetFlow_FailedTestsPager drives the "Failed tests" meta option, opens
// the first failed test, and confirms its message shows in the pager.
func TestRunGetFlow_FailedTestsPager(t *testing.T) {
	failed := []ui.RunGetTestItem{
		{Icon: "✗", Label: "TestAlpha (pkg/foo)", Message: "alpha boom\nexpected 1 got 2"},
		{Icon: "✗", Label: "TestBravo (pkg/bar)", Message: "bravo boom"},
	}
	tm := startFlow(t, newTestsFlow(failed))

	assert.Assert(t, t.Run("navigate to the failed-test picker", func(t *testing.T) {
		driveToFailedTestsPicker(t, tm)
		waitForOutput(t, tm, "TestAlpha (pkg/foo)")
	}))

	assert.Assert(t, t.Run("open the first failed test's message in the pager", func(t *testing.T) {
		tm.Send(keyEnt) // cursor defaults to the first test
		waitForOutput(t, tm, "expected 1 got 2")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "alpha boom"))
		assert.Check(t, cmp.Contains(v, "esc back"))
	}))
}

// TestRunGetFlow_FailedTestsPagerEscResumes confirms esc from the message pager
// returns to the failed-test picker.
func TestRunGetFlow_FailedTestsPagerEscResumes(t *testing.T) {
	failed := []ui.RunGetTestItem{
		{Icon: "✗", Label: "TestAlpha (pkg/foo)", Message: "alpha boom"},
		{Icon: "✗", Label: "TestBravo (pkg/bar)", Message: "bravo boom"},
	}
	tm := startFlow(t, newTestsFlow(failed))

	assert.Assert(t, t.Run("open a failed test's message", func(t *testing.T) {
		driveToFailedTestsPicker(t, tm)
		waitForOutput(t, tm, "TestAlpha (pkg/foo)")
		tm.Send(keyEnt)
		waitForOutput(t, tm, "alpha boom")
	}))

	assert.Assert(t, t.Run("esc returns to the failed-test picker", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a failed test"))
		assert.Check(t, cmp.Contains(v, "TestBravo (pkg/bar)"))
	}))
}

// TestRunGetFlow_FailedTestsEmpty shows the placeholder row when a job recorded
// no failed tests, and esc returns to the step picker.
func TestRunGetFlow_FailedTestsEmpty(t *testing.T) {
	tm := startFlow(t, newTestsFlow(nil))

	assert.Assert(t, t.Run("placeholder row for no failed tests", func(t *testing.T) {
		driveToFailedTestsPicker(t, tm)
		waitForOutput(t, tm, "no failed tests recorded")
	}))

	assert.Assert(t, t.Run("esc returns to the step picker", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a step"))
	}))
}

// driveToStepPicker selects the only run, then the single workflow and job (each
// picker leads with a "see all" summary option, so the real item is one row
// down), landing on the step picker with the cursor on the failed step.
func driveToStepPicker(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	// Each picker is recognized by a unique, fully-rewritten row rather than its
	// title: titles share the "Select a " prefix and are diff-rewritten in place,
	// so they do not appear contiguously in the output stream.
	tm.Send(keyEnt) // select the only run
	waitForOutput(t, tm, "See all workflows")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "build"
	waitForOutput(t, tm, "All jobs in workflow")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "test"
	waitForOutput(t, tm, "checkout")
}

// TestRunGetFlow_StepPagerStreams selects the failed step and drives the
// streaming pager: stdout arrives over two polled chunks then terminates, after
// which stderr is appended. It asserts the ANSI colors survive (the output is
// replayed through termrender, which re-serializes SGR — "\x1b[31m" becomes the
// normalized "\x1b[0;31m"), the footer reflects streaming vs. done, and every
// chunk lands in the pager.
func TestRunGetFlow_StepPagerStreams(t *testing.T) {
	chunks := [][]byte{
		[]byte("\x1b[31mERROR\x1b[0m first line\n"),
		[]byte("second line\n"),
	}
	terminal := []bool{false, true}
	var n int
	stdout := func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error) {
		i := n
		n++
		if i >= len(chunks) {
			return nil, true, nil
		}
		return chunks[i], terminal[i], nil
	}
	stderr := func(context.Context, uuid.UUID, int, int) ([]byte, error) {
		return []byte("stderr tail\n"), nil
	}

	tm := startFlow(t, newStepFlow(stdout, stderr))

	assert.Assert(t, t.Run("navigate to the failed step", func(t *testing.T) {
		driveToStepPicker(t, tm)
	}))
	assert.Assert(t, t.Run("stream stdout then stderr to completion", func(t *testing.T) {
		// The cursor defaults to the failed step ("run tests"); selecting it
		// streams. The first chunk opens the pager (still streaming); the 2s stdout
		// poll then fires on its own, terminating stdout and triggering the
		// one-shot stderr fetch — the final token to sync on.
		tm.Send(keyEnt)
		waitForOutput(t, tm, "stderr tail")
	}))

	assert.Assert(t, t.Run("the pager shows every chunk and clears the streaming indicator", func(t *testing.T) {
		// teatest's WaitFor consumes the stream, so the content is asserted from
		// the snapshot, which holds the whole accumulated buffer.
		raw := flowFrame(t, tm)
		body := ansi.Strip(raw)

		assert.Check(t, cmp.Contains(body, "ERROR first line"))
		assert.Check(t, cmp.Contains(body, "second line"))
		assert.Check(t, cmp.Contains(body, "stderr tail"))
		assert.Check(t, !strings.Contains(body, "streaming…"), "streaming indicator should clear once terminal")
		assert.Check(t, cmp.Contains(raw, "\x1b[0;31m"), "ANSI colors must be preserved (SGR re-serialized by termrender)")
	}))
}

// TestRunGetFlow_StepPagerCollapsesCarriageReturns confirms that carriage-return
// overwrites in step output (apt/npm-style progress bars that redraw a line many
// times) are collapsed to their final frame. Left intact, lipgloss renders each
// "\r" as a line break, inflating the viewport past its height and pushing the
// footer off the bottom of the screen.
func TestRunGetFlow_StepPagerCollapsesCarriageReturns(t *testing.T) {
	// One logical line ("\n"-terminated) redrawn three times via "\r", plus a
	// CRLF-terminated line whose final redraw is colored green — only the last
	// redraw of each should survive, with its color intact.
	progress := "0% [Working]\r            \rHit:1 archive InRelease\n" +
		"downloading 50%\r\x1b[32mdownloading 100%\x1b[0m\r\n"
	stdout := func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error) {
		return []byte(progress), true, nil
	}
	stderr := func(context.Context, uuid.UUID, int, int) ([]byte, error) { return nil, nil }

	tm := startFlow(t, newStepFlow(stdout, stderr))
	driveToStepPicker(t, tm)
	tm.Send(keyEnt) // open the failed step's output in the pager
	waitForOutput(t, tm, "Hit:1 archive InRelease")

	raw := flowFrame(t, tm)
	body := ansi.Strip(raw)

	assert.Check(t, cmp.Contains(body, "Hit:1 archive InRelease"))
	assert.Check(t, cmp.Contains(body, "downloading 100%"))
	// The intermediate redraws are overwritten, not shown as extra lines.
	assert.Check(t, !strings.Contains(body, "0% [Working]"))
	assert.Check(t, !strings.Contains(body, "downloading 50%"))
	// The surviving frame keeps its color (green foreground).
	assert.Check(t, cmp.Contains(raw, "\x1b[0;32mdownloading 100%"))
}

// TestRunGetFlow_StepPagerEscResumes confirms esc from the pager returns to the
// step picker with the cursor restored to the step that was opened.
func TestRunGetFlow_StepPagerEscResumes(t *testing.T) {
	stdout := func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error) {
		return []byte("output\n"), true, nil
	}
	stderr := func(context.Context, uuid.UUID, int, int) ([]byte, error) { return nil, nil }

	tm := startFlow(t, newStepFlow(stdout, stderr))

	assert.Assert(t, t.Run("open the failed step's output", func(t *testing.T) {
		driveToStepPicker(t, tm)
		tm.Send(keyEnt) // terminal immediately
		waitForOutput(t, tm, "output")
	}))

	assert.Assert(t, t.Run("esc returns to the step picker on the opened step", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a step"))
		assert.Check(t, cmp.Contains(v, "› ✗ run tests"), "cursor should resume on the opened step")
	}))
}

// TestRunGetFlow_FilterDialogOpensOnSlash confirms "/" opens the search dialog
// with its Trigger/Status tabs.
func TestRunGetFlow_FilterDialogOpensOnSlash(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{"": {runItem("aaaaaaa [main] - all")}},
		[]ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	))

	tm.Send(keySlash)
	waitForOutput(t, tm, "Trigger")

	v := flowSnapshot(t, tm)
	for _, want := range []string{"Trigger", "Status", "Filter runs by trigger"} {
		assert.Check(t, cmp.Contains(v, want), "dialog missing %q", want)
	}
}

// TestRunGetFlow_FilterApplyRefetchesRuns drives the dialog to the Status tab,
// picks "failed", applies with enter, and confirms the run list is re-fetched for
// that status.
func TestRunGetFlow_FilterApplyRefetchesRuns(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{
			"":       {runItem("aaaaaaa [main] - all")},
			"failed": {runItem("ccccccc [main] - failed")},
		},
		[]ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	))

	tm.Send(keySlash)
	waitForOutput(t, tm, "Trigger")
	tm.Send(keyRight) // Trigger → Status tab
	tm.Send(keyDown)  // all statuses → failed
	tm.Send(keyEnt)   // apply
	waitForOutput(t, tm, "ccccccc [main] - failed")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "Select a run [main · failed]"))
}

// TestRunGetFlow_FilterCancelReturnsToPicker confirms esc in the dialog returns to
// the run picker with the list unchanged.
func TestRunGetFlow_FilterCancelReturnsToPicker(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{"": {runItem("aaaaaaa [main] - all")}},
		[]ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	))

	tm.Send(keySlash)
	waitForOutput(t, tm, "Trigger")
	tm.Send(keyEsc)
	waitForOutput(t, tm, "Select a run")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "aaaaaaa [main] - all"))
	assert.Check(t, !strings.Contains(v, "Trigger"), "dialog should be dismissed")
}

// TestRunGetFlow_FilterHintShownWhenEnabled confirms the run picker footer
// advertises "/ search" when a branch scope or status filter is available.
func TestRunGetFlow_FilterHintShownWhenEnabled(t *testing.T) {
	v := flowSnapshot(t, startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{"": {runItem("aaaaaaa [main] - all")}},
		[]ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	)))
	assert.Check(t, cmp.Contains(v, "/ search"))
}

// TestRunGetFlow_FilterApplyCreated drives the dialog to the Created tab, selects
// an age, applies, and confirms the created filter reaches FetchRuns and is named
// in the picker title.
func TestRunGetFlow_FilterApplyCreated(t *testing.T) {
	var gotCreated ui.RunCreatedFilter
	m := ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - all")},
		CurrentBranch: "main",
		FetchRuns: func(_ context.Context, _, _ string, created ui.RunCreatedFilter) ([]ui.RunGetItem, error) {
			gotCreated = created
			return []ui.RunGetItem{runItem("ccccccc [main] - recent")}, nil
		},
		StatusFilters: []ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	})
	tm := startFlow(t, m)

	tm.Send(keySlash)
	waitForOutput(t, tm, "Trigger")
	tm.Send(keyRight) // Trigger → Status
	tm.Send(keyRight) // Status → Created
	waitForOutput(t, tm, "1 Hour")
	tm.Send(keyDown) // all dates → 1 Hour (the cursor is the selection)
	tm.Send(keyEnt)  // apply
	waitForOutput(t, tm, "ccccccc [main] - recent")

	assert.Check(t, gotCreated.Active())
	assert.Check(t, cmp.Equal(gotCreated.Duration, time.Hour))
	assert.Check(t, !gotCreated.Newer, "direction defaults to older")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "older than 1 Hour"))
}

// newSummaryFlow builds a single-execution flow (run → workflow "build" → job
// "test" → step picker) with every summary renderer wired, so the "see all
// workflows", "all jobs in workflow", "job report", "full job report" and
// "resource usage" options
// open their summary in an in-flow pager whose esc returns to the offering picker
// rather than quitting. RenderMarkdown returns the markdown verbatim so tests
// can assert on the distinctive body each renderer emits. The step picker has a
// passing "checkout" step and a failed "run tests" step (the default cursor).
func newSummaryFlow() ui.RunGetFlowModel {
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - now")},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✓", Label: "build"}}, nil
		},
		FetchJobs: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✗", Label: "test"}}, nil
		},
		FetchExecutions: func(context.Context, uuid.UUID) ([]ui.RunGetExecution, error) {
			return []ui.RunGetExecution{{Index: 0, Steps: []ui.RunGetStepItem{
				{Label: "checkout", Icon: "✓", Execution: 0, StepNum: 100},
				{Label: "run tests", Icon: "✗", Execution: 0, StepNum: 101},
			}}}, nil
		},
		RenderRunSummary:      func(context.Context, uuid.UUID) (string, error) { return "RUN-SUMMARY-BODY", nil },
		RenderWorkflowSummary: func(context.Context, uuid.UUID) (string, error) { return "WORKFLOW-SUMMARY-BODY", nil },
		RenderJobSummary:      func(context.Context, uuid.UUID) (string, error) { return "JOB-REPORT-BODY", nil },
		RenderJobOutput:       func(context.Context, uuid.UUID) (string, error) { return "FULL-JOB-OUTPUT-BODY", nil },
		RenderResourceUsage:   func(context.Context, uuid.UUID) (string, error) { return "RESOURCE-USAGE-BODY", nil },
		RenderMarkdown:        func(md string, _ int) string { return md },
	})
}

// TestRunGetFlow_RunSummaryPager confirms "see all workflows" opens the run
// summary in an in-flow pager and esc returns to the workflow picker (rather than
// quitting the flow, as it did before the pager was embedded).
func TestRunGetFlow_RunSummaryPager(t *testing.T) {
	tm := startFlow(t, newSummaryFlow())

	assert.Assert(t, t.Run("see all workflows opens the run summary", func(t *testing.T) {
		tm.Send(keyEnt) // select the only run → workflow picker
		waitForOutput(t, tm, "See all workflows")
		tm.Send(keyEnt) // "see all workflows" is the leading row
		waitForOutput(t, tm, "RUN-SUMMARY-BODY")
	}))

	assert.Assert(t, t.Run("esc returns to the workflow picker", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a workflow"))
		assert.Check(t, cmp.Contains(v, "See all workflows"))
	}))
}

// TestRunGetFlow_WorkflowSummaryPager confirms "all jobs in workflow" opens the
// workflow summary in an in-flow pager and esc returns to the job picker.
func TestRunGetFlow_WorkflowSummaryPager(t *testing.T) {
	tm := startFlow(t, newSummaryFlow())

	assert.Assert(t, t.Run("all jobs in workflow opens the workflow summary", func(t *testing.T) {
		tm.Send(keyEnt) // run → workflow picker
		waitForOutput(t, tm, "See all workflows")
		tm.Send(keyDown)
		tm.Send(keyEnt) // "build" → job picker
		waitForOutput(t, tm, "All jobs in workflow")
		tm.Send(keyEnt) // "all jobs in workflow" is the leading row
		waitForOutput(t, tm, "WORKFLOW-SUMMARY-BODY")
	}))

	assert.Assert(t, t.Run("esc returns to the job picker", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a job"))
		assert.Check(t, cmp.Contains(v, "All jobs in workflow"))
	}))
}

// TestRunGetFlow_JobReportPager confirms the "job report" option on the step
// picker opens the job report in an in-flow pager and esc returns to the step
// picker.
func TestRunGetFlow_JobReportPager(t *testing.T) {
	tm := startFlow(t, newSummaryFlow())

	assert.Assert(t, t.Run("job report opens in the pager", func(t *testing.T) {
		driveToStepPicker(t, tm)
		// The cursor starts on the failed step (below the four meta rows); five ups
		// reach the first option, "Job report".
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyEnt)
		waitForOutput(t, tm, "JOB-REPORT-BODY")
	}))

	assert.Assert(t, t.Run("esc returns to the step picker", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	}))
}

// newQueuedJobFlow builds a flow whose workflow holds a started job ("test") and
// one that has not started yet ("deploy", queued). Only the started job has
// executions: a queued job reports none, so picking it has no steps to show.
func newQueuedJobFlow() ui.RunGetFlowModel {
	started := ui.RunGetItem{ID: uuid.New(), Icon: "●", Label: "test"}
	queued := ui.RunGetItem{ID: uuid.New(), Icon: "○", Label: "deploy", Pending: "queued"}
	return ui.NewRunGetFlow(context.Background(), ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - now")},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "●", Label: "build"}}, nil
		},
		FetchJobs: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{started, queued}, nil
		},
		FetchExecutions: func(_ context.Context, jobID uuid.UUID) ([]ui.RunGetExecution, error) {
			if jobID != started.ID {
				return nil, nil
			}
			return []ui.RunGetExecution{{Index: 0, Steps: []ui.RunGetStepItem{
				{Label: "checkout", Icon: "✓", Execution: 0, StepNum: 100},
			}}}, nil
		},
		RenderJobSummary: func(context.Context, uuid.UUID) (string, error) { return "QUEUED-JOB-REPORT-BODY", nil },
		RenderMarkdown:   func(md string, _ int) string { return md },
	})
}

// driveToQueuedJob selects the only run, then the single workflow, then moves the
// cursor onto the queued job — two rows down from the leading "all jobs in
// workflow" option, past the started job.
func driveToQueuedJob(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Send(keyEnt) // select the only run
	waitForOutput(t, tm, "See all workflows")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "build"
	waitForOutput(t, tm, "All jobs in workflow")
	tm.Send(keyDown) // → "test"
	tm.Send(keyDown) // → "deploy (queued)"
}

// TestRunGetFlow_JobPickerLabelsQueuedJob confirms a job that has not started
// carries its status in the label, so it does not read like the started job
// beside it (whose label stays bare).
func TestRunGetFlow_JobPickerLabelsQueuedJob(t *testing.T) {
	tm := startFlow(t, newQueuedJobFlow())
	driveToQueuedJob(t, tm)

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "deploy (queued)"))
	assert.Check(t, cmp.Contains(v, "test"))
	assert.Check(t, !strings.Contains(v, "test ("))
}

// TestRunGetFlow_QueuedJobReportPagesInFlow confirms a job with no steps — a
// queued one — opens its job report in the in-flow pager, so esc returns to the
// job picker instead of the flow quitting and printing the report.
func TestRunGetFlow_QueuedJobReportPagesInFlow(t *testing.T) {
	tm := startFlow(t, newQueuedJobFlow())

	assert.Assert(t, t.Run("the queued job opens its report in the pager", func(t *testing.T) {
		driveToQueuedJob(t, tm)
		tm.Send(keyEnt)
		waitForOutput(t, tm, "QUEUED-JOB-REPORT-BODY")
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "esc back"))
	}))

	assert.Assert(t, t.Run("esc returns to the job picker", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Select a job"))
		// Back on the row it was opened from.
		assert.Check(t, cmp.Contains(v, "› ○ deploy (queued)"))
	}))
}

// TestRunGetFlow_JobPickerOpensStartedJob is the counterpart: a job that has
// started still leads to its step picker.
func TestRunGetFlow_JobPickerOpensStartedJob(t *testing.T) {
	tm := startFlow(t, newQueuedJobFlow())

	tm.Send(keyEnt) // select the only run
	waitForOutput(t, tm, "See all workflows")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "build"
	waitForOutput(t, tm, "All jobs in workflow")
	tm.Send(keyDown) // → "test"
	tm.Send(keyEnt)
	waitForOutput(t, tm, "checkout")

	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
}

// TestRunGetFlow_FullJobOutputPager confirms the "full job report" option opens
// the full per-step output in an in-flow pager and esc returns to the step
// picker.
func TestRunGetFlow_FullJobOutputPager(t *testing.T) {
	tm := startFlow(t, newSummaryFlow())

	assert.Assert(t, t.Run("full job report opens in the pager", func(t *testing.T) {
		driveToStepPicker(t, tm)
		// Four ups from the failed step reach the second option, "Full job report".
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyEnt)
		waitForOutput(t, tm, "FULL-JOB-OUTPUT-BODY")
	}))

	assert.Assert(t, t.Run("esc returns to the step picker", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	}))
}

// TestRunGetFlow_ResourceUsagePager confirms the "resource usage" option on the
// step picker opens the usage charts in an in-flow pager and esc returns to the
// step picker. It is the last of the four meta options, so two ups from the
// failed step reach it — one past the passing "checkout" step above it.
func TestRunGetFlow_ResourceUsagePager(t *testing.T) {
	tm := startFlow(t, newSummaryFlow())

	assert.Assert(t, t.Run("resource usage opens in the pager", func(t *testing.T) {
		driveToStepPicker(t, tm)
		tm.Send(keyUp)
		tm.Send(keyUp)
		tm.Send(keyEnt)
		waitForOutput(t, tm, "RESOURCE-USAGE-BODY")
	}))

	assert.Assert(t, t.Run("esc returns to the step picker", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	}))
}

// waitForFrame blocks until the flow's current frame contains s, polling
// snapshots rather than reading the output stream. bubbletea repaints only what
// changed between frames, so a line that is still on screen (a title, a row that
// did not move) may never appear in the stream again — waitForOutput would hang
// on it even though the screen reads correctly.
func waitForFrame(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	deadline := time.Now().Add(teaTimeout)
	for {
		if frame := flowSnapshot(t, tm); strings.Contains(frame, s) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("frame never contained %q after %s; last frame:\n%s", s, teaTimeout, flowSnapshot(t, tm))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- the artifact browser ---

// artifactFlow is the set of artifact stubs a browser test wires up. A nil field
// means "the caller did not supply this closure", which is exactly how the flow
// decides which keys to offer.
type artifactFlow struct {
	items    []ui.RunGetArtifactItem
	fetchErr error
	// content is the bytes a view returns, text whether they are displayable.
	content     []byte
	contentText bool
	contentErr  error
	// downloaded records what a download was asked to write, and downloadErr the
	// error it reports.
	downloaded  *[]ui.RunGetArtifactItem
	downloadDir *string
	downloadErr error
	opened      *string
	// jobIcon is the picked job's status glyph — a finished job is what makes the
	// artifacts option appear at all.
	jobIcon string
	// noContent and noDownload drop the respective closures, so the flow sees them
	// as unwired.
	noContent  bool
	noDownload bool
	noOpen     bool
}

func artifactItem(path string) ui.RunGetArtifactItem {
	return ui.RunGetArtifactItem{Path: path, URL: "https://artifacts.example/" + path}
}

// newArtifactsFlow builds a single-execution flow whose run → workflow → job
// chain leads to a step picker offering the artifacts option.
func newArtifactsFlow(f artifactFlow) ui.RunGetFlowModel {
	icon := f.jobIcon
	if icon == "" {
		icon = "✓"
	}
	opts := ui.RunGetFlowOptions{
		Runs:          []ui.RunGetItem{runItem("aaaaaaa [main] - now")},
		CurrentBranch: "main",
		FetchWorkflows: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: "✓", Label: "build"}}, nil
		},
		FetchJobs: func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
			return []ui.RunGetItem{{ID: uuid.New(), Icon: icon, Label: "test"}}, nil
		},
		FetchExecutions: func(context.Context, uuid.UUID) ([]ui.RunGetExecution, error) {
			return []ui.RunGetExecution{{Index: 0, Steps: []ui.RunGetStepItem{
				{Label: "run tests", Icon: "✗", Execution: 0, StepNum: 101},
			}}}, nil
		},
		FetchArtifacts: func(context.Context, uuid.UUID) ([]ui.RunGetArtifactItem, error) {
			return f.items, f.fetchErr
		},
	}
	if !f.noContent {
		opts.FetchArtifactContent = func(context.Context, ui.RunGetArtifactItem) ([]byte, bool, error) {
			return f.content, f.contentText, f.contentErr
		}
	}
	if !f.noDownload {
		opts.DownloadArtifacts = func(_ context.Context, items []ui.RunGetArtifactItem, dir string) error {
			if f.downloaded != nil {
				*f.downloaded = items
			}
			if f.downloadDir != nil {
				*f.downloadDir = dir
			}
			return f.downloadErr
		}
	}
	if !f.noOpen {
		opts.OpenArtifactURL = func(url string) error {
			if f.opened != nil {
				*f.opened = url
			}
			return nil
		}
	}
	return ui.NewRunGetFlow(context.Background(), opts)
}

// driveToArtifacts navigates run → workflow → job to the step picker and opens
// the artifacts option. The cursor lands on the failed step, which sits just
// below the five meta rows, so one keyUp reaches "Artifacts" (the last of them).
func driveToArtifacts(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Send(keyEnt) // select the only run
	waitForOutput(t, tm, "See all workflows")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "build"
	waitForOutput(t, tm, "All jobs in workflow")
	tm.Send(keyDown)
	tm.Send(keyEnt) // select "test"
	waitForOutput(t, tm, "Artifacts (browse and download files)")
	tm.Send(keyUp)  // failed step → "Artifacts"
	tm.Send(keyEnt) // open the browser
}

var browsePaths = []ui.RunGetArtifactItem{
	artifactItem("coverage.out"),
	artifactItem("reports/junit/results.xml"),
	artifactItem("reports/summary.txt"),
}

// TestRunGetFlow_ArtifactBrowse walks the browser: the tree opens at the root
// with directories first, → descends, ← comes back, and esc at the root returns
// to the picker that offered it.
func TestRunGetFlow_ArtifactBrowse(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{items: browsePaths}))

	assert.Assert(t, t.Run("the tree opens at the root", func(t *testing.T) {
		driveToArtifacts(t, tm)
		waitForFrame(t, tm, "Artifacts /")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "reports/  (2 files)"))
		assert.Check(t, cmp.Contains(v, "coverage.out"))
		assert.Check(t, cmp.Contains(v, "d download"))
		assert.Check(t, cmp.Contains(v, "o browser"))
	}))

	assert.Assert(t, t.Run("→ descends into a directory", func(t *testing.T) {
		tm.Send(keyRight)
		waitForFrame(t, tm, "Artifacts /reports")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "junit/  (1 file)"))
		assert.Check(t, cmp.Contains(v, "summary.txt"))
	}))

	assert.Assert(t, t.Run("← comes back out", func(t *testing.T) {
		tm.Send(keyLeft)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Artifacts /"))
	}))

	assert.Assert(t, t.Run("esc at the root returns to the step picker", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	}))
}

// TestRunGetFlow_ArtifactFilter drives the "/" filter: it narrows the listing
// recursively from the current directory, and esc clears it before esc leaves.
func TestRunGetFlow_ArtifactFilter(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{items: browsePaths}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("typing narrows the listing", func(t *testing.T) {
		tm.Send(keySlash)
		tm.Send(tea.KeyPressMsg{Code: 'x', Text: "x"})
		tm.Send(tea.KeyPressMsg{Code: 'm', Text: "m"})
		tm.Send(tea.KeyPressMsg{Code: 'l', Text: "l"})
		waitForFrame(t, tm, "1 match")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "reports/junit/results.xml"))
		assert.Check(t, !strings.Contains(v, "coverage.out"))
	}))

	assert.Assert(t, t.Run("enter commits the filter", func(t *testing.T) {
		tm.Send(keyEnt)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "/xml"))
	}))

	assert.Assert(t, t.Run("esc clears the filter, keeping the browser open", func(t *testing.T) {
		tm.Send(keyEsc)
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "coverage.out"))
		assert.Check(t, !strings.Contains(v, "/xml"))
	}))
}

// TestRunGetFlow_ArtifactViewInPager opens a text artifact in the pager, and
// confirms esc returns to the browser rather than the step picker.
func TestRunGetFlow_ArtifactViewInPager(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:       browsePaths,
		content:     []byte("mode: set\ntotal coverage 91.2%"),
		contentText: true,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("enter on a file pages its content", func(t *testing.T) {
		tm.Send(keyDown) // reports/ → coverage.out
		tm.Send(keyEnt)
		waitForFrame(t, tm, "total coverage 91.2%")
	}))

	assert.Assert(t, t.Run("esc returns to the browser", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Artifacts /"))
	}))
}

// TestRunGetFlow_ArtifactImageRendered confirms an image artifact is drawn in the
// pager as a block mosaic rather than refused as binary, named in the footer, and
// fitted to the window so it needs no scrolling — a mosaic keeps mosaic's trailing
// newline otherwise, which costs a row and scrolls at every terminal size.
func TestRunGetFlow_ArtifactImageRendered(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:   []ui.RunGetArtifactItem{artifactItem("screenshots/login.png")},
		content: flowTestPNG(t, 120, 60),
		// An image is not text: the flow classifies it from the bytes, so the text
		// flag being false must not stop it rendering.
		contentText: false,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("the image renders, named in the footer", func(t *testing.T) {
		tm.Send(keyRight) // into screenshots/
		tm.Send(keyEnt)   // view login.png
		waitForFrame(t, tm, "png 120×60")
		frame := flowSnapshot(t, tm)
		assert.Check(t, strings.ContainsAny(frame, "▀▄█▌▐"), "no block glyphs in %q", frame)
		assert.Check(t, !strings.Contains(frame, "not text"))
	}))

	assert.Assert(t, t.Run("it fits the window, so nothing scrolls", func(t *testing.T) {
		// The pager reports its scroll position: 100% with the viewport still at the
		// top means every row is already on screen.
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "100%"))
	}))

	assert.Assert(t, t.Run("a resize re-renders it to fit again", func(t *testing.T) {
		tm.Send(tea.WindowSizeMsg{Width: 40, Height: 12})
		waitForFrame(t, tm, "png 120×60")
		frame := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(frame, "100%"))
		for _, row := range mosaicRows(frame) {
			assert.Check(t, ansi.StringWidth(row) <= 40, "image row wider than the terminal: %q", row)
		}
	}))

	assert.Assert(t, t.Run("esc returns to the browser", func(t *testing.T) {
		tm.Send(keyEsc)
		waitForFrame(t, tm, "Artifacts /screenshots")
	}))
}

// TestRunGetFlow_ArtifactBinaryNotPaged confirms a file that is not displayable
// text is never paged: the browser says so and points at the download key.
func TestRunGetFlow_ArtifactBinaryNotPaged(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:       []ui.RunGetArtifactItem{artifactItem("build/app.bin")},
		content:     []byte{0x7f, 'E', 'L', 'F'},
		contentText: false,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	t.Run("the browser names the file and points at the download key", func(t *testing.T) {
		tm.Send(keyRight) // into build/
		tm.Send(keyEnt)   // open app.bin
		waitForFrame(t, tm, "not text or a supported image")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "press d to download"))
		assert.Check(t, cmp.Contains(v, "app.bin"))
	})
}

// TestRunGetFlow_ArtifactDownload confirms the download key confirms first, then
// writes the whole highlighted directory to ./artifacts and reports the outcome.
func TestRunGetFlow_ArtifactDownload(t *testing.T) {
	var got []ui.RunGetArtifactItem
	var dir string
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:       browsePaths,
		downloaded:  &got,
		downloadDir: &dir,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("d asks before writing anything", func(t *testing.T) {
		tm.Send(keyD)
		waitForFrame(t, tm, "[y/N]")
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Download 2 files from reports/ to ./artifacts?"))
		assert.Check(t, cmp.Equal(len(got), 0))
	}))

	assert.Assert(t, t.Run("y downloads the directory", func(t *testing.T) {
		tm.Send(keyY)
		waitForFrame(t, tm, "Downloaded 2 files to ./artifacts")
		assert.Check(t, cmp.DeepEqual(artifactItemPaths(got), []string{
			"reports/junit/results.xml",
			"reports/summary.txt",
		}))
		assert.Check(t, cmp.Equal(dir, "./artifacts"))
	}))
}

// TestRunGetFlow_ArtifactDownloadCancelled confirms answering no writes nothing.
func TestRunGetFlow_ArtifactDownloadCancelled(t *testing.T) {
	var got []ui.RunGetArtifactItem
	tm := startFlow(t, newArtifactsFlow(artifactFlow{items: browsePaths, downloaded: &got}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	t.Run("answering no writes nothing", func(t *testing.T) {
		tm.Send(keyD)
		waitForFrame(t, tm, "[y/N]")
		tm.Send(keyN)
		waitForFrame(t, tm, "Download cancelled")
		assert.Check(t, cmp.Equal(len(got), 0))
	})
}

// TestRunGetFlow_ArtifactDownloadError reports a failed download in the browser
// rather than ending the flow.
func TestRunGetFlow_ArtifactDownloadError(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:       browsePaths,
		downloadErr: errors.New("disk full"),
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	t.Run("the failure is reported in the browser, which stays open", func(t *testing.T) {
		tm.Send(keyD)
		waitForFrame(t, tm, "[y/N]")
		tm.Send(keyY)
		waitForFrame(t, tm, "disk full")
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Artifacts /"))
	})
}

// TestRunGetFlow_ArtifactOpenInBrowser confirms "o" opens the highlighted file's
// URL, and that a directory row says why it cannot.
func TestRunGetFlow_ArtifactOpenInBrowser(t *testing.T) {
	var opened string
	tm := startFlow(t, newArtifactsFlow(artifactFlow{items: browsePaths, opened: &opened}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("a directory has no URL of its own", func(t *testing.T) {
		tm.Send(keyO)
		waitForFrame(t, tm, "Only a file can be opened in a browser")
		assert.Check(t, cmp.Equal(opened, ""))
	}))

	assert.Assert(t, t.Run("a file opens", func(t *testing.T) {
		tm.Send(keyDown) // reports/ → coverage.out
		tm.Send(keyO)
		waitForFrame(t, tm, "Opened coverage.out in your browser")
		assert.Check(t, cmp.Equal(opened, "https://artifacts.example/coverage.out"))
	}))
}

// TestRunGetFlow_ArtifactsPerExecution confirms a job whose artifacts span
// parallel executions groups them under one directory per execution, mirroring
// what a download writes to disk.
func TestRunGetFlow_ArtifactsPerExecution(t *testing.T) {
	var got []ui.RunGetArtifactItem
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items: []ui.RunGetArtifactItem{
			{Path: "junit.xml", URL: "https://artifacts.example/0/junit.xml", Execution: 0},
			{Path: "junit.xml", URL: "https://artifacts.example/3/junit.xml", Execution: 3},
		},
		downloaded: &got,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	assert.Assert(t, t.Run("the root lists one directory per execution", func(t *testing.T) {
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "exec-0000/  (1 file)"))
		assert.Check(t, cmp.Contains(v, "exec-0003/  (1 file)"))
	}))

	assert.Assert(t, t.Run("a download from one execution keeps its directory", func(t *testing.T) {
		// Otherwise the two same-named files would collide on disk.
		tm.Send(keyDown) // exec-0000 → exec-0003
		tm.Send(keyD)
		waitForFrame(t, tm, "[y/N]")
		tm.Send(keyY)
		waitForFrame(t, tm, "Downloaded 1 file to ./artifacts")
		assert.Check(t, cmp.DeepEqual(artifactItemPaths(got), []string{"exec-0003/junit.xml"}))
	}))
}

// TestRunGetFlow_ArtifactsEmpty confirms a job that produced no artifacts opens a
// browser that explains itself, with esc back to the picker.
func TestRunGetFlow_ArtifactsEmpty(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{}))
	driveToArtifacts(t, tm)

	assert.Assert(t, t.Run("the browser explains itself", func(t *testing.T) {
		waitForFrame(t, tm, "This job produced no artifacts")
	}))

	t.Run("esc returns to the picker that offered it", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	})
}

// TestRunGetFlow_ArtifactsFetchError shows a failed listing in the browser rather
// than ending the flow, so esc still gets the user back.
func TestRunGetFlow_ArtifactsFetchError(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{fetchErr: errors.New("artifacts unavailable")}))
	driveToArtifacts(t, tm)

	assert.Assert(t, t.Run("the error shows in the browser", func(t *testing.T) {
		waitForFrame(t, tm, "artifacts unavailable")
	}))

	t.Run("esc still gets back to the picker", func(t *testing.T) {
		tm.Send(keyEsc)
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Select a step"))
	})
}

// TestRunGetFlow_ArtifactsOptionNeedsFinishedJob confirms the option is offered
// only for a job that has finished: a running job has no artifacts to browse.
func TestRunGetFlow_ArtifactsOptionNeedsFinishedJob(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{items: browsePaths, jobIcon: "●"}))

	assert.Assert(t, t.Run("drill into the running job", func(t *testing.T) {
		tm.Send(keyEnt)
		waitForFrame(t, tm, "See all workflows")
		tm.Send(keyDown)
		tm.Send(keyEnt)
		waitForFrame(t, tm, "All jobs in workflow")
		tm.Send(keyDown)
		tm.Send(keyEnt)
		waitForFrame(t, tm, "Select a step")
	}))

	t.Run("the step picker offers the summaries but no artifacts row", func(t *testing.T) {
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "Resource usage"))
		assert.Check(t, !strings.Contains(v, "Artifacts"), "artifacts offered for an unfinished job")
	})
}

// TestRunGetFlow_ArtifactActionKeysFollowOptions confirms the footer advertises
// only the actions the caller wired up.
func TestRunGetFlow_ArtifactActionKeysFollowOptions(t *testing.T) {
	tm := startFlow(t, newArtifactsFlow(artifactFlow{
		items:      browsePaths,
		noDownload: true,
		noOpen:     true,
	}))
	driveToArtifacts(t, tm)
	waitForFrame(t, tm, "Artifacts /")

	t.Run("the footer omits the actions the caller did not wire up", func(t *testing.T) {
		v := flowSnapshot(t, tm)
		assert.Check(t, cmp.Contains(v, "/ search"))
		assert.Check(t, !strings.Contains(v, "d download"), "download offered with no downloader")
		assert.Check(t, !strings.Contains(v, "o browser"), "browser offered with no opener")
	})
}

// mosaicRows returns the image rows of a pager frame: the lines carrying block
// glyphs, with the frame's right-hand padding stripped. lipgloss pads every line
// of a frame to the width of its widest block — which is the footer's key hints,
// not the image — so the padding has to go before a row can be measured.
func mosaicRows(frame string) []string {
	var rows []string
	for _, line := range strings.Split(frame, "\n") {
		if strings.ContainsAny(line, "▀▄█▌▐") {
			rows = append(rows, strings.TrimRight(line, " "))
		}
	}
	return rows
}

// flowTestPNG encodes a w×h gradient PNG for the image-viewing test.
func flowTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	assert.NilError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func artifactItemPaths(items []ui.RunGetArtifactItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Path
	}
	return out
}
