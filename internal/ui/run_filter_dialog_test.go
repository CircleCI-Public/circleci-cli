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
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

var (
	dlgLeft     = tea.KeyPressMsg{Code: tea.KeyLeft}
	dlgRight    = tea.KeyPressMsg{Code: tea.KeyRight}
	dlgDown     = tea.KeyPressMsg{Code: tea.KeyDown}
	dlgUp       = tea.KeyPressMsg{Code: tea.KeyUp}
	dlgTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	dlgShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	dlgSpace    = tea.KeyPressMsg{Code: ' ', Text: " "}
	dlgEnter    = tea.KeyPressMsg{Code: tea.KeyEnter}
	dlgEsc      = tea.KeyPressMsg{Code: tea.KeyEscape}
	dlgReset    = tea.KeyPressMsg{Code: 'r', Text: "r"}
)

// newTestDialog builds a dialog with the trigger scopes (current branch, all
// branches, your runs — each carrying its glyph) and two statuses, seeded on the
// current branch / all statuses, sized to a known terminal.
func newTestDialog() runFilterDialog {
	scopes := buildRunScopes("main", "", true, true)
	statuses := []RunStatusFilter{
		{Label: "all statuses", Icon: "★"},
		{Value: "failed", Label: "failed", Icon: "✗"},
	}
	return newRunFilterDialog(scopes, statuses, 0, 0, RunCreatedFilter{}, false).SetSize(80, 24)
}

// drive feeds a sequence of messages through the dialog and returns the result.
func drive(d runFilterDialog, msgs ...tea.Msg) runFilterDialog {
	for _, msg := range msgs {
		d, _ = d.Update(msg)
	}
	return d
}

func TestRunFilterDialog_OpensOnActiveSelection(t *testing.T) {
	d := newTestDialog()
	scope, status := d.Selected()
	assert.Check(t, cmp.Equal(d.Outcome(), runFilterOpen))
	assert.Check(t, cmp.Equal(scope, 0))
	assert.Check(t, cmp.Equal(status, 0))
}

func TestRunFilterDialog_ListNavigationSetsSelection(t *testing.T) {
	// Trigger tab: down selects the second scope.
	d := drive(newTestDialog(), dlgDown)
	scope, _ := d.Selected()
	assert.Check(t, cmp.Equal(scope, 1))

	// Right switches to the Status tab; down there selects the second status,
	// leaving the trigger selection untouched.
	d = drive(d, dlgRight, dlgDown)
	scope, status := d.Selected()
	assert.Check(t, cmp.Equal(scope, 1))
	assert.Check(t, cmp.Equal(status, 1))
}

func TestRunFilterDialog_TabSwitching(t *testing.T) {
	// right cycles Trigger → Status → Created → Trigger; left reverses.
	d := drive(newTestDialog(), dlgRight)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabStatus))
	d = drive(d, dlgRight)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabCreated))
	d = drive(d, dlgRight)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabTrigger))
	d = drive(d, dlgLeft)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabCreated))

	// tab advances, shift+tab reverses.
	d = drive(newTestDialog(), dlgTab)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabStatus))
	d = drive(d, dlgShiftTab)
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabTrigger))
}

// gotoCreated switches a freshly-opened dialog to the Created tab.
func gotoCreated(d runFilterDialog) runFilterDialog {
	return drive(d, dlgRight, dlgRight)
}

func TestRunFilterDialog_CreatedInactiveByDefault(t *testing.T) {
	// The date picker opens on "all dates", so the created filter is inactive and
	// adds no constraint.
	assert.Check(t, !newTestDialog().Created().Active())
}

func TestRunFilterDialog_CreatedSelectsAgeOlderByDefault(t *testing.T) {
	// On the Created tab the date list opens on "all dates"; moving down to an age
	// (the cursor is the selection) activates the filter, older by default.
	d := gotoCreated(newTestDialog())
	d = drive(d, dlgDown) // all dates → 1 Hour
	created := d.Created()
	assert.Check(t, created.Active())
	assert.Check(t, cmp.Equal(created.Duration, time.Hour))
	assert.Check(t, cmp.Equal(created.Label, "1 Hour"))
	assert.Check(t, !created.Newer, "direction should default to older")
}

func TestRunFilterDialog_CreatedSpaceTogglesDirection(t *testing.T) {
	// Space toggles older ↔ newer for the selected age.
	d := gotoCreated(newTestDialog())
	d = drive(d, dlgDown) // select "1 Hour" (older by default)
	assert.Assert(t, !d.Created().Newer)
	d = drive(d, dlgSpace)
	assert.Check(t, d.Created().Newer, "space should switch to newer")
	d = drive(d, dlgSpace)
	assert.Check(t, !d.Created().Newer, "space again should switch back to older")
}

