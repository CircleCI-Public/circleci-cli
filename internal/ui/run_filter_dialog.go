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
// applied (the user confirmed a trigger + status selection), or cancelled.
type runFilterOutcome int

const (
	runFilterOpen runFilterOutcome = iota
	runFilterApply
	runFilterCancel
)

// runFilterTab is the active facet in the dialog: the trigger (scope) list or the
// status-filter list.
type runFilterTab int

const (
	filterTabTrigger runFilterTab = iota
	filterTabStatus
)

// dialog keys. left/right (and tab/shift+tab) switch the active Trigger/Status
// tab; "r" resets the selection to its defaults. Enter applies and esc cancels
// (via the shared bindings).
var (
	filterKeyLeft  = key.NewBinding(key.WithKeys("left", "h"))
	filterKeyRight = key.NewBinding(key.WithKeys("right", "l"))
	filterKeyReset = key.NewBinding(key.WithKeys("r", "R"))
)

// runFilterDialog is the "/" search overlay on the run picker: a two-tab
// (Trigger / Status) chooser that lets the user set the trigger scope and status
// filter explicitly from lists. It embeds two components.SelectModel lists (one
// per tab) so the picker's navigation, scrolling and rendering are reused; the
// dialog only adds the tab bar on top. Enter applies, esc cancels, "r" resets.
// It never quits the program — the host reads Outcome() after each Update and
// acts on Apply/Cancel.
type runFilterDialog struct {
	scopes   []runScope
	statuses []RunStatusFilter

	triggerSel components.SelectModel
	statusSel  components.SelectModel

	tab runFilterTab

	// defaultScope / defaultStatus are the indexes "r" (reset) restores (the
	// current branch and "all statuses" — always the first of each cycle).
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
		tab:      filterTabTrigger,
		color:    color,
	}
	d.triggerSel = d.newTriggerSelect(scopeIdx)
	d.statusSel = d.newStatusSelect(statusIdx)
	return d
}

// newTriggerSelect builds the chrome-free trigger list (no title, no footer)
// seeded at idx. Each trigger carries a glyph (see scopeIcons): a heart for
// "my runs", and distinct marks for the current branch, default branch and
// "all branches".
func (d runFilterDialog) newTriggerSelect(idx int) components.SelectModel {
	return components.NewSelectModel("", d.scopeLabels()).
		WithCursor(idx).WithKeys().WithHeight(d.listHeight).
		WithIcons(d.scopeIcons())
}

// scopeIcons renders each trigger's glyph for the list, colored per its role
// (see triggerIconStyle) when color is on: blue for the current branch, green for
// the default branch, gold for "all branches", and a red heart for "my runs".
func (d runFilterDialog) scopeIcons() []string {
	icons := make([]string, len(d.scopes))
	for i, s := range d.scopes {
		icon := s.icon
		if d.color && icon != "" {
			if style, ok := triggerIconStyle(icon); ok {
				icon = style.Render(icon)
			}
		}
		icons[i] = icon
	}
	return icons
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

// scopeLabels renders the Trigger tab's row labels. When color is on, each
// label's trailing "[…]" (the branch name on the current/default branch scopes)
// is tinted with the secondary gold accent, matching the run picker title's
// scope bracket.
func (d runFilterDialog) scopeLabels() []string {
	labels := make([]string, len(d.scopes))
	for i, s := range d.scopes {
		label := s.pickerLabel
		if d.color {
			label = colorizeLabelBracket(label)
		}
		labels[i] = label
	}
	return labels
}

// colorizeLabelBracket tints a trailing "[…]" segment of a label with the
// secondary (gold) accent. A label with no trailing bracket (e.g. "all branches",
// "my runs") is returned unchanged.
func colorizeLabelBracket(label string) string {
	if !strings.HasSuffix(label, "]") {
		return label
	}
	open := strings.LastIndexByte(label, '[')
	if open < 0 {
		return label
	}
	return label[:open] + theme.SecondaryStyle.Render(label[open:])
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
	d.triggerSel = d.triggerSel.WithHeight(listHeight)
	d.statusSel = d.statusSel.WithHeight(listHeight)
	return d
}

// Outcome reports whether the dialog is still open, or the user applied or
// cancelled it. The host checks this after each Update.
func (d runFilterDialog) Outcome() runFilterOutcome { return d.outcome }

// Selected returns the chosen scope and status indexes. Valid once Outcome() is
// runFilterApply.
func (d runFilterDialog) Selected() (scopeIdx, statusIdx int) {
	return d.triggerSel.Selected(), d.statusSel.Selected()
}

func (d runFilterDialog) Init() tea.Cmd { return nil }

// Update handles the dialog's keys. It never emits a command; navigation (up/
// down/paging) is forwarded to the active tab's embedded list. ctrl+c is left to
// the host (it quits the whole program), so the dialog binds esc (cancel), enter
// (apply), "r" (reset), and left/right + tab/shift+tab (switch the Trigger/Status
// tab).
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
	case key.Matches(k, components.KeyEnter):
		d.outcome = runFilterApply
		return d, nil
	case key.Matches(k, filterKeyReset):
		d.reset()
		return d, nil
	case key.Matches(k, filterKeyLeft):
		d.tab = filterTabTrigger
		return d, nil
	case key.Matches(k, filterKeyRight):
		d.tab = filterTabStatus
		return d, nil
	case key.Matches(k, components.KeyTab, components.KeyShiftTab):
		if d.tab == filterTabTrigger {
			d.tab = filterTabStatus
		} else {
			d.tab = filterTabTrigger
		}
		return d, nil
	}

	// Any other key (up/down, paging, g/G) drives the active tab's list.
	d.forwardToList(msg)
	return d, nil
}

