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
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// This file is the single source of truth for every key the TUI reacts to.
// Update loops dispatch by matching a message against these bindings with
// key.Matches (rather than comparing raw msg.String() values), and footers
// render the bindings that carry help text via SelectModel/PagerModel.WithKeys
// or Hints. Defining each keystroke once here keeps dispatch and the on-screen
// hints from drifting apart.

// Dispatch bindings. These carry no help text — they never appear in a footer,
// they only classify a key press. Movement is split per-direction because up and
// down (etc.) trigger different actions; the grouped display bindings below layer
// the combined "↑/↓" hint label on top of the same keystrokes.
var (
	KeyEnter    = key.NewBinding(key.WithKeys("enter"))
	KeyEsc      = key.NewBinding(key.WithKeys("esc"))
	KeyCtrlC    = key.NewBinding(key.WithKeys("ctrl+c"))
	KeyYes      = key.NewBinding(key.WithKeys("y", "Y"))
	KeyNo       = key.NewBinding(key.WithKeys("n", "N"))
	KeyTab      = key.NewBinding(key.WithKeys("tab"))
	KeyShiftTab = key.NewBinding(key.WithKeys("shift+tab"))
	KeySpace    = key.NewBinding(key.WithKeys(" ", "space"))

	// List/viewport movement. k/j accompany the arrows for vim-style navigation.
	KeyUp       = key.NewBinding(key.WithKeys("up", "k"))
	KeyDown     = key.NewBinding(key.WithKeys("down", "j"))
	KeyPageUp   = key.NewBinding(key.WithKeys("pgup"))
	KeyPageDown = key.NewBinding(key.WithKeys("pgdown"))
	KeyTop      = key.NewBinding(key.WithKeys("g", "home"))
	KeyBottom   = key.NewBinding(key.WithKeys("G", "end"))

	// Search "/" prompt editing. History recall is arrow-only (not KeyUp/KeyDown)
	// so k/j are typed into the pattern as literal text instead of recalling.
	KeyBackspace = key.NewBinding(key.WithKeys("backspace"))
	KeyHistPrev  = key.NewBinding(key.WithKeys("up"))
	KeyHistNext  = key.NewBinding(key.WithKeys("down"))

	// Pager committed-search navigation.
	KeySearchNext = key.NewBinding(key.WithKeys("n"))
	KeySearchPrev = key.NewBinding(key.WithKeys("N"))

	// Run picker: jump the status filter back to "all statuses". The forward
	// cycle reuses BindStatus below (it is also a footer entry).
	KeyStatusClear = key.NewBinding(key.WithKeys("S"))
)

// Footer bindings that also serve as dispatch bindings: single-key actions whose
// help label matches what they do, so one binding covers both roles.
var (
	BindSelect  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	BindSearch  = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search"))
	BindRefresh = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	BindStatus  = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "status"))
	BindHelp    = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help"))
	BindQuit    = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
)

// Footer-only display bindings. The two esc variants exist because esc reads as
// "back" in pickers/pagers that pop a level and "quit" in the ones that exit;
// the grouped movement bindings reuse the dispatch keystrokes above so the key
// strings stay defined in exactly one place.
var (
	BindBack      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	BindQuitEsc   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit"))
	BindMove      = grouped("↑/↓", "move", KeyUp, KeyDown)
	BindScroll    = grouped("↑/↓", "scroll", KeyUp, KeyDown)
	BindTopBottom = grouped("g/G", "top/bottom", KeyTop, KeyBottom)
	BindPage      = grouped("f/b", "page", KeyPageUp, KeyPageDown)
)

// grouped builds a footer display binding: the combined key/desc hint label over
// the union of the source bindings' keystrokes, so a single "↑/↓ move" entry
// covers the separate up and down dispatch bindings without redeclaring keys.
func grouped(label, desc string, src ...key.Binding) key.Binding {
	var keys []string
	for _, b := range src {
		keys = append(keys, b.Keys()...)
	}
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, desc))
}

// footerHelp returns a help.Model configured to render footer key hints in the
// CLI's house style: every segment muted (theme.ColorMuted) and joined with
// " · ". Width is left at zero so lines are never truncated.
func footerHelp() help.Model {
	h := help.New()
	h.ShortSeparator = " · "
	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	h.Styles.ShortKey = muted
	h.Styles.ShortDesc = muted
	h.Styles.ShortSeparator = muted
	h.Styles.Ellipsis = muted
	return h
}

// Hints renders a one-line, muted footer for the given key bindings using the
// shared house style. Use it for standalone footers a host draws itself (a
// theme picker, a token prompt); components that own their footer store the
// bindings and render them internally.
func Hints(bindings ...key.Binding) string {
	return footerHelp().ShortHelpView(bindings)
}
