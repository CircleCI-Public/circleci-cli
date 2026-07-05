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

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

var (
	dlgLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	dlgRight = tea.KeyPressMsg{Code: tea.KeyRight}
	dlgDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	dlgTab   = tea.KeyPressMsg{Code: tea.KeyTab}
	dlgEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	dlgEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	dlgReset = tea.KeyPressMsg{Code: 'r', Text: "r"}
)

// newTestDialog builds a dialog with two branch scopes and two statuses, seeded
// on the current branch / all statuses, sized to a known terminal.
func newTestDialog() runFilterDialog {
	scopes := []runScope{
		{branch: "main", label: "main branch"},
		{label: "all branches"},
	}
	statuses := []RunStatusFilter{
		{Label: "all statuses", Icon: "★"},
		{Value: "failed", Label: "failed", Icon: "✗"},
	}
	return newRunFilterDialog(scopes, statuses, 0, 0, false).SetSize(80, 24)
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
	// Branch tab: down selects the second scope.
	d := drive(newTestDialog(), dlgDown)
	scope, _ := d.Selected()
	assert.Check(t, cmp.Equal(scope, 1))

	// Right switches to the Status tab; down there selects the second status,
	// leaving the branch selection untouched.
	d = drive(d, dlgRight, dlgDown)
	scope, status := d.Selected()
	assert.Check(t, cmp.Equal(scope, 1))
	assert.Check(t, cmp.Equal(status, 1))
}

func TestRunFilterDialog_TabSwitching(t *testing.T) {
	// left selects the Branch tab, right the Status tab, regardless of the current
	// tab; tab toggles between them.
	d := drive(newTestDialog(), dlgRight)
	assert.Check(t, cmp.Equal(d.tab, filterTabStatus))
	d = drive(d, dlgLeft)
	assert.Check(t, cmp.Equal(d.tab, filterTabBranch))
	d = drive(d, dlgTab)
	assert.Check(t, cmp.Equal(d.tab, filterTabStatus))
	d = drive(d, dlgTab)
	assert.Check(t, cmp.Equal(d.tab, filterTabBranch))
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
	assert.Check(t, cmp.Equal(d.tab, filterTabBranch), "reset returns to the Branch tab")
}

func TestRunFilterDialog_ViewShowsTabsAndOptions(t *testing.T) {
	// The Branch tab is active on open: both tabs, its branch options and its help
	// description show.
	v := ansi.Strip(newTestDialog().View().Content)
	for _, want := range []string{"Branch", "Status", "main", "all branches", "Filter runs by branch"} {
		assert.Check(t, cmp.Contains(v, want), "branch view missing %q", want)
	}

	// Switching to the Status tab shows the status options (with icons: a star for
	// the special "all statuses" entry) and the status help description.
	s := ansi.Strip(drive(newTestDialog(), dlgRight).View().Content)
	assert.Check(t, cmp.Contains(s, "★ all statuses"))
	assert.Check(t, cmp.Contains(s, "✗ failed"))
	assert.Check(t, cmp.Contains(s, "Filter runs by status"))
}
