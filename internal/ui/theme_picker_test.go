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
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// echoRender returns a render callback that echoes the theme and width it was
// asked for, so tests can assert the preview tracks the highlighted option.
func echoRender() func(string, int) string {
	return func(theme string, width int) string {
		return fmt.Sprintf("rendered:%s@%d", theme, width)
	}
}

// newTestPicker builds a three-theme picker with animation on (the interactive
// default: a cache-miss preview renders asynchronously behind a placeholder).
func newTestPicker(t *testing.T) ui.ThemePickerModel {
	t.Helper()
	labels := []string{"auto (default)", "dark", "light"}
	themes := []string{"auto", "dark", "light"}
	return ui.NewThemePickerModel("Select a theme", labels, themes, echoRender(), false, true).WithCursor(0)
}

// newStaticPicker is newTestPicker with animation off, so previews render
// synchronously into every frame. teatest drives it through its alt-screen
// program loop; a synchronous preview means each frame's model already holds the
// rendered content, so assertions read the final model's View rather than racing
// the async render through the output stream.
func newStaticPicker(t *testing.T) ui.ThemePickerModel {
	t.Helper()
	labels := []string{"auto (default)", "dark", "light"}
	themes := []string{"auto", "dark", "light"}
	return ui.NewThemePickerModel("Select a theme", labels, themes, echoRender(), false, false).WithCursor(0)
}

// updatePicker applies msg and runs any returned command to completion, feeding
// each resulting message back in. The preview is rendered off the Update loop
// via a command, so synchronous tests must drain it to observe the content.
// SelectModel emits no commands, so the chain is at most one render deep.
func updatePicker(m ui.ThemePickerModel, msg tea.Msg) ui.ThemePickerModel {
	model, cmd := m.Update(msg)
	m = model.(ui.ThemePickerModel)
	for cmd != nil {
		next := cmd()
		if next == nil {
			break
		}
		model, cmd = m.Update(next)
		m = model.(ui.ThemePickerModel)
	}
	return m
}

// themeHarness drives a ThemePickerModel as a teatest program and quits on
// quitMsg without disturbing the inner model, so its live View (with the preview
// pane) can be snapshotted before the program's own quit paths run.
type themeHarness struct {
	m ui.ThemePickerModel
}

func (h themeHarness) Init() tea.Cmd { return h.m.Init() }

func (h themeHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(quitMsg); ok {
		return h, tea.Quit
	}
	u, cmd := h.m.Update(msg)
	h.m = u.(ui.ThemePickerModel)
	return h, cmd
}

func (h themeHarness) View() tea.View { return h.m.View() }

// startPicker runs a picker at a known terminal size and waits for the selector
// pane's first frame (so the initial window size has been laid out).
func startPicker(t *testing.T, m ui.ThemePickerModel) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, themeHarness{m: m}, teatest.WithInitialTermSize(100, 24))
	waitForOutput(t, tm, "Select a theme")
	return tm
}

// themeFinal returns the inner model after the program ends — reached either by
// the picker's own quit (enter/esc) or by a prior quitMsg. State and View are
// read from it, sidestepping the alt-screen output stream.
func themeFinal(t *testing.T, tm *teatest.TestModel) ui.ThemePickerModel {
	t.Helper()
	return tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(themeHarness).m
}

// TestThemePickerNotReady confirms the view is empty before a window size
// arrives (so the program prints nothing until it can lay out the split). This
// is the pre-program state, asserted directly on the exported View.
func TestThemePickerNotReady(t *testing.T) {
	assert.Check(t, cmp.Equal(newTestPicker(t).View().Content, ""))
}

// TestThemePickerRendersPreview confirms a window size lays out both panes: the
// selector list and the preview rendered for the initially-highlighted theme.
func TestThemePickerRendersPreview(t *testing.T) {
	tm := startPicker(t, newStaticPicker(t))
	tm.Send(quitMsg{})

	view := ansi.Strip(themeFinal(t, tm).View().Content)
	assert.Check(t, cmp.Contains(view, "Select a theme"), "selector pane missing")
	assert.Check(t, cmp.Contains(view, "rendered:auto@"), "preview for first theme missing")
}

