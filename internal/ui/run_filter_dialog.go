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
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// Tab borders, styled after the lipgloss "layout" example: an active tab opens at
// the bottom (a blank Bottom edge) so it reads as continuous with the window body
// below it, while an inactive tab is a closed box whose bottom line forms part of
// the window's top border. The first/last tab's bottom outer corner is patched at
// render time so the tab row seams into the window's side borders (see renderTabs).
var (
	filterActiveTabBorder = lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	filterTabBorder = lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}
)

// runFilterOutcome is the state of a runFilterDialog after an Update: still open,
// applied (the user confirmed a branch + status selection), or cancelled.
type runFilterOutcome int

const (
	runFilterOpen runFilterOutcome = iota
	runFilterApply
	runFilterCancel
)

// runFilterTab is the active facet in the dialog: the branch (scope) list or the
// status-filter list.
type runFilterTab int

const (
	filterTabBranch runFilterTab = iota
	filterTabStatus
)

// runFilterZone is where keyboard focus sits: in the active tab's list, or on the
// OK / Cancel / Reset button row.
type runFilterZone int

const (
	filterZoneList runFilterZone = iota
	filterZoneButtons
)

// runFilterButton identifies a dialog button.
type runFilterButton int

const (
	filterBtnOK runFilterButton = iota
	filterBtnCancel
	filterBtnReset
)

// filterBtnCount is the number of buttons. It is kept out of the runFilterButton
// enum (an untyped constant) so switches over the button values stay exhaustive.
const filterBtnCount = 3

// dialog keys. tab/shift+tab move focus between the list and the button row;
// left/right switch the active tab (in the list) or move between buttons (on the
// button row).
var (
	filterKeyLeft  = key.NewBinding(key.WithKeys("left", "h"))
	filterKeyRight = key.NewBinding(key.WithKeys("right", "l"))

	// Button mnemonics: accelerators that fire OK / Cancel / Reset from anywhere
	// in the dialog, matching the underlined letter shown on each button.
	filterKeyOK     = key.NewBinding(key.WithKeys("o", "O"))
	filterKeyCancel = key.NewBinding(key.WithKeys("c", "C"))
	filterKeyReset  = key.NewBinding(key.WithKeys("r", "R"))
)

// runFilterDialog is the "/" search overlay on the run picker: a two-tab
// (Branch / Status) chooser that lets the user set the branch scope and status
// filter explicitly from lists, with OK / Cancel / Reset buttons. It embeds two
// components.SelectModel lists (one per tab) so the picker's navigation, scrolling
// and rendering are reused; the dialog only adds the tab bar, the button row and
// the focus model on top. It never quits the program — the host reads Outcome()
// after each Update and acts on Apply/Cancel.
type runFilterDialog struct {
	scopes   []runScope
	statuses []RunStatusFilter

	branchSel components.SelectModel
	statusSel components.SelectModel

	tab    runFilterTab
	zone   runFilterZone
	button runFilterButton

	// defaultScope / defaultStatus are the indexes the Reset button restores
	// (the current branch and "all statuses" — always the first of each cycle).
	defaultScope  int
	defaultStatus int

	color         bool
	width, height int
	listHeight    int // rows given to each embedded list (see SetSize)
	outcome       runFilterOutcome
}

// newRunFilterDialog builds the dialog seeded with the currently active scope and
// status so it opens on what the picker is already showing. scopeIdx/statusIdx are
// the active selections; the defaults it resets to are the first of each cycle.
func newRunFilterDialog(scopes []runScope, statuses []RunStatusFilter, scopeIdx, statusIdx int, color bool) runFilterDialog {
	d := runFilterDialog{
		scopes:   scopes,
		statuses: statuses,
		tab:      filterTabBranch,
		zone:     filterZoneList,
		button:   filterBtnOK,
		color:    color,
	}
	d.branchSel = d.newBranchSelect(scopeIdx)
	d.statusSel = d.newStatusSelect(statusIdx)
	return d
}

