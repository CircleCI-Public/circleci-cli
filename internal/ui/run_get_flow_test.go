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
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func runItem(label string) RunGetItem {
	return RunGetItem{ID: uuid.New(), Icon: "✓", Label: label}
}

// view renders the model with ANSI styling stripped, so substring assertions are
// stable regardless of the platform's lipgloss color profile (which on Windows
// can insert escape codes that split an asserted substring).
func view(m RunGetFlowModel) string {
	return ansi.Strip(m.View().Content)
}

var (
	// switchKey cycles the run picker to the next branch scope. It is the
	// platform's bound key: shift+tab normally, plain Tab on Windows.
	switchKey = func() tea.KeyPressMsg {
		if switchScopeKey == "shift+tab" {
			return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		}
		return tea.KeyPressMsg{Code: tea.KeyTab}
	}()
	// keyR refreshes the current stage's data.
	keyR = tea.KeyPressMsg{Code: 'r', Text: "r"}
)

// fetchByBranch returns a FetchRuns that maps a branch filter ("" = all
// branches) to its run list, returning an empty list for unmapped branches.
func fetchByBranch(byBranch map[string][]RunGetItem) func(context.Context, string) ([]RunGetItem, error) {
	return func(_ context.Context, branch string) ([]RunGetItem, error) {
		return byBranch[branch], nil
	}
}

// newToggleFlow builds a run-get flow on branch "feature" with default branch
// "main". Animation is left off, so loadingCmd returns the bare fetch command
// and tests can execute it directly without unwrapping a spinner batch.
func newToggleFlow(fetch func(context.Context, string) ([]RunGetItem, error)) RunGetFlowModel {
	return NewRunGetFlow(context.Background(), RunGetFlowOptions{
		Runs:          []RunGetItem{runItem("aaaaaaa [feature] - 1 minute ago")},
		CurrentBranch: "feature",
		DefaultBranch: "main",
		FetchRuns:     fetch,
	})
}

// applyRunsFetch drives a key press that triggers a runs fetch: it confirms the
// model entered the loading stage, executes the (bare, animation-off) fetch
// command, feeds the resulting message back, and returns the updated model.
func applyRunsFetch(t *testing.T, m RunGetFlowModel, key tea.Msg) RunGetFlowModel {
	t.Helper()
	updated, cmd := m.Update(key)
	m = updated.(RunGetFlowModel)
	assert.Equal(t, m.stage, runGetStageLoadingRuns)
	assert.Assert(t, cmd != nil)

	msg := cmd()
	runs, ok := msg.(runGetRunsMsg)
	assert.Assert(t, ok, "expected runGetRunsMsg, got %T", msg)

	updated, _ = m.Update(runs)
	return updated.(RunGetFlowModel)
}

// TestRunGetFlow_TitleNamesActiveScope shows the active scope, bracketed, in the
// picker title.
func TestRunGetFlow_TitleNamesActiveScope(t *testing.T) {
	assert.Check(t, cmp.Contains(view(newToggleFlow(fetchByBranch(nil))), "Select a run [feature]"))
}

// TestRunGetFlow_FooterShortcuts shows the footer advertises the refresh and
// branch-switch shortcuts (the active branch is named in the title, not here).
func TestRunGetFlow_FooterShortcuts(t *testing.T) {
	v := view(newToggleFlow(fetchByBranch(nil)))
	assert.Check(t, cmp.Contains(v, "r to refresh"))
	// The switch key is platform-specific (shift+tab, or Tab on Windows).
	assert.Check(t, cmp.Contains(v, switchScopeKeyLabel+" to switch branch"))
}

