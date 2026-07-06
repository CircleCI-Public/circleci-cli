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
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// runFilterOutcome is the state of a runFilterDialog after an Update: still open,
// applied (the user confirmed a trigger + status selection), or cancelled.
type runFilterOutcome int

const (
	runFilterOpen runFilterOutcome = iota
	runFilterApply
	runFilterCancel
)

// runFilterTab is the active facet in the dialog: the trigger (scope) list, the
// status-filter list, or the created-age picker. The values index the tab bar
// (see components.Tabs), so their order must match filterTabLabels.
type runFilterTab int

const (
	filterTabTrigger runFilterTab = iota
	filterTabStatus
	filterTabCreated
)

// filterTabLabels are the tab bar's labels, in runFilterTab order.
var filterTabLabels = []string{"Trigger", "Status", "Created"}

// createdDuration is one selectable relative age on the Created tab: its display
// label and the duration it maps to (measured back from now).
type createdDuration struct {
	label    string
	duration time.Duration
}

// createdDurations are the fixed relative-age options on the Created tab, coarse
// buckets rather than a free-form date picker.
var createdDurations = []createdDuration{
	{"1 Hour", time.Hour},
	{"6 Hours", 6 * time.Hour},
	{"12 Hours", 12 * time.Hour},
	{"24 Hours", 24 * time.Hour},
	{"7 Days", 7 * 24 * time.Hour},
	{"2 Weeks", 14 * 24 * time.Hour},
	{"1 Month", 30 * 24 * time.Hour},
}

// createdAllDatesLabel is the Created date picker's first entry: it clears the
// created filter (no age constraint), analogous to the Status tab's "all
// statuses".
const createdAllDatesLabel = "all dates"

// dialog keys. "r" resets the selection to its defaults; Enter applies and esc
// cancels (via the shared bindings). Tab switching is owned by components.Tabs.
var filterKeyReset = key.NewBinding(key.WithKeys("r", "R"))

// runFilterDialog is the "/" search overlay on the run picker: a three-tab
// (Trigger / Status / Created) chooser that lets the user set the trigger scope,
// status filter and created-age window. It embeds a components.SelectModel list
// per tab (so the picker's navigation, scrolling and rendering are reused) and a
// components.Tabs bar for the chrome; the Created tab pairs its date list with an
// older/newer direction toggled by space. Enter applies, esc cancels, "r" resets.
// It never quits the program — the host reads Outcome() after each Update and
// acts on Apply/Cancel.
type runFilterDialog struct {
	scopes   []runScope
	statuses []RunStatusFilter

	triggerSel components.SelectModel
	statusSel  components.SelectModel
	// dateSel is the Created tab's age picker ("all dates" plus the relative-age
	// buckets); createdNewer is the older/newer direction the space key toggles.
	dateSel      components.SelectModel
	createdNewer bool

	tabs components.Tabs

	// defaultScope / defaultStatus are the indexes "r" (reset) restores (the
	// current branch and "all statuses" — always the first of each cycle).
	defaultScope  int
	defaultStatus int

	color         bool
	width, height int
	listHeight    int // rows given to each embedded list (see SetSize)
	outcome       runFilterOutcome
}

// newRunFilterDialog builds the dialog seeded with the currently active scope,
// status and created filter so it opens on what the picker is already showing.
// scopeIdx/statusIdx are the active selections; the defaults it resets to are the
// first of each cycle. created seeds the Created tab (an inactive zero value
// leaves the age on "all dates" and the direction on its "older" default).
func newRunFilterDialog(scopes []runScope, statuses []RunStatusFilter, scopeIdx, statusIdx int, created RunCreatedFilter, color bool) runFilterDialog {
	d := runFilterDialog{
		scopes:       scopes,
		statuses:     statuses,
		tabs:         components.NewTabs(filterTabLabels, color),
		createdNewer: created.Newer,
		color:        color,
	}
	d.triggerSel = d.newTriggerSelect(scopeIdx)
	d.statusSel = d.newStatusSelect(statusIdx)
	d.dateSel = d.newDateSelect(createdDateCursor(created))
	return d
}