// newBranchSelect builds the chrome-free branch list (no title, no footer) seeded
// at idx.
func (d runFilterDialog) newBranchSelect(idx int) components.SelectModel {
	return components.NewSelectModel("", scopeLabels(d.scopes)).
		WithCursor(idx).WithKeys().WithHeight(d.listHeight)
}

// newStatusSelect builds the chrome-free status list seeded at idx. Each status
// carries its glyph (a star for the "all statuses" no-filter entry, the picker's
// status symbols otherwise), and the no-filter entry's label is italicised (and
// tinted when color is on) so it reads as the special "clear the filter" option
// rather than a real status.
func (d runFilterDialog) newStatusSelect(idx int) components.SelectModel {
	statuses, color := d.statuses, d.color
	return components.NewSelectModel("", statusLabels(statuses)).
		WithCursor(idx).WithKeys().WithHeight(d.listHeight).
		WithIcons(d.statusIcons()).
		WithItemStyleFunc(func(i int) lipgloss.Style {
			if i >= 0 && i < len(statuses) && statuses[i].Value == "" {
				st := lipgloss.NewStyle().Italic(true)
				if color {
					st = st.Foreground(theme.ColorSecondary)
				}
				return st
			}
			return lipgloss.NewStyle()
		})
}

// statusIcons renders each status's glyph for the list: the star (tinted
// secondary) for the "all statuses" no-filter entry, the picker's colored status
// symbol otherwise.
func (d runFilterDialog) statusIcons() []string {
	icons := make([]string, len(d.statuses))
	for i, s := range d.statuses {
		if s.Value == "" { // the "all statuses" star
			star := s.Icon
			if star == "" {
				star = "★"
			}
			if d.color {
				star = theme.SecondaryStyle.Render(star)
			}
			icons[i] = star
			continue
		}
		icons[i] = colorizeStatusIcon(s.Icon, d.color)
	}
	return icons
}

func scopeLabels(scopes []runScope) []string {
	labels := make([]string, len(scopes))
	for i, s := range scopes {
		labels[i] = s.titleName()
	}
	return labels
}

func statusLabels(statuses []RunStatusFilter) []string {
	labels := make([]string, len(statuses))
	for i, s := range statuses {
		labels[i] = s.Label
	}
	return labels
}

// SetSize records the terminal size and lays out the embedded lists. The lists
// are sized to hold every option (they are short: a handful of branches and
// statuses), so switching tabs never changes the dialog's height; a very short
// terminal clamps them instead.
func (d runFilterDialog) SetSize(width, height int) runFilterDialog {
	d.width, d.height = width, height
	// +1: the embedded select reserves a row for its (empty) footer hint, so it
	// needs one extra row to show every option without a spurious scroll bar.
	listHeight := d.bodyRows() + 1
	if height > 0 && height-9 < listHeight {
		listHeight = height - 9
	}
	if listHeight < 2 {
		listHeight = 2
	}
	d.listHeight = listHeight
	d.branchSel = d.branchSel.WithHeight(listHeight)
	d.statusSel = d.statusSel.WithHeight(listHeight)
	return d
}

// Outcome reports whether the dialog is still open, or the user applied or
// cancelled it. The host checks this after each Update.
func (d runFilterDialog) Outcome() runFilterOutcome { return d.outcome }

// Selected returns the chosen scope and status indexes. Valid once Outcome() is
// runFilterApply.
func (d runFilterDialog) Selected() (scopeIdx, statusIdx int) {
	return d.branchSel.Selected(), d.statusSel.Selected()
}

func (d runFilterDialog) Init() tea.Cmd { return nil }