// TestThemePickerShowsLoadingPlaceholder confirms that before the async render
// lands the preview shows the loading placeholder, not blank space. The command
// is deliberately left undrained — a transient state the program loop would race
// past — so this drives Update directly.
func TestThemePickerShowsLoadingPlaceholder(t *testing.T) {
	m := newTestPicker(t)
	model, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = model.(ui.ThemePickerModel)
	assert.Check(t, cmd != nil, "a cache miss should return a render command")

	view := m.View().Content
	assert.Check(t, strings.Contains(view, "Loading"), "loading placeholder missing")
	assert.Check(t, !strings.Contains(view, "rendered:auto@"), "preview should not be rendered yet")
}

// TestThemePickerSpinnerAnimationGating confirms the spinner ticks only when
// animation is enabled; disabled (CIRCLE_SPINNER_DISABLED / non-TTY) it stays
// still and renders synchronously so scripted sessions don't see repaints. This
// inspects Init/command behavior, so it drives the model directly.
func TestThemePickerSpinnerAnimationGating(t *testing.T) {
	labels := []string{"auto"}
	themes := []string{"auto"}
	render := func(string, int) string { return "x" }

	assert.Assert(t, t.Run("animation enabled starts the spinner ticking", func(t *testing.T) {
		animated := ui.NewThemePickerModel("t", labels, themes, render, false, true)
		assert.Check(t, animated.Init() != nil, "animated picker should start ticking")
	}))
	assert.Assert(t, t.Run("animation disabled stays still and renders synchronously", func(t *testing.T) {
		static := ui.NewThemePickerModel("t", labels, themes, render, false, false)
		assert.Check(t, static.Init() == nil, "static picker should not tick")

		model, cmd := static.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
		static = model.(ui.ThemePickerModel)
		assert.Check(t, cmd == nil, "static picker should render synchronously, not via a command")
		view := static.View().Content
		assert.Check(t, strings.Contains(view, "x"), "preview content should be rendered")
		assert.Check(t, !strings.Contains(view, "Loading"), "no placeholder when synchronous")
	}))
}

// TestThemePickerPreviewFollowsCursor confirms moving the cursor re-renders the
// preview for the newly-highlighted theme and advances the selection.
func TestThemePickerPreviewFollowsCursor(t *testing.T) {
	tm := startPicker(t, newStaticPicker(t))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(quitMsg{})

	m := themeFinal(t, tm)
	assert.Check(t, cmp.Contains(ansi.Strip(m.View().Content), "rendered:dark@"), "preview did not follow the cursor")
	assert.Check(t, cmp.Equal(m.Selected(), 1))
}

// TestThemePickerEnterSelects confirms Enter confirms the highlighted theme
// without cancelling.
func TestThemePickerEnterSelects(t *testing.T) {
	tm := startPicker(t, newStaticPicker(t))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	m := themeFinal(t, tm)
	assert.Check(t, cmp.Equal(m.Cancelled(), false))
	assert.Check(t, cmp.Equal(m.Selected(), 1))
}

// TestThemePickerCachesRenders confirms revisiting a theme is served from cache:
// render runs once per theme, not on every pass, which is what keeps navigation
// from blocking on glamour. It drains commands synchronously to count renders
// deterministically.
func TestThemePickerCachesRenders(t *testing.T) {
	labels := []string{"auto", "dark", "light"}
	themes := []string{"auto", "dark", "light"}
	calls := map[string]int{}
	render := func(theme string, _ int) string {
		calls[theme]++
		return "rendered:" + theme
	}
	m := ui.NewThemePickerModel("Select a theme", labels, themes, render, false, true).WithCursor(0)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})

	// Walk down to the bottom and back up to the top, crossing each theme twice.
	for range 2 {
		m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for range 2 {
		m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyUp})
	}

	for theme, n := range calls {
		assert.Check(t, cmp.Equal(n, 1), "theme %q rendered %d times, want 1", theme, n)
	}
	assert.Check(t, cmp.Equal(len(calls), 3), "every theme should have been rendered once")
}

// TestThemePickerEscCancels confirms Esc quits with the cancelled flag set.
func TestThemePickerEscCancels(t *testing.T) {
	tm := startPicker(t, newStaticPicker(t))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Check(t, cmp.Equal(themeFinal(t, tm).Cancelled(), true))
}
