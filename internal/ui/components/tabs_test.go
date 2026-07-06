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

package components_test

import (
	"bytes"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
)

// tabsHarness drives a Tabs bar as a standalone teatest program, quitting on
// ctrl+c. Tabs never quits itself (its host owns enter/esc), so the harness does.
// The wrapped Tabs stays reachable via t for post-run assertions.
type tabsHarness struct {
	t components.Tabs
}

func (h tabsHarness) Init() tea.Cmd { return nil }

func (h tabsHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeyCtrlC) {
		return h, tea.Quit
	}
	updated, _ := h.t.Update(msg)
	h.t = updated
	return h, nil
}

func (h tabsHarness) View() tea.View { return tea.NewView(h.t.View("body content")) }

func startTabs(t *testing.T, tabs components.Tabs) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, tabsHarness{t: tabs}, teatest.WithInitialTermSize(60, 12))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("body content"))
	}, teatest.WithDuration(time.Second))
	return tm
}

func finalTabs(t *testing.T, tm *teatest.TestModel) components.Tabs {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(tabsHarness).t
}

var (
	tabRight    = tea.KeyPressMsg{Code: tea.KeyRight}
	tabLeft     = tea.KeyPressMsg{Code: tea.KeyLeft}
	tabTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	tabShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
)

func TestTabs_StartsOnFirst(t *testing.T) {
	tm := startTabs(t, components.NewTabs([]string{"A", "B", "C"}, false))
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 0))
}

func TestTabs_RightAndTabAdvance(t *testing.T) {
	tm := startTabs(t, components.NewTabs([]string{"A", "B", "C"}, false))
	tm.Send(tabRight)
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 1))

	tm = startTabs(t, components.NewTabs([]string{"A", "B", "C"}, false))
	tm.Send(tabTab)
	tm.Send(tabTab)
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 2))
}

func TestTabs_LeftAndShiftTabReverse(t *testing.T) {
	tm := startTabs(t, components.NewTabs([]string{"A", "B", "C"}, false))
	tm.Send(tabRight)
	tm.Send(tabLeft)
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 0))

	tm = startTabs(t, components.NewTabs([]string{"A", "B", "C"}, false))
	tm.Send(tabShiftTab)
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 2), "shift+tab from the first wraps to the last")
}

func TestTabs_Wraps(t *testing.T) {
	tm := startTabs(t, components.NewTabs([]string{"A", "B"}, false))
	tm.Send(tabRight)
	tm.Send(tabRight)
	assert.Check(t, cmp.Equal(finalTabs(t, tm).Active(), 0), "advancing past the last wraps to the first")
}

func TestTabs_UpdateReportsHandled(t *testing.T) {
	tabs := components.NewTabs([]string{"A", "B"}, false)
	_, handled := tabs.Update(tabRight)
	assert.Check(t, handled, "a tab-switch key is consumed")
	_, handled = tabs.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Check(t, !handled, "a non-switch key is left for the host")
}

func TestTabs_ViewRendersLabelsAndBody(t *testing.T) {
	tm := startTabs(t, components.NewTabs([]string{"Alpha", "Beta"}, false))
	// A wide body so each tab cell is roomy enough to hold its label on one line.
	body := "a reasonably wide body so the tab cells are not squished"
	v := ansi.Strip(finalTabs(t, tm).View(body))
	for _, want := range []string{"Alpha", "Beta", body} {
		assert.Check(t, cmp.Contains(v, want), "view missing %q", want)
	}
}
