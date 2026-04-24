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
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var confirmTitleStyle = lipgloss.NewStyle().Bold(true).MarginRight(2)

// ConfirmModel is a bubbletea model for a y/N confirmation prompt.
// Press y/Y to confirm, n/N/esc/ctrl+c to decline.
type ConfirmModel struct {
	prompt    string
	confirmed bool
	done      bool
}

func NewConfirmModel(prompt string) ConfirmModel {
	return ConfirmModel{prompt: prompt}
}

func (m ConfirmModel) Confirmed() bool { return m.confirmed }
func (m ConfirmModel) Done() bool      { return m.done }

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "N", "esc", "ctrl+c", "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfirmModel) View() tea.View {
	title := confirmTitleStyle.Render(m.prompt)

	var input string
	if m.done {
		answer := "N"
		if m.confirmed {
			answer = "y"
		}
		input = "[y/N] " + answer + "\n"
	} else {
		input = "[y/N] "
	}

	return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, title, input))
}
