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
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// helpKeys is the footer key hint set for the help overlay. esc/q both dismiss
// it, so they share a single binding.
var helpKeys = []key.Binding{
	BindScroll,
	BindTopBottom,
	BindSearch,
	key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "back")),
}

// HelpModel is a scrollable full-screen help overlay: markdown rendered in a
// frame (a PagerModel, so it scrolls and offers the less-style "/" search),
// meant to be embedded in another program that opens it on "?" and dismisses it
// on esc/q. Unlike MarkdownViewportModel — which is its own program and quits on
// esc/q — HelpModel records the dismissal via Dismissed() so the host can route
// back to whatever it was showing rather than exiting.
//
// Content is produced by a render callback so the markdown re-wraps to the live
// terminal width on resize. Build with NewHelp; the zero value is not usable.
type HelpModel struct {
	pager     PagerModel
	dismissed bool
}

// NewHelp returns a help overlay that displays the markdown produced by render.
// render is given the column width the content must fit into and is re-invoked
// on every resize so the markdown re-wraps.
func NewHelp(render func(width int) string) HelpModel {
	return HelpModel{
		pager: NewPager().
			WithKeys(helpKeys...).
			WithReflow(render).
			WithBorder(theme.ColorSecondary),
	}
}

// Dismissed reports whether the user closed the overlay (esc/q). The host checks
// this after Update to decide whether to route back to its previous view.
func (m HelpModel) Dismissed() bool { return m.dismissed }

// Searching reports whether the "/" search prompt is open, so the host can let
// esc/ctrl+c reach the prompt (cancel) instead of treating them as its own keys.
func (m HelpModel) Searching() bool { return m.pager.Searching() }

// SetSize applies a terminal size to the underlying pager, re-wrapping the
// content to the new width.
func (m HelpModel) SetSize(width, height int) HelpModel {
	m.pager = m.pager.SetSize(width, height)
	return m
}

// Reopen clears the dismissed flag, drops any prior search, and scrolls to the
// top, readying the overlay for a fresh open.
func (m HelpModel) Reopen() HelpModel {
	m.dismissed = false
	m.pager = m.pager.ResetSearch().GotoTop()
	return m
}

func (m HelpModel) Init() tea.Cmd { return nil }

// Update handles scrolling and the "/" search via the pager, and binds esc/q to
// dismiss the overlay (esc first clears an active search). ctrl+c is left to the
// host. Guarding on Searching keeps esc/q typed into the "/" prompt while it is
// open rather than closing the overlay.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.pager.Searching() {
		switch {
		case key.Matches(k, KeyEsc):
			if m.pager.SearchActive() {
				m.pager = m.pager.ClearSearch()
				return m, nil
			}
			m.dismissed = true
			return m, nil
		case key.Matches(k, BindQuit):
			m.dismissed = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.pager, cmd = m.pager.Update(msg)
	return m, cmd
}

func (m HelpModel) View() tea.View {
	return m.pager.View("")
}
