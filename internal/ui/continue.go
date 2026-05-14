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

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// ContinueModel is a bubbletea model for a simple Enter-to-continue gate.
// Esc / Ctrl+C cancel. Enter confirms.
type ContinueModel struct {
	message   string
	cancelled bool
	done      bool
}

func NewContinueModel(message string) ContinueModel {
	return ContinueModel{message: message}
}

func (m ContinueModel) Cancelled() bool { return m.cancelled }

func (m ContinueModel) Init() tea.Cmd { return nil }

func (m ContinueModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC, components.KeyEsc:
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case components.KeyEnter:
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ContinueModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(theme.HelperStyle.Render(m.message))
}
