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
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// SelectModel is a single-choice picker rendered as a vertical list with a
// cursor (›) marking the focused option. ↑/↓ or k/j move; Enter confirms;
// Esc/Ctrl+C cancels.
type SelectModel struct {
	prompt  string
	options []string
	icons   []string
	hint    string
	cursor  int
	chosen  bool
}

func NewSelectModel(prompt string, options []string) SelectModel {
	return SelectModel{
		prompt:  prompt,
		options: options,
		hint:    "(↑/↓ to move, enter to select, esc to quit)",
	}
}

// WithHint returns a copy of the model with a custom footer hint line.
func (m SelectModel) WithHint(hint string) SelectModel {
	m.hint = hint
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
	return m
}

// Selected returns the index chosen by the user. Only valid when Done().
func (m SelectModel) Selected() int { return m.cursor }

// Done reports whether the user has confirmed a selection.
func (m SelectModel) Done() bool { return m.chosen }

func (m SelectModel) Init() tea.Cmd { return nil }

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case KeyEnter:
			m.chosen = true
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m SelectModel) View() tea.View {
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("? "+m.prompt) + "\n")

	if m.chosen {
		b.WriteString("  " + m.iconPrefix(m.cursor) + theme.SuccessStyle.Render(m.options[m.cursor]) + "\n")
		return tea.NewView(b.String())
	}

	for i, opt := range m.options {
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
	b.WriteString(theme.HelperStyle.Render(m.hint))
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