// TestRunGetFlow_ToggleCyclesScopes drives shift+tab through the full cycle:
// current branch → default branch → all branches → back to current, swapping the
// run list and title each step.
func TestRunGetFlow_ToggleCyclesScopes(t *testing.T) {
	m := newToggleFlow(fetchByBranch(map[string][]RunGetItem{
		"feature": {runItem("aaaaaaa [feature] - 1 minute ago")},
		"main":    {runItem("bbbbbbb [main] - 2 minutes ago")},
		"":        {runItem("ccccccc [other] - 3 minutes ago")},
	}))

	m = applyRunsFetch(t, m, switchKey) // feature → main
	assert.Equal(t, m.activeBranch, "main")
	assert.Check(t, cmp.Contains(view(m), "Select a run [main]"))
	assert.Check(t, cmp.Contains(view(m), "bbbbbbb [main]"))

	m = applyRunsFetch(t, m, switchKey) // main → all branches
	assert.Equal(t, m.activeBranch, "")
	assert.Check(t, cmp.Contains(view(m), "Select a run [all branches]"))
	assert.Check(t, cmp.Contains(view(m), "ccccccc [other]"))

	m = applyRunsFetch(t, m, switchKey) // all branches → feature (wrap)
	assert.Equal(t, m.activeBranch, "feature")
	assert.Check(t, cmp.Contains(view(m), "Select a run [feature]"))
}

// TestRunGetFlow_ToggleNoRuns keeps the current list and surfaces a footer note
// when the next scope has no runs.
func TestRunGetFlow_ToggleNoRuns(t *testing.T) {
	m := newToggleFlow(fetchByBranch(map[string][]RunGetItem{
		"feature": {runItem("aaaaaaa [feature] - 1 minute ago")},
		// "main" unmapped → empty result.
	}))

	m = applyRunsFetch(t, m, switchKey) // feature → main (empty)
	assert.Equal(t, m.activeBranch, "feature")
	v := view(m)
	assert.Check(t, cmp.Contains(v, "No runs found on main"))
	assert.Check(t, cmp.Contains(v, "aaaaaaa [feature]"))
}

// TestRunGetFlow_RefreshRefetchesActiveScope confirms r re-fetches the active
// branch and swaps in the fresh list without changing scope.
func TestRunGetFlow_RefreshRefetchesActiveScope(t *testing.T) {
	m := newToggleFlow(fetchByBranch(map[string][]RunGetItem{
		"feature": {runItem("zzzzzzz [feature] - just now")},
	}))

	m = applyRunsFetch(t, m, keyR)
	assert.Equal(t, m.activeBranch, "feature")
	assert.Check(t, cmp.Contains(view(m), "zzzzzzz [feature]"))
}

// TestRunGetFlow_SpinnerRespectsAnimate confirms the loading command honors the
// Animate flag: bare fetch when off (CIRCLE_SPINNER_DISABLED / non-interactive),
// batched with the spinner tick when on.
func TestRunGetFlow_SpinnerRespectsAnimate(t *testing.T) {
	opts := func(animate bool) RunGetFlowOptions {
		return RunGetFlowOptions{
			Runs:          []RunGetItem{runItem("aaaaaaa [feature] - now")},
			CurrentBranch: "feature",
			DefaultBranch: "main",
			FetchRuns:     fetchByBranch(nil),
			Animate:       animate,
		}
	}

	_, cmd := NewRunGetFlow(context.Background(), opts(false)).Update(keyR)
	assert.Assert(t, cmd != nil)
	_, isRuns := cmd().(runGetRunsMsg)
	assert.Check(t, isRuns, "animation off: loading cmd should be the bare fetch")

	_, cmd = NewRunGetFlow(context.Background(), opts(true)).Update(keyR)
	assert.Assert(t, cmd != nil)
	_, isBatch := cmd().(tea.BatchMsg)
	assert.Check(t, isBatch, "animation on: loading cmd should batch the spinner tick")
}

// stepSelectFlow builds a flow parked on the step picker of a single-execution
// job with two steps (the second failed), a known terminal size so the pager can
// build, and the given stdout/stderr fetchers.
func stepSelectFlow(stdout func(context.Context, uuid.UUID, int, int, int64) ([]byte, bool, error),
	stderr func(context.Context, uuid.UUID, int, int) ([]byte, error),
) RunGetFlowModel {
	m := NewRunGetFlow(context.Background(), RunGetFlowOptions{
		Runs:            []RunGetItem{runItem("aaaaaaa [main] - now")},
		FetchStepStdout: stdout,
		FetchStepStderr: stderr,
	})
	u, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = u.(RunGetFlowModel)

	m.jobID = uuid.New()
	m.executions = []RunGetExecution{{Index: 0}} // single execution → step picker carries the summary options
	m.steps = []RunGetStepItem{
		{Label: "checkout", Icon: "✓", Execution: 0, StepNum: 100},
		{Label: "run tests", Icon: "✗", Execution: 0, StepNum: 101},
	}
	m.stepCursor = -1
	m.stepSelect = m.newStepSelect()
	m.stage = runGetStageStepSelect
	return m
}

