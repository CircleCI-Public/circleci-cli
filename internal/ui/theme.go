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

import "charm.land/lipgloss/v2"

// Semantic color tokens. Use these instead of raw lipgloss.Color values so
// every widget speaks the same visual language, and a future palette swap
// only has to touch this file.
//
// 256-color codes are used (not hex) so terminals with reduced color depth
// degrade gracefully instead of falling back to white.
var (
	ColorAccent  = lipgloss.Color("205") // pink — in-progress state, focused input
	ColorSuccess = lipgloss.Color("42")  // green — completed actions
	ColorError   = lipgloss.Color("196") // red — failures
	ColorWarning = lipgloss.Color("220") // yellow — warnings, "press Y to confirm"
	ColorMuted   = lipgloss.Color("244") // gray — footer hints, less important text
)

// Style presets that combine the colors above with shared layout choices
// (bold/margins/etc). Widgets should reach for these before declaring a
// local style; only fork into a one-off style when the preset genuinely
// doesn't fit.
var (
	// TitleStyle is for prompt headings and emphasized labels. Bold, no color
	// so it works across light and dark terminals.
	TitleStyle = lipgloss.NewStyle().Bold(true)

	// AccentStyle is for active or in-progress UI: the spinner glyph, the
	// focused-input cursor accent, anything the user's eye should track.
	AccentStyle = lipgloss.NewStyle().Foreground(ColorAccent)

	// HelperStyle is for de-emphasized hint text — footers like
	// "(esc to quit)", inline shortcuts, secondary instructions.
	HelperStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// SuccessStyle / ErrorStyle / WarningStyle are for status glyphs and
	// short status words. Pair them with the icons below.
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ColorError)
	WarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
)

// Status icons. Keep these in one place so prompts, results, and JSON-text
// fallbacks all reach for the same glyph.
const (
	IconOK   = "✓"
	IconWarn = "⚠"
	IconFail = "✗"
)