// createdDateCursor maps a created filter to the date picker's cursor: the "all
// dates" entry (0) when inactive, otherwise the row of the matching age bucket.
func createdDateCursor(created RunCreatedFilter) int {
	if created.Active() {
		for i, cd := range createdDurations {
			if cd.duration == created.Duration {
				return i + 1 // +1 for the leading "all dates" entry
			}
		}
	}
	return 0
}

// newDateSelect builds the chrome-free Created date list seeded at idx. Its first
// entry ("all dates") clears the filter and is italicised (and tinted when color
// is on) so it reads as the special "no age filter" option, matching the Status
// tab's "all statuses".
func (d runFilterDialog) newDateSelect(idx int) components.SelectModel {
	color := d.color
	return components.NewSelectModel("", createdDateLabels()).
		WithCursor(idx).WithKeys().WithHeight(d.listHeight).
		WithItemStyleFunc(func(i int) lipgloss.Style {
			if i == 0 { // the "all dates" no-filter entry
				st := lipgloss.NewStyle().Italic(true)
				if color {
					st = st.Foreground(theme.ColorSecondary)
				}
				return st
			}
			return lipgloss.NewStyle()
		})
}

// createdDateLabels lists the date picker's option labels: the "all dates" clear
// entry followed by each relative-age bucket.
func createdDateLabels() []string {
	labels := make([]string, 0, len(createdDurations)+1)
	labels = append(labels, createdAllDatesLabel)
	for _, cd := range createdDurations {
		labels = append(labels, cd.label)
	}
	return labels
}

// activeTab is the currently selected tab, read from the tab bar.
func (d runFilterDialog) activeTab() runFilterTab { return runFilterTab(d.tabs.Active()) }

// newTriggerSelect builds the chrome-free trigger list (no title, no footer)
// seeded at idx. Each trigger carries a glyph (see scopeIcons): a heart for
// "my runs", and distinct marks for the current branch, default branch and
// "all branches". The current/default branch rows carry their branch name as a
// nested child (see scopeChildren) rather than a "[…]" bracket on the row.
func (d runFilterDialog) newTriggerSelect(idx int) components.SelectModel {
	return components.NewSelectModel("", d.scopeLabels()).
		WithCursor(idx).WithKeys().WithHeight(d.listHeight).
		WithIcons(d.scopeIcons()).
		WithChildren(d.scopeChildren())
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

// scopeLabels renders the Trigger tab's row labels — the scope roles ("current
// branch", "all branches", …). The branch name is carried separately as a nested
// child (see scopeChildren), not on the row.
func (d runFilterDialog) scopeLabels() []string {
	labels := make([]string, len(d.scopes))
	for i, s := range d.scopes {
		labels[i] = s.pickerLabel
	}
	return labels
}

// scopeChildren renders each trigger's nested branch-name sub-entry as "[branch]"
// (empty for scopes without one). When color is on the bracket is tinted with the
// secondary gold accent, matching the run picker title's scope bracket.
func (d runFilterDialog) scopeChildren() []string {
	children := make([]string, len(d.scopes))
	for i, s := range d.scopes {
		if s.pickerChild == "" {
			continue
		}
		child := "[" + s.pickerChild + "]"
		if d.color {
			child = theme.SecondaryStyle.Render(child)
		}
		children[i] = child
	}
	return children
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
	d.dateSel = d.dateSel.WithHeight(listHeight)
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

// Created returns the chosen created-age filter, or an inactive zero value when
// the date picker is on "all dates". Valid once Outcome() is runFilterApply.
func (d runFilterDialog) Created() RunCreatedFilter {
	idx := d.dateSel.Selected()
	if idx <= 0 || idx > len(createdDurations) {
		return RunCreatedFilter{} // "all dates" (0) or out of range → no filter
	}
	cd := createdDurations[idx-1] // -1 for the leading "all dates" entry
	return RunCreatedFilter{
		Newer:    d.createdNewer,
		Duration: cd.duration,
		Label:    cd.label,
	}
}

func (d runFilterDialog) Init() tea.Cmd { return nil }

// Update handles the dialog's keys. It never emits a command; tab switching is
// delegated to the tab bar, and everything else (up/down/paging, and space on the
// Created tab) drives the active tab. ctrl+c is left to the host (it quits the
// whole program), so the dialog binds esc (cancel), enter (apply) and "r" (reset).
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
	}

	// Tab switching (←/→, tab/shift+tab) is owned by the tab bar; a consumed key
	// switches tabs, anything else drives the active tab's list.
	if tabs, handled := d.tabs.Update(msg); handled {
		d.tabs = tabs
		return d, nil
	}

	d.forwardToList(msg)
	return d, nil
}