// Update handles the dialog's keys. It never emits a command; navigation is
// forwarded to the active tab's embedded list. ctrl+c is left to the host (it
// quits the whole program), so the dialog only binds esc (cancel), tab/shift+tab
// (switch focus zone), left/right (switch tab or move button) and enter (apply,
// or activate the focused button).
func (d runFilterDialog) Update(msg tea.Msg) (runFilterDialog, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		d = d.SetSize(ws.Width, ws.Height)
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}

	switch {
	case key.Matches(k, components.KeyEsc):
		d.outcome = runFilterCancel
		return d, nil
	case key.Matches(k, filterKeyOK):
		d.outcome = runFilterApply
		return d, nil
	case key.Matches(k, filterKeyCancel):
		d.outcome = runFilterCancel
		return d, nil
	case key.Matches(k, filterKeyReset):
		d.reset()
		return d, nil
	case key.Matches(k, components.KeyTab, components.KeyShiftTab):
		d.toggleZone()
		return d, nil
	case key.Matches(k, filterKeyLeft):
		d.moveLeft()
		return d, nil
	case key.Matches(k, filterKeyRight):
		d.moveRight()
		return d, nil
	case key.Matches(k, components.KeyEnter):
		return d.activate()
	}

	// Any other key (up/down, paging, g/G) drives the active tab's list, but only
	// while the list zone has focus.
	if d.zone == filterZoneList {
		d.forwardToList(msg)
	}
	return d, nil
}

// toggleZone flips focus between the list and the button row.
func (d *runFilterDialog) toggleZone() {
	if d.zone == filterZoneList {
		d.zone = filterZoneButtons
	} else {
		d.zone = filterZoneList
	}
}

// moveLeft switches to the previous tab (list zone) or the previous button
// (button zone), clamping at the ends.
func (d *runFilterDialog) moveLeft() {
	if d.zone == filterZoneList {
		if d.tab == filterTabStatus {
			d.tab = filterTabBranch
		}
		return
	}
	if d.button > 0 {
		d.button--
	}
}

// moveRight is moveLeft's mirror: next tab or next button.
func (d *runFilterDialog) moveRight() {
	if d.zone == filterZoneList {
		if d.tab == filterTabBranch {
			d.tab = filterTabStatus
		}
		return
	}
	if d.button < filterBtnCount-1 {
		d.button++
	}
}

// activate handles enter: in the list zone it applies the current selection (a
// convenience shortcut for OK); on the button row it runs the focused button.
func (d runFilterDialog) activate() (runFilterDialog, tea.Cmd) {
	if d.zone == filterZoneList {
		d.outcome = runFilterApply
		return d, nil
	}
	switch d.button {
	case filterBtnOK:
		d.outcome = runFilterApply
	case filterBtnCancel:
		d.outcome = runFilterCancel
	case filterBtnReset:
		d.reset()
	}
	return d, nil
}

// reset restores the branch and status selections to their defaults (the current
// branch, all statuses) and returns focus to the Branch list.
func (d *runFilterDialog) reset() {
	d.branchSel = d.newBranchSelect(d.defaultScope)
	d.statusSel = d.newStatusSelect(d.defaultStatus)
	d.tab = filterTabBranch
	d.zone = filterZoneList
	d.button = filterBtnOK
}

// forwardToList sends a message to whichever tab's list is active, keeping the
// embedded SelectModel's navigation and scrolling behaviour.
func (d *runFilterDialog) forwardToList(msg tea.Msg) {
	if d.tab == filterTabBranch {
		updated, _ := d.branchSel.Update(msg)
		d.branchSel = updated.(components.SelectModel)
	} else {
		updated, _ := d.statusSel.Update(msg)
		d.statusSel = updated.(components.SelectModel)
	}
}

func (d runFilterDialog) View() tea.View {
	dialog := d.renderDialog()
	footer := components.Hints(
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "list/buttons")),
		key.NewBinding(key.WithKeys("←/→"), key.WithHelp("←/→", d.leftRightHelp())),
		components.BindMove,
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	)

	// Centre the dialog in the screen above the footer, and pin the help text to
	// the very bottom row. The dialog fills the screen, so it renders on the
	// alternate screen (like the help overlay and pager) — otherwise bubbletea's
	// inline renderer scrolls and corrupts the frame as the content repaints.
	if d.width > 0 && d.height > 1 {
		top := lipgloss.Place(d.width, d.height-1, lipgloss.Center, lipgloss.Center, dialog)
		foot := lipgloss.Place(d.width, 1, lipgloss.Center, lipgloss.Bottom, footer)
		v := tea.NewView(top + "\n" + foot)
		v.AltScreen = true
		return v
	}
	v := tea.NewView(dialog + "\n" + footer)
	v.AltScreen = true
	return v
}

