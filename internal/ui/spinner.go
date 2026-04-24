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
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// SpinnerModel is a bubbletea model that displays an animated spinner with a
// message. Run it in a goroutine via tea.NewProgram; call p.Quit() to stop it.
type SpinnerModel struct {
	spinner spinner.Model
	msg     string
}

func NewSpinnerModel(msg string, color bool) SpinnerModel {
	s := spinner.New()
	if color {
		s.Spinner = spinner.MiniDot
		s.Style = spinnerStyle
	} else {
		s.Spinner = spinner.Line
	}
	return SpinnerModel{spinner: s, msg: msg}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s, cmd := m.spinner.Update(msg)
	m.spinner = s
	return m, cmd
}

func (m SpinnerModel) View() tea.View {
	return tea.NewView(m.spinner.View() + " " + m.msg)
}