// reset restores the trigger and status selections to their defaults (the current
// branch, all statuses), returns the Created tab to "all dates" / older, and
// returns to the Trigger tab.
func (d *runFilterDialog) reset() {
	d.triggerSel = d.newTriggerSelect(d.defaultScope)
	d.statusSel = d.newStatusSelect(d.defaultStatus)
	d.dateSel = d.newDateSelect(0) // "all dates" — no created filter
	d.createdNewer = false         // older — the default direction
	d.tabs = d.tabs.SetActive(int(filterTabTrigger))
}

// forwardToList sends a message to whichever tab is active, keeping the embedded
// SelectModel's navigation behaviour. On the Created tab, space toggles the
// older/newer direction; every other key drives the date list.
func (d *runFilterDialog) forwardToList(msg tea.Msg) {
	switch d.activeTab() {
	case filterTabTrigger:
		updated, _ := d.triggerSel.Update(msg)
		d.triggerSel = updated.(components.SelectModel)
	case filterTabStatus:
		updated, _ := d.statusSel.Update(msg)
		d.statusSel = updated.(components.SelectModel)
	case filterTabCreated:
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeySpace) {
			d.createdNewer = !d.createdNewer
			return
		}
		updated, _ := d.dateSel.Update(msg)
		d.dateSel = updated.(components.SelectModel)
	}
}

func (d runFilterDialog) View() tea.View {
	dialog := d.renderDialog()
	hints := []key.Binding{
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "tab")),
		components.BindMove,
	}
	// Space toggles older/newer, which only means something on the Created tab; on
	// the list tabs it does nothing, so advertise it only there.
	if d.activeTab() == filterTabCreated {
		hints = append(hints, key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "older/newer")))
	}
	hints = append(hints,
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	)
	footer := components.Hints(hints...)

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

// renderDialog assembles the tabbed window: the tab bar (components.Tabs) draws
// the tab row and the rounded-bordered window frame around the tab body (options
// on the left, a help description on the right).
func (d runFilterDialog) renderDialog() string {
	return d.tabs.View(d.tabBody())
}

