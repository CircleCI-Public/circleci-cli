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

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
)

// PreambleModel is a bubbletea model for an "Enter to continue · Esc to cancel"
// gate shown before a multi-step orchestrator runs. Default is opt-in: Enter
// proceeds, Esc/Ctrl+C cancels.
type PreambleModel struct {
	title   string
	bullets []string
	dir     string
	proceed bool
	done    bool
}

// NewPreambleModel returns a PreambleModel rendering the given title, bullet
// lines, and directory context.
func NewPreambleModel(title, dir string, bullets []string) PreambleModel {
	return PreambleModel{title: title, dir: dir, bullets: bullets}
}

// Proceed reports whether the user pressed Enter to continue. False when
// cancelled via Esc or Ctrl+C.
func (m PreambleModel) Proceed() bool { return m.proceed }

// Done reports whether the user has made a choice.
func (m PreambleModel) Done() bool { return m.done }

func (m PreambleModel) Init() tea.Cmd { return nil }

func (m PreambleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, components.KeyEnter):
			m.proceed = true
			m.done = true
			return m, tea.Quit
		case key.Matches(keyMsg, components.KeyEsc, components.KeyCtrlC):
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PreambleModel) View() tea.View {
	var sb strings.Builder
	sb.WriteString(m.title + "\n")
	for _, b := range m.bullets {
		sb.WriteString("  • " + b + "\n")
	}
	if m.dir != "" {
		sb.WriteString("\nThis will run in: " + m.dir + "\n")
	}
	if !m.done {
		sb.WriteString("\nPress Enter to continue · Esc to cancel\n")
	}
	return tea.NewView(sb.String())
}
