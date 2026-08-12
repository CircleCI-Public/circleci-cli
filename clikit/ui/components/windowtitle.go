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

import tea "charm.land/bubbletea/v2"

// titlePrefix leads every window title so the terminal tab is recognisable as
// the CLI at a glance. The ◉ (fisheye) glyph echoes CircleCI's ringed logo
// mark, the way the `claude` CLI leads its title with ✳.
const titlePrefix = "◉ circleci"

// FlowTitle builds a terminal window/tab title for an interactive flow. Our
// full-screen flows set this so the hosting terminal's tab reflects what the
// CLI is doing — the way the `claude` CLI does. Bubble Tea emits it as OSC 2
// (via tea.View.WindowTitle) and clears it again when the program exits.
//
// An empty action yields the bare prefix, for generic flows (e.g. the pager)
// that have no single action to name.
func FlowTitle(action string) string {
	if action == "" {
		return titlePrefix
	}
	return titlePrefix + " · " + action
}

// WithWindowTitle stamps title onto v and returns it. Callers set it on every
// frame so the title stays put for the flow's lifetime; Bubble Tea only writes
// the escape when the value changes, and resets the terminal title on exit.
func WithWindowTitle(v tea.View, title string) tea.View {
	v.WindowTitle = title
	return v
}
