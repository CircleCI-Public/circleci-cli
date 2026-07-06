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

package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/list"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// SelectModel is a single-choice picker rendered as a vertical list with a
// cursor (›) marking the focused option. ↑/↓ or k/j move; Enter confirms;
// Esc/Ctrl+C cancels. When the option list is taller than the available height
// the list scrolls to keep the cursor visible (see WithHeight).
type SelectModel struct {
	prompt   string
	options  []string
	icons    []string
	children []string      // optional nested, non-selectable sub-entry per option
	keys     []key.Binding // footer key bindings; nil renders no hint line
	help     help.Model    // renders keys into the muted footer line
	note     string        // optional lines rendered between the title and the options
	cursor   int
	offset   int // index of the first visible option when the list scrolls
	height   int // terminal rows available; 0 = unlimited (render every option)
	chosen   bool

	// itemStyleFunc, when set, supplies a per-option label style so a caller can
	// mark a row as special (e.g. a muted "all statuses" no-filter entry). It is
	// overlaid with the accent foreground on the cursor row so selection still
	// reads. Nil means every label renders plain (accent only on the cursor).
	itemStyleFunc func(i int) lipgloss.Style
}

func NewSelectModel(prompt string, options []string) SelectModel {
	return SelectModel{
		prompt:  prompt,
		options: options,
		keys:    []key.Binding{BindMove, BindSelect, BindQuitEsc},
		help:    footerHelp(),
	}
}

// WithKeys returns a copy of the model with custom footer key bindings, replacing
// the default (move / select / quit) hint line.
func (m SelectModel) WithKeys(keys ...key.Binding) SelectModel {
	m.keys = keys
	return m
}

// WithNote returns a copy of the model with an informational note rendered
// between the title and the options (e.g. a run's config error). An empty note
// renders nothing. The note is emitted verbatim — style it in the caller if
// color is wanted — and may span multiple lines, which are reserved for when
// the option list scrolls.
func (m SelectModel) WithNote(note string) SelectModel {
	m.note = note
	m.clampOffset()
	return m
}

// WithItemStyleFunc returns a copy whose option labels are styled per-index by
// fn. The returned style applies to the label text (not the icon or cursor
// arrow); on the cursor row the accent foreground is layered on top so the
// highlighted row still stands out while keeping fn's other attributes (e.g.
// italic). Use it to give a special row a distinct look.
func (m SelectModel) WithItemStyleFunc(fn func(i int) lipgloss.Style) SelectModel {
	m.itemStyleFunc = fn
	return m
}

// WithIcons attaches an optional status icon to each option. icons is parallel
// to the options passed to NewSelectModel; an empty string means "no icon" for
// that row. Each icon is rendered in a fixed column before the label and is
// emitted verbatim — already styled by the caller if color is wanted — outside
// the cursor/selection styling, so a status color survives even on the
// highlighted or chosen row. Rows align one column further in when any option
// carries an icon.
func (m SelectModel) WithIcons(icons []string) SelectModel {
	m.icons = icons
	return m
}

// WithChildren attaches an optional nested sub-entry to each option. children is
// parallel to the options; an empty string means "no child" for that row. A
// child renders as a muted, non-selectable "└ value" line indented beneath its
// option (e.g. a branch name under a trigger), keeping a long attribute off the
// option's own row. Children are meant for short lists shown in full; the list
// still scrolls by option when it overflows, but the window is sized to show
// every option and its children together, so a caller relying on children should
// leave enough height for them.
func (m SelectModel) WithChildren(children []string) SelectModel {
	m.children = children
	m.clampOffset()
	return m
}

// WithCursor returns a copy of the model with the initial cursor positioned at
// index i, clamped to the available options. Use this to pre-select a default
// choice.
func (m SelectModel) WithCursor(i int) SelectModel {
	switch {
	case i < 0:
		i = 0
	case i >= len(m.options):
		i = len(m.options) - 1
	}
	m.cursor = i
	m.clampOffset()
	return m
}

// WithHeight sets the number of terminal rows available to the picker. When the
// option list is taller than this, the list scrolls to keep the cursor visible
// and a position indicator ("(3–12 of 40)") is appended to the hint. Zero (the
// default) imposes no limit and renders every option.
func (m SelectModel) WithHeight(rows int) SelectModel {
	m.height = rows
	m.clampOffset()
	return m
}

// Selected returns the index chosen by the user. Only valid when Done().
func (m SelectModel) Selected() int { return m.cursor }

// Done reports whether the user has confirmed a selection.
func (m SelectModel) Done() bool { return m.chosen }

func (m SelectModel) Init() tea.Cmd { return nil }

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.clampOffset()
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, KeyEnter):
			m.chosen = true
		case key.Matches(msg, KeyUp):
			if m.cursor > 0 {
				m.cursor--
			}
			m.clampOffset()
		case key.Matches(msg, KeyDown):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
			m.clampOffset()
		case key.Matches(msg, KeyPageUp):
			m.cursor -= m.visibleRows()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampOffset()
		case key.Matches(msg, KeyPageDown):
			m.cursor += m.visibleRows()
			if m.cursor > len(m.options)-1 {
				m.cursor = len(m.options) - 1
			}
			m.clampOffset()
		case key.Matches(msg, KeyTop):
			m.cursor = 0
			m.clampOffset()
		case key.Matches(msg, KeyBottom):
			m.cursor = len(m.options) - 1
			m.clampOffset()
		}
	}
	return m, nil
}