func TestRunFilterDialog_CreatedAllDatesClearsFilter(t *testing.T) {
	// Moving back to the "all dates" entry clears the created filter.
	d := gotoCreated(newTestDialog())
	d = drive(d, dlgDown, dlgDown) // 6 Hours
	assert.Assert(t, d.Created().Active())
	d = drive(d, dlgUp, dlgUp) // back to "all dates"
	assert.Check(t, !d.Created().Active())
}

func TestRunFilterDialog_ResetClearsCreated(t *testing.T) {
	// r resets every tab: the date list back to "all dates" and direction to older.
	d := gotoCreated(newTestDialog())
	d = drive(d, dlgDown, dlgSpace) // 1 Hour, newer
	assert.Assert(t, d.Created().Active())
	assert.Assert(t, d.Created().Newer)
	d = drive(d, dlgReset)
	assert.Check(t, !d.Created().Active())
	assert.Check(t, !d.createdNewer, "reset returns direction to older")
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabTrigger), "reset returns to the Trigger tab")
}

func TestRunFilterDialog_CreatedViewShowsPickerAndDirection(t *testing.T) {
	// The Created tab shows the direction toggle, the date options (including the
	// "all dates" clear entry) and its help.
	v := ansi.Strip(gotoCreated(newTestDialog()).View().Content)
	for _, want := range []string{"older", "newer", "all dates", "1 Hour", "24 Hours", "1 Month", "Filter runs by age"} {
		assert.Check(t, cmp.Contains(v, want), "created view missing %q", want)
	}
}

func TestRunFilterDialog_CreatedSeededFromActiveFilter(t *testing.T) {
	// Opening with an active created filter selects its age and direction so the
	// dialog reflects what the picker is already showing.
	scopes := buildRunScopes("main", "", true, true)
	statuses := []RunStatusFilter{{Label: "all statuses", Icon: "★"}}
	created := RunCreatedFilter{Newer: true, Duration: 24 * time.Hour, Label: "24 Hours"}
	d := newRunFilterDialog(scopes, statuses, 0, 0, created, false).SetSize(80, 24)
	assert.Check(t, cmp.Equal(d.Created(), created))
}

func TestRunFilterDialog_EnterApplies(t *testing.T) {
	d := drive(newTestDialog(), dlgDown, dlgEnter)
	assert.Check(t, cmp.Equal(d.Outcome(), runFilterApply))
	scope, _ := d.Selected()
	assert.Check(t, cmp.Equal(scope, 1))
}

func TestRunFilterDialog_EscCancels(t *testing.T) {
	d := drive(newTestDialog(), dlgEsc)
	assert.Check(t, cmp.Equal(d.Outcome(), runFilterCancel))
}

func TestRunFilterDialog_ResetRestoresDefaults(t *testing.T) {
	// Move both selections off their defaults, then reset with "r".
	d := drive(newTestDialog(), dlgDown, dlgRight, dlgDown)
	scope, status := d.Selected()
	assert.Assert(t, cmp.Equal(scope, 1))
	assert.Assert(t, cmp.Equal(status, 1))

	d = drive(d, dlgReset)
	scope, status = d.Selected()
	assert.Check(t, cmp.Equal(d.Outcome(), runFilterOpen), "reset should not close the dialog")
	assert.Check(t, cmp.Equal(scope, 0))
	assert.Check(t, cmp.Equal(status, 0))
	assert.Check(t, cmp.Equal(d.activeTab(), filterTabTrigger), "reset returns to the Trigger tab")
}

func TestRunFilterDialog_ViewShowsTabsAndOptions(t *testing.T) {
	// The Trigger tab is active on open: both tabs, its trigger options (the
	// current branch role with its branch name nested beneath as "[main]", and a
	// heart glyph on "my runs") and its help description show.
	v := ansi.Strip(newTestDialog().View().Content)
	for _, want := range []string{"Trigger", "Status", "current branch", "[main]", "all branches", "♥ my runs", "Filter runs by trigger"} {
		assert.Check(t, cmp.Contains(v, want), "trigger view missing %q", want)
	}

	// Switching to the Status tab shows the status options (with icons: a star for
	// the special "all statuses" entry) and the status help description.
	s := ansi.Strip(drive(newTestDialog(), dlgRight).View().Content)
	assert.Check(t, cmp.Contains(s, "★ all statuses"))
	assert.Check(t, cmp.Contains(s, "✗ failed"))
	assert.Check(t, cmp.Contains(s, "Filter runs by status"))
}
