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
)

// quitMsg tells flowHarness to end the program. The flow ignores unknown message
// types, so sending it does not perturb the model's state — the harness quits
// the program with the inner model parked on its current (live) stage, so its
// View can be snapshotted. (The flow's own quit paths switch to a "done" stage
// that renders an empty frame, which would defeat a snapshot.)
type quitMsg struct{}

// flowHarness drives a RunGetFlowModel as a standalone teatest program and quits
// on quitMsg without disturbing the inner model.
type flowHarness struct {
	m ui.RunGetFlowModel
}

func (h flowHarness) Init() tea.Cmd { return h.m.Init() }

func (h flowHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(quitMsg); ok {
		return h, tea.Quit
	}
	u, cmd := h.m.Update(msg)
	h.m = u.(ui.RunGetFlowModel)
	return h, cmd
}

func (h flowHarness) View() tea.View { return h.m.View() }

// startFlow runs a flow at a known terminal size and waits for the run picker.
func startFlow(t *testing.T, m ui.RunGetFlowModel) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, flowHarness{m: m}, teatest.WithInitialTermSize(80, 24))
	waitForOutput(t, tm, "Select a run")
	return tm
}

// waitForOutput blocks until the program's cumulative output contains s. The
// timeout is generous so the streaming pager's 2s stdout poll has time to fire;
// it returns as soon as the substring appears, so fast assertions stay fast.
func waitForOutput(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}, teatest.WithDuration(4*time.Second))
}

// flowSnapshot quits via quitMsg and returns the inner model's final,
// ANSI-stripped frame.
func flowSnapshot(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(quitMsg{})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(flowHarness)
	return ansi.Strip(fm.m.View().Content)
}

func runItem(label string) ui.RunGetItem {
	return ui.RunGetItem{ID: uuid.New(), Icon: "✓", Label: label}
}

// fetchByBranch returns a FetchRuns that maps a branch filter ("" = all
// branches) to its run list, returning an empty list for unmapped branches. The
// status argument is ignored (status filtering is covered separately).
func fetchByBranch(byBranch map[string][]ui.RunGetItem) func(context.Context, string, string) ([]ui.RunGetItem, error) {
	return func(_ context.Context, branch, _ string) ([]ui.RunGetItem, error) {
		return byBranch[branch], nil
	}
}

// newToggleFlow builds a run-get flow on branch "feature" with default branch
// "main". Animation is off so the loading command is the bare fetch (no spinner
// tick), keeping the program loop deterministic under teatest.
func newToggleFlow(fetch func(context.Context, string, string) ([]ui.RunGetItem, error)) ui.RunGetFlowModel {
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
		FetchMyRuns: func(context.Context, string) ([]ui.RunGetItem, error) {
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
		FetchRuns: func(_ context.Context, _, status string) ([]ui.RunGetItem, error) {
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
		FetchRuns: func(_ context.Context, _, status string) ([]ui.RunGetItem, error) {
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
// sits just below the three meta rows, so one keyUp reaches "Failed tests". It
// stops after sending the select; callers wait on their own distinctive row (a
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
	tm.Send(keyUp)  // failed step → "Failed tests"
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
		tm.Send(quitMsg{})
		fm := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(flowHarness)
		raw := fm.m.View().Content
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

	tm.Send(quitMsg{})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(flowHarness)
	raw := fm.m.View().Content
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
// with its Branch/Status tabs and OK/Cancel/Reset buttons.
func TestRunGetFlow_FilterDialogOpensOnSlash(t *testing.T) {
	tm := startFlow(t, newStatusFlow(
		map[string][]ui.RunGetItem{"": {runItem("aaaaaaa [main] - all")}},
		[]ui.RunStatusFilter{{Value: "failed", Label: "failed"}},
	))

	tm.Send(keySlash)
	waitForOutput(t, tm, "Cancel")

	v := flowSnapshot(t, tm)
	for _, want := range []string{"Branch", "Status", "OK", "Cancel", "Reset"} {
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
	waitForOutput(t, tm, "Cancel")
	tm.Send(keyRight) // Branch → Status tab
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
	waitForOutput(t, tm, "Cancel")
	tm.Send(keyEsc)
	waitForOutput(t, tm, "Select a run")

	v := flowSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "aaaaaaa [main] - all"))
	assert.Check(t, !strings.Contains(v, "Branch"), "dialog should be dismissed")
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
