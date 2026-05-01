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
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui/theme"
)

// SecretModel is a bubbletea model for prompting a secret value.
// Input is masked (echo password mode). The prompt header is configurable
// so callers can name the specific secret being requested.
type SecretModel struct {
	textInput textinput.Model
	header    string
	quitting  bool
	value     string
}

// NewSecretModel creates a SecretModel with the given header text.
func NewSecretModel(header string) SecretModel {
	ti := textinput.New()
	ti.SetVirtualCursor(false)
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()

	return SecretModel{textInput: ti, header: header}
}

// Quitting reports whether the user pressed Esc or Ctrl+C.
func (m SecretModel) Quitting() bool { return m.quitting }

// Value returns the entered secret value.
func (m SecretModel) Value() string { return m.value }

func (m SecretModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SecretModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC, components.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case components.KeyEnter:
			m.value = m.textInput.Value()
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SecretModel) View() tea.View {
	if m.value != "" {
		return tea.NewView("")
	}

	var c *tea.Cursor
	if !m.textInput.VirtualCursor() {
		c = m.textInput.Cursor()
		c.Y += lipgloss.Height(m.headerView())
	}

	str := lipgloss.JoinVertical(lipgloss.Top, m.headerView(), m.textInput.View(), m.footerView())
	if m.quitting {
		str += "\n"
	}

	v := tea.NewView(str)
	v.Cursor = c
	return v
}

func (m SecretModel) headerView() string { return theme.TitleStyle.Render(m.header) }
func (m SecretModel) footerView() string { return theme.HelperStyle.Render("(esc to quit)") }
