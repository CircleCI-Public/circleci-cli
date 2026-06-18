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
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// PreviewModel is a display-only pane that frames pre-rendered content inside a
// bordered box. It does no rendering of its own — callers supply already-styled
// content (e.g. glamour-rendered markdown) and the box size, and PreviewModel
// only draws the frame. It is used as the right-hand pane of the theme picker to
// show sample markdown rendered in the highlighted theme.
//
// PreviewModel is not a standalone bubbletea program: it has no Update loop and
// View returns a plain string so a parent model can compose it horizontally with
// other panes.
type PreviewModel struct {
	content string
	width   int // total outer width of the box, including border and padding
	height  int // total outer height of the box, including border
}

// previewFramePadding is the columns/rows lipgloss border + horizontal padding
// add around the content. The border costs 1 cell on each side (2 cols, 2 rows)
// and the padding adds 1 col on each side (2 cols).
const (
	previewBorderSize = 2 // left+right border, or top+bottom border
	previewPaddingX   = 2 // one column of padding on each side

	// PreviewFrameWidth is how many columns the frame steals from the box's
	// outer width before content. Callers subtract it from the pane width to get
	// the column count content must wrap to (e.g. glamour word-wrap width).
	PreviewFrameWidth = previewBorderSize + previewPaddingX

	// PreviewFrameHeight is the rows the frame consumes, so callers can size the
	// box to leave room for surrounding chrome.
	PreviewFrameHeight = previewBorderSize // top+bottom border
)

// NewPreviewModel returns an empty preview pane.
func NewPreviewModel() PreviewModel {
	return PreviewModel{}
}

// WithContent returns a copy of the pane displaying content. Content is assumed
// to be pre-wrapped to ContentWidth(); anything wider is clipped by the box.
func (m PreviewModel) WithContent(content string) PreviewModel {
	m.content = content
	return m
}

// WithSize returns a copy of the pane sized to the given outer width and height
// (in terminal cells), border and padding included.
func (m PreviewModel) WithSize(width, height int) PreviewModel {
	m.width = width
	m.height = height
	return m
}

// ContentWidth is the column count available for content inside the frame.
// Callers wrap their content (e.g. via glamour) to this width.
func (m PreviewModel) ContentWidth() int {
	w := m.width - PreviewFrameWidth
	if w < 1 {
		return 1
	}
	return w
}

// ContentHeight is the row count available for content inside the frame. Callers
// use it to vertically place content (e.g. to center a loading placeholder).
func (m PreviewModel) ContentHeight() int {
	h := m.height - PreviewFrameHeight
	if h < 1 {
		return 1
	}
	return h
}

// View renders the bordered content box as a plain string for a parent model to
// place. Returns "" until a size has been set.
func (m PreviewModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	innerWidth := m.width - previewBorderSize
	innerHeight := m.height - previewBorderSize
	if innerHeight < 1 {
		innerHeight = 1
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorMuted).
		Padding(0, 1).
		Width(innerWidth).
		Height(innerHeight).
		MaxHeight(innerHeight + previewBorderSize).
		Render(m.content)
}