// tabBody lays out the active tab as two columns: the options list on the left, a
// vertical divider, and the tab's help description on the right. Both columns are
// sized to a fixed width and height (the same for either tab) so switching tabs
// never resizes the dialog. The options list is vertically centered in its
// column so a short list (e.g. the trigger picker) sits beside the middle of the
// taller help text rather than hugging the top.
func (d runFilterDialog) tabBody() string {
	h := d.panelHeight()
	// Centre the tab's content block horizontally within the column (as one unit,
	// so a list's rows keep their shared left edge) and vertically beside the help.
	content := centerHorizontally(strings.TrimRight(d.activeList(), "\n"), d.listColumnWidth())
	left := lipgloss.NewStyle().Width(d.listColumnWidth()).Height(h).
		AlignVertical(lipgloss.Center).
		Render(content)

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

// centerHorizontally shifts a (possibly multi-line) block right so it is centred
// within width, preserving the block's internal left alignment — rather than
// centring each line independently, which would leave a list's rows ragged. A
// block already at least as wide as width is returned unchanged.
func centerHorizontally(content string, width int) string {
	pad := (width - lipgloss.Width(content)) / 2
	if pad <= 0 {
		return content
	}
	return lipgloss.NewStyle().PaddingLeft(pad).Render(content)
}

// panelHeight is the fixed row count of the tab body: the tallest of the option
// lists (including the Created tab's stacked radios) and the tab help
// descriptions (wrapped to filterHelpWidth), so every tab renders at the same
// height regardless of which has more content.
func (d runFilterDialog) panelHeight() int {
	h := max(len(d.scopes), len(d.statuses))
	h = max(h, lipgloss.Height(d.createdBody()))
	for _, help := range []string{triggerHelpText, statusHelpText, createdHelpText} {
		wrapped := lipgloss.NewStyle().Width(filterHelpWidth).Render(help)
		h = max(h, lipgloss.Height(wrapped))
	}
	return h
}

// listColumnWidth is the width of the options column, the widest of the tabs'
// bodies so the column (and thus the dialog) stays the same width on every tab.
func (d runFilterDialog) listColumnWidth() int {
	return max(
		lipgloss.Width(strings.TrimRight(d.triggerSel.View().Content, "\n")),
		lipgloss.Width(strings.TrimRight(d.statusSel.View().Content, "\n")),
		lipgloss.Width(d.createdBody()),
	)
}

// activeList renders the embedded list for the active tab.
func (d runFilterDialog) activeList() string {
	switch d.activeTab() {
	case filterTabStatus:
		return d.statusSel.View().Content
	case filterTabCreated:
		return d.createdBody()
	case filterTabTrigger:
	}
	return d.triggerSel.View().Content
}

// createdBody renders the Created tab: the date list with the older/newer
// direction toggle stacked vertically to its right. The direction options carry a
// filled/hollow marker on the active one (space flips it); the whole toggle is
// muted while the filter is inactive ("all dates" selected), since direction is
// meaningless with no age.
func (d runFilterDialog) createdBody() string {
	dates := strings.TrimRight(d.dateSel.View().Content, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top,
		dates,
		lipgloss.NewStyle().PaddingLeft(6).Render(d.createdDirectionColumn()),
	)
}

// createdDirectionColumn renders the "older / newer" toggle as two stacked rows,
// each "● word" (active) or "○ word", accenting the active one — or muting the
// whole toggle when the created filter is inactive.
func (d runFilterDialog) createdDirectionColumn() string {
	inactive := !d.Created().Active()
	return d.directionWord("older", !d.createdNewer, inactive) + "\n" +
		d.directionWord("newer", d.createdNewer, inactive)
}

// directionWord renders one direction option as "● word" (active) or "○ word",
// accenting the active word when color is on, or muting it when the created
// filter is inactive.
func (d runFilterDialog) directionWord(word string, active, inactive bool) string {
	glyph := "○"
	if active {
		glyph = "●"
	}
	text := glyph + " " + word
	if !d.color {
		return text
	}
	switch {
	case inactive:
		return theme.HelperStyle.Render(text)
	case active:
		return theme.AccentStyle.Bold(true).Render(text)
	default:
		return theme.HelperStyle.Render(text)
	}
}

// tabHelp is the description shown on the right of the active tab.
func (d runFilterDialog) tabHelp() string {
	switch d.activeTab() {
	case filterTabStatus:
		return statusHelpText
	case filterTabCreated:
		return createdHelpText
	case filterTabTrigger:
	}
	return triggerHelpText
}

const (
	triggerHelpText = "Filter runs by trigger.\n\n" +
		"Pick a branch to list only its runs. \"all branches\" shows every branch; " +
		"\"my runs\" lists your runs across all projects."
	statusHelpText = "Filter runs by status.\n\n" +
		"\"all statuses\" clears the filter. Pick a status to show only the runs in " +
		"that state."
	createdHelpText = "Filter runs by age.\n\n" +
		"Pick how far back to look; \"all dates\" clears the filter. Press space to " +
		"toggle between older and newer than the chosen age."
)

// bodyRows is the number of option rows the tabs' lists hold: the most of any
// tab (the Created date list has the most, with its "all dates" entry plus the age
// buckets), so every tab reserves the same number of list rows and switching tabs
// never resizes the dialog.
func (d runFilterDialog) bodyRows() int {
	return max(len(d.scopes), len(d.statuses), len(createdDurations)+1)
}