// reservedRows is the number of non-option lines the view occupies: the hint,
// the prompt (when set), and each line of the note (when set). The prompt is
// omitted for an embedded picker (empty prompt), so it reserves no row then.
func (m SelectModel) reservedRows() int {
	reserved := 1 // hint
	if m.prompt != "" {
		reserved++
	}
	if m.note != "" {
		reserved += strings.Count(m.note, "\n") + 1
	}
	return reserved
}

// visibleRows is how many option rows fit, reserving lines for the prompt, hint
// and note. Zero height (or a list that already fits, counting each option's
// nested child line) means no limit.
func (m SelectModel) visibleRows() int {
	reserved := m.reservedRows()
	if m.height <= 0 || m.height-reserved >= len(m.options)+m.childLines() {
		return len(m.options)
	}
	if rows := m.height - reserved; rows > 0 {
		return rows
	}
	return 1
}

// childLines is the number of options carrying a nested child sub-entry (each
// renders one extra line beneath its option).
func (m SelectModel) childLines() int {
	n := 0
	for _, c := range m.children {
		if c != "" {
			n++
		}
	}
	return n
}

// childAt returns option i's nested sub-entry, or "" when it has none.
func (m SelectModel) childAt(i int) string {
	if i >= 0 && i < len(m.children) {
		return m.children[i]
	}
	return ""
}

// clampOffset scrolls the visible window so the cursor stays inside it and the
// window never runs past the end of the list.
func (m *SelectModel) clampOffset() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if maxOffset := len(m.options) - rows; m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m SelectModel) View() tea.View {
	var b strings.Builder
	// An empty prompt (an embedded picker, e.g. inside a tabbed dialog) renders
	// no title line so the host owns the heading.
	if m.prompt != "" {
		b.WriteString(theme.TitleStyle.Render("? "+m.prompt) + "\n")
	}

	if m.note != "" {
		b.WriteString(m.note + "\n")
	}

	if m.chosen {
		b.WriteString("  " + m.iconPrefix(m.cursor) + theme.SuccessStyle.Render(m.options[m.cursor]) + "\n")
		return tea.NewView(b.String())
	}

	rows := m.visibleRows()
	start := m.offset
	end := start + rows
	if end > len(m.options) {
		end = len(m.options)
	}
	b.WriteString(m.renderOptions(start, end) + "\n")

	hint := m.help.ShortHelpView(m.keys)
	if rows < len(m.options) {
		// The list is scrolling; show which slice of it is visible.
		pos := fmt.Sprintf("(%d–%d of %d)", start+1, end, len(m.options))
		if hint != "" {
			hint += "  "
		}
		hint += theme.HelperStyle.Render(pos)
	}
	b.WriteString(hint)
	return tea.NewView(b.String())
}

// renderOptions renders the visible window [start, end) as a lipgloss list. The
// enumerator column is unused (each row builds its own "› "/"  " prefix) so that
// the cursor arrow, its space and the label render as a single styled run for the
// highlighted row. Keeping that text contiguous matters: bubbletea diffs frames
// and a PTY (especially Windows ConPTY) may split a row that is emitted as
// several separately-styled pieces, which breaks terminals and tests that match
// on the visible row. A status icon, when present, is still kept outside the
// accent style so its own color survives.
func (m SelectModel) renderOptions(start, end int) string {
	l := list.New().
		Enumerator(func(list.Items, int) string { return "" }).
		EnumeratorStyle(lipgloss.NewStyle())
	for i := start; i < end; i++ {
		l.Item(m.renderRow(i))
		if child := m.childAt(i); child != "" {
			l.Item(m.renderChild(i, child))
		}
	}
	return l.String()
}

// renderChild renders a non-selectable sub-entry on its own line beneath option
// i, aligned with where the option's label text begins (i.e. past the "› "/"  "
// cursor prefix and the icon column). The value is emitted verbatim so a caller
// can style it (e.g. tint a branch name).
func (m SelectModel) renderChild(i int, child string) string {
	indent := len("  ") + lipgloss.Width(m.iconPrefix(i))
	return strings.Repeat(" ", indent) + child
}

// renderRow renders option i as "› label" (cursor) or "  label", styled so the
// row's visible text stays contiguous. The icon, when present, sits between the
// prefix and the label outside the accent style.
func (m SelectModel) renderRow(i int) string {
	prefix := "  "
	if i == m.cursor {
		prefix = "› "
	}
	label, icon := m.options[i], m.iconPrefix(i)
	switch {
	case icon != "" && i == m.cursor:
		return theme.AccentStyle.Render(prefix) + icon + m.labelStyle(i).Render(label)
	case icon != "":
		return prefix + icon + m.labelStyle(i).Render(label)
	case i == m.cursor:
		// One contiguous run: the arrow, its space and the label.
		return m.labelStyle(i).Render(prefix + label)
	default:
		return prefix + m.labelStyle(i).Render(label)
	}
}

// labelStyle is the style applied to option i's label: the caller-supplied
// per-item style (WithItemStyleFunc), with the accent foreground layered on when
// the row is the cursor so the highlighted row still stands out. With no per-item
// style this is the plain label, accent-colored only on the cursor — matching the
// original rendering.
func (m SelectModel) labelStyle(i int) lipgloss.Style {
	var st lipgloss.Style
	if m.itemStyleFunc != nil {
		st = m.itemStyleFunc(i)
	}
	if i == m.cursor {
		st = st.Foreground(theme.ColorAccent)
	}
	return st
}

// iconPrefix returns option i's icon followed by a space, or "" when the option
// has no icon. The trailing space gives a one-column gap before the label.
func (m SelectModel) iconPrefix(i int) string {
	if i < len(m.icons) && m.icons[i] != "" {
		return m.icons[i] + " "
	}
	return ""
}
