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
)

// SelectModel is a top-level bubbletea model that wraps components.SelectModel
// and quits the program on selection or cancellation.
type SelectModel struct {
	inner     components.SelectModel
	cancelled bool
}

// NewSelectModel creates a standalone select prompt.
// options is the list of choices to display.
func NewSelectModel(prompt string, options []string) SelectModel {
	return SelectModel{
		inner: components.NewSelectModel(prompt, options),
	}
}

// Selected returns the index of the chosen option. Only valid when !Cancelled().
func (m SelectModel) Selected() int { return m.inner.Selected() }

// Cancelled reports whether the user quit without selecting.
func (m SelectModel) Cancelled() bool { return m.cancelled }

func (m SelectModel) Init() tea.Cmd { return nil }

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyEsc, components.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		case components.KeyEnter:
			updated, _ := m.inner.Update(msg)
			m.inner = updated.(components.SelectModel)
			return m, tea.Quit
		}
	}
	updated, cmd := m.inner.Update(msg)
	m.inner = updated.(components.SelectModel)
	return m, cmd
}

func (m SelectModel) View() tea.View {
	return m.inner.View()
}