// leftRightHelp names what ←/→ does in the current zone, so the footer stays
// truthful as focus moves.
func (d runFilterDialog) leftRightHelp() string {
	if d.zone == filterZoneButtons {
		return "button"
	}
	return "tab"
}

// filterHelpWidth is the column width of the right-hand help panel in each tab.
const filterHelpWidth = 34

// renderDialog assembles the tabbed window: a row of tabs whose bottom edge forms
// the window's top border, over a rounded-bordered body holding the tab body
// (options on the left, a help description on the right) and the button row. The
// window is sized to the content, and the tab row is split to the same width so
// the two seam together.
func (d runFilterDialog) renderDialog() string {
	body := d.tabBody()
	// contentWidth is the window's interior width. It must hold both the two-column
	// body and the button row (whose Reset is right-aligned to this width).
	contentWidth := max(lipgloss.Width(body), lipgloss.Width(d.renderButtons(0)), 44)
	// rowWidth is the dialog's outer width. lipgloss .Width() here sets the outer
	// box width (border + padding included), so the window is contentWidth +
	// border(2) + padding(2); the tab row is set to the same width so they line up.
	rowWidth := contentWidth + 4
	if rowWidth%2 != 0 {
		rowWidth++ // even split across the two tabs
		contentWidth++
	}

	window := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false). // the tab row supplies the top edge
		Padding(0, 1).
		Width(rowWidth)
	if d.color {
		window = window.BorderForeground(theme.ColorSecondary)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, body, "", d.renderButtons(contentWidth))
	return lipgloss.JoinVertical(lipgloss.Left, d.renderTabs(rowWidth), window.Render(inner))
}

// tabBody lays out the active tab as two columns: the options list on the left, a
// vertical divider, and the tab's help description on the right. Both columns are
// sized to a fixed width and height (the same for either tab) so switching tabs
// never resizes the dialog.
func (d runFilterDialog) tabBody() string {
	h := d.panelHeight()
	left := lipgloss.NewStyle().Width(d.listColumnWidth()).Height(h).
		Render(strings.TrimRight(d.activeList(), "\n"))

	// The divider is muted so it recedes; the description is left in the terminal's
	// default foreground so it reads stronger than the muted divider and footer
	// hints around it.
	divStyle := lipgloss.NewStyle()
	helpStyle := lipgloss.NewStyle().Width(filterHelpWidth).Height(h)
	if d.color {
		divStyle = divStyle.Foreground(theme.ColorMuted)
	}
	div := divStyle.Height(h).Render(strings.TrimRight(strings.Repeat("│\n", h), "\n"))
	help := helpStyle.Render(d.tabHelp())

	return lipgloss.JoinHorizontal(lipgloss.Top,
		left,
		lipgloss.NewStyle().Padding(0, 1).Render(div),
		help,
	)
}

// panelHeight is the fixed row count of the tab body: the taller of the option
// lists and the two help descriptions (wrapped to filterHelpWidth), so both tabs
// render at the same height regardless of which has more options or longer help.
func (d runFilterDialog) panelHeight() int {
	h := max(len(d.scopes), len(d.statuses))
	for _, help := range []string{branchHelpText, statusHelpText} {
		wrapped := lipgloss.NewStyle().Width(filterHelpWidth).Render(help)
		h = max(h, lipgloss.Height(wrapped))
	}
	return h
}

// listColumnWidth is the width of the options column, the wider of the two lists
// so the column (and thus the dialog) stays the same width on either tab.
func (d runFilterDialog) listColumnWidth() int {
	return max(
		lipgloss.Width(strings.TrimRight(d.branchSel.View().Content, "\n")),
		lipgloss.Width(strings.TrimRight(d.statusSel.View().Content, "\n")),
	)
}

// activeList renders the embedded list for the active tab.
func (d runFilterDialog) activeList() string {
	if d.tab == filterTabBranch {
		return d.branchSel.View().Content
	}
	return d.statusSel.View().Content
}

// tabHelp is the description shown on the right of the active tab.
func (d runFilterDialog) tabHelp() string {
	if d.tab == filterTabBranch {
		return branchHelpText
	}
	return statusHelpText
}

