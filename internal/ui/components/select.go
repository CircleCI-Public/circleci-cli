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

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// SelectModel is a single-choice picker rendered as a vertical list with a
// cursor (›) marking the focused option. ↑/↓ or k/j move; Enter confirms;
// Esc/Ctrl+C cancels. When the option list is taller than the available height
// the list scrolls to keep the cursor visible (see WithHeight).
type SelectModel struct {
	prompt  string
	options []string
	icons   []string
	keys    []key.Binding // footer key bindings; nil renders no hint line
	help    help.Model    // renders keys into the muted footer line
	note    string        // optional lines rendered between the title and the options
	cursor  int
	offset  int // index of the first visible option when the list scrolls
	height  int // terminal rows available; 0 = unlimited (render every option)
	chosen  bool
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

// reservedRows is the number of non-option lines the view occupies: the prompt,
// the hint, and each line of the note (when set).
func (m SelectModel) reservedRows() int {
	reserved := 2 // prompt + hint
	if m.note != "" {
		reserved += strings.Count(m.note, "\n") + 1
	}
	return reserved
}

// visibleRows is how many option rows fit, reserving lines for the prompt, hint
// and note. Zero height (or a list that already fits) means no limit.
func (m SelectModel) visibleRows() int {
	reserved := m.reservedRows()
	if m.height <= 0 || m.height-reserved >= len(m.options) {
		return len(m.options)
	}
	if rows := m.height - reserved; rows > 0 {
		return rows
	}
	return 1
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
	b.WriteString(theme.TitleStyle.Render("? "+m.prompt) + "\n")

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
	for i := start; i < end; i++ {
		opt := m.options[i]
		// The icon (if any) sits before the label and outside the accent style,
		// so its status color is preserved on the highlighted row. The label is
		// styled separately. With no icon this reduces to the original layout.
		p := m.iconPrefix(i)
		switch {
		case i == m.cursor && p != "":
			b.WriteString(theme.AccentStyle.Render("› ") + p + theme.AccentStyle.Render(opt) + "\n")
		case i == m.cursor:
			b.WriteString(theme.AccentStyle.Render("› "+opt) + "\n")
		default:
			b.WriteString("  " + p + opt + "\n")
		}
	}

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

// iconPrefix returns option i's icon followed by a space, or "" when the option
// has no icon. The trailing space gives a one-column gap before the label.
func (m SelectModel) iconPrefix(i int) string {
	if i < len(m.icons) && m.icons[i] != "" {
		return m.icons[i] + " "
	}
	return ""
}