// runMsg executes a synchronous (non-tick) command and returns its message.
func runMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	assert.Assert(t, cmd != nil)
	return cmd()
}

// TestRunGetFlow_StepPagerStreams selects a step and drives the streaming pager:
// stdout arrives over two polled chunks then terminates, after which stderr is
// appended. It asserts the raw ANSI is preserved (colors), the footer reflects
// streaming vs. done, and every chunk lands in the buffer.
func TestRunGetFlow_StepPagerStreams(t *testing.T) {
	chunks := [][]byte{
		[]byte("\x1b[31mERROR\x1b[0m first line\n"),
		[]byte("second line\n"),
	}
	terminal := []bool{false, true}
	var n int
	stdout := func(_ context.Context, _ uuid.UUID, _, _ int, _ int64) ([]byte, bool, error) {
		i := n
		n++
		if i >= len(chunks) {
			return nil, true, nil
		}
		return chunks[i], terminal[i], nil
	}
	stderr := func(_ context.Context, _ uuid.UUID, _, _ int) ([]byte, error) {
		return []byte("stderr tail\n"), nil
	}

	m := stepSelectFlow(stdout, stderr)

	// The cursor defaults to the failed step ("run tests"); selecting it starts
	// the stream.
	u, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = u.(RunGetFlowModel)
	assert.Equal(t, m.stage, runGetStageLoadingStep)
	assert.Equal(t, m.stepNum, 101)

	// First stdout chunk → the pager opens, still streaming, and a poll is queued.
	u, cmd = m.Update(runMsg(t, cmd))
	m = u.(RunGetFlowModel)
	assert.Equal(t, m.stage, runGetStageStepPager)
	assert.Check(t, cmp.Contains(string(m.pagerBuf), "\x1b[31mERROR\x1b[0m"), "raw ANSI/colors must be preserved")
	assert.Check(t, cmp.Contains(view(m), "ERROR first line"))
	assert.Check(t, cmp.Contains(view(m), "streaming…"))
	assert.Assert(t, cmd != nil) // poll scheduled

	// Simulate the 2s poll firing (without waiting): it fetches the next chunk,
	// which terminates stdout and triggers the one-shot stderr fetch.
	u, cmd = m.Update(runGetStepPollMsg{epoch: m.pagerEpoch})
	m = u.(RunGetFlowModel)
	u, cmd = m.Update(runMsg(t, cmd)) // deliver second stdout chunk (terminal)
	m = u.(RunGetFlowModel)
	assert.Check(t, m.pagerTerminal)
	u, _ = m.Update(runMsg(t, cmd)) // deliver stderr
	m = u.(RunGetFlowModel)

	body := view(m)
	assert.Check(t, cmp.Contains(body, "ERROR first line"))
	assert.Check(t, cmp.Contains(body, "second line"))
	assert.Check(t, cmp.Contains(body, "stderr tail"))
	assert.Check(t, !strings.Contains(body, "streaming…"), "streaming indicator should clear once terminal")
}

// TestRunGetFlow_StepPagerEscResumes confirms esc from the pager returns to the
// step picker with the cursor restored to the step that was opened.
func TestRunGetFlow_StepPagerEscResumes(t *testing.T) {
	stdout := func(_ context.Context, _ uuid.UUID, _, _ int, _ int64) ([]byte, bool, error) {
		return []byte("output\n"), true, nil
	}
	stderr := func(_ context.Context, _ uuid.UUID, _, _ int) ([]byte, error) { return nil, nil }

	m := stepSelectFlow(stdout, stderr)
	picked := m.stepSelect.Selected() // failed step ("run tests")

	u, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = u.(RunGetFlowModel)
	u, _ = m.Update(runMsg(t, cmd)) // stdout arrives (terminal) → pager opens
	m = u.(RunGetFlowModel)
	assert.Equal(t, m.stage, runGetStageStepPager)

	// esc returns to the step picker, resuming on the opened step.
	u, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = u.(RunGetFlowModel)
	assert.Equal(t, m.stage, runGetStageStepSelect)
	assert.Equal(t, m.stepSelect.Selected(), picked)
}