const (
	branchHelpText = "Filter runs by branch.\n\n" +
		"Pick a branch to list only its runs. \"all branches\" shows every branch; " +
		"\"your runs\" lists your runs across all projects."
	statusHelpText = "Filter runs by status.\n\n" +
		"\"all statuses\" clears the filter. Pick a status to show only the runs in " +
		"that state."
)

// renderTabs draws the two-tab row at rowWidth, split evenly. The active tab uses
// the open-bottom border so it reads as continuous with the window and its label
// is pink; the outer bottom corners of the first and last tab are patched so the
// row seams into the window's side borders below.
func (d runFilterDialog) renderTabs(rowWidth int) string {
	widths := splitWidth(rowWidth, 2)
	labels := [2]string{"Branch", "Status"}
	active := [2]bool{d.tab == filterTabBranch, d.tab == filterTabStatus}

	cells := make([]string, 2)
	for i := 0; i < 2; i++ {
		border := filterTabBorder
		if active[i] {
			border = filterActiveTabBorder
		}
		switch i {
		case 0: // first tab: bottom-left seams into the window's left border
			if active[i] {
				border.BottomLeft = "│"
			} else {
				border.BottomLeft = "├"
			}
		case 1: // last tab: bottom-right seams into the window's right border
			if active[i] {
				border.BottomRight = "│"
			} else {
				border.BottomRight = "┤"
			}
		}

		st := lipgloss.NewStyle().Border(border, true).Padding(0, 1).
			Align(lipgloss.Center).Width(widths[i])
		switch {
		case d.color && active[i]:
			st = st.BorderForeground(theme.ColorSecondary).Foreground(theme.ColorAccent).Bold(true)
		case d.color:
			st = st.BorderForeground(theme.ColorSecondary).Foreground(theme.ColorMuted)
		case active[i]:
			st = st.Bold(true)
		}
		cells[i] = st.Render(labels[i])
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, cells[0], cells[1])
}

// renderButtons draws the OK / Cancel / Reset row. OK and Cancel sit on the left;
// Reset is right-aligned to width (pass 0 to lay the row out at its natural width,
// used only to measure it). Every button carries a visible background (muted when
// idle, accent when focused).
func (d runFilterDialog) renderButtons(width int) string {
	ok := d.renderButton(filterBtnOK, "OK")
	cancel := d.renderButton(filterBtnCancel, "Cancel")
	reset := d.renderButton(filterBtnReset, "Reset")

	left := ok + " " + cancel
	if width <= 0 {
		return left + " " + reset
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(reset)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + reset
}

// button renders one button with a visible background (accent when focused, muted
// otherwise) and its first letter underlined as the mnemonic accelerator. The
// parts are rendered separately but all carry the same background, so there is no
// gap between them.
func (d runFilterDialog) renderButton(id runFilterButton, label string) string {
	focused := d.zone == filterZoneButtons && d.button == id
	base := lipgloss.NewStyle()
	switch {
	case d.color && focused:
		base = base.Foreground(lipgloss.Color("232")).Background(theme.ColorAccent)
	case d.color:
		base = base.Foreground(lipgloss.Color("232")).Background(theme.ColorMuted)
	case focused:
		base = base.Reverse(true)
	}
	rest := base
	if focused {
		rest = rest.Bold(true)
	}
	mnem := rest.Underline(true).Bold(true)

	r := []rune(label)
	return base.Render("  ") + mnem.Render(string(r[0])) + rest.Render(string(r[1:])) + base.Render("  ")
}

// bodyRows is the number of option rows the lists hold: the longer of the two, so
// both tabs reserve the same number of list rows.
func (d runFilterDialog) bodyRows() int {
	return max(len(d.scopes), len(d.statuses))
}

// splitWidth divides total into n parts as evenly as possible, handing the
// remainder to the leading parts, so the parts sum back to total.
func splitWidth(total, n int) []int {
	base := total / n
	out := make([]int, n)
	for i := range out {
		out[i] = base
	}
	for i := 0; i < total-base*n; i++ {
		out[i]++
	}
	return out
}