// reset restores the trigger and status selections to their defaults (the current
// branch, all statuses) and returns to the Trigger tab.
func (d *runFilterDialog) reset() {
	d.triggerSel = d.newTriggerSelect(d.defaultScope)
	d.statusSel = d.newStatusSelect(d.defaultStatus)
	d.tab = filterTabTrigger
}

// forwardToList sends a message to whichever tab's list is active, keeping the
// embedded SelectModel's navigation and scrolling behaviour.
func (d *runFilterDialog) forwardToList(msg tea.Msg) {
	if d.tab == filterTabTrigger {
		updated, _ := d.triggerSel.Update(msg)
		d.triggerSel = updated.(components.SelectModel)
	} else {
		updated, _ := d.statusSel.Update(msg)
		d.statusSel = updated.(components.SelectModel)
	}
}

func (d runFilterDialog) View() tea.View {
	dialog := d.renderDialog()
	footer := components.Hints(
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "tab")),
		components.BindMove,
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset")),
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

// filterHelpWidth is the column width of the right-hand help panel in each tab.
const filterHelpWidth = 34

// renderDialog assembles the tabbed window: a row of tabs whose bottom edge forms
// the window's top border, over a rounded-bordered body holding the tab body
// (options on the left, a help description on the right). The window is sized to
// the content, and the tab row is split to the same width so the two seam
// together.
func (d runFilterDialog) renderDialog() string {
	body := d.tabBody()
	contentWidth := max(lipgloss.Width(body), 44)
	// rowWidth is the dialog's outer width. lipgloss .Width() here sets the outer
	// box width (border + padding included), so the window is contentWidth +
	// border(2) + padding(2); the tab row is set to the same width so they line up.
	rowWidth := contentWidth + 4
	if rowWidth%2 != 0 {
		rowWidth++ // even split across the two tabs
	}

	window := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false). // the tab row supplies the top edge
		Padding(0, 1).
		Width(rowWidth)
	if d.color {
		window = window.BorderForeground(theme.ColorSecondary)
	}

	return lipgloss.JoinVertical(lipgloss.Left, d.renderTabs(rowWidth), window.Render(body))
}

// tabBody lays out the active tab as two columns: the options list on the left, a
// vertical divider, and the tab's help description on the right. Both columns are
// sized to a fixed width and height (the same for either tab) so switching tabs
// never resizes the dialog. The options list is vertically centered in its
// column so a short list (e.g. the trigger picker) sits beside the middle of the
// taller help text rather than hugging the top.
func (d runFilterDialog) tabBody() string {
	h := d.panelHeight()
	left := lipgloss.NewStyle().Width(d.listColumnWidth()).Height(h).
		AlignVertical(lipgloss.Center).
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
	for _, help := range []string{triggerHelpText, statusHelpText} {
		wrapped := lipgloss.NewStyle().Width(filterHelpWidth).Render(help)
		h = max(h, lipgloss.Height(wrapped))
	}
	return h
}

// listColumnWidth is the width of the options column, the wider of the two lists
// so the column (and thus the dialog) stays the same width on either tab.
func (d runFilterDialog) listColumnWidth() int {
	return max(
		lipgloss.Width(strings.TrimRight(d.triggerSel.View().Content, "\n")),
		lipgloss.Width(strings.TrimRight(d.statusSel.View().Content, "\n")),
	)
}

// activeList renders the embedded list for the active tab.
func (d runFilterDialog) activeList() string {
	if d.tab == filterTabTrigger {
		return d.triggerSel.View().Content
	}
	return d.statusSel.View().Content
}

// tabHelp is the description shown on the right of the active tab.
func (d runFilterDialog) tabHelp() string {
	if d.tab == filterTabTrigger {
		return triggerHelpText
	}
	return statusHelpText
}

const (
	triggerHelpText = "Filter runs by trigger.\n\n" +
		"Pick a branch to list only its runs. \"all branches\" shows every branch; " +
		"\"my runs\" lists your runs across all projects."
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
	labels := [2]string{"Trigger", "Status"}
	active := [2]bool{d.tab == filterTabTrigger, d.tab == filterTabStatus}

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
