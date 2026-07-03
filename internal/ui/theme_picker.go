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
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// previewGap is the number of blank columns between the selector and the
// preview pane. minPreviewWidth keeps the preview usable on narrow terminals.
const (
	previewGap      = 3
	minPreviewWidth = 24
)

// ThemePickerModel is a top-level bubbletea program that splits the screen
// horizontally: the left pane is the standard select list of theme names, and
// the right pane shows sample markdown rendered in the currently-highlighted
// theme. Moving the cursor re-renders the preview, so the user sees each theme
// applied live before committing. Enter confirms; Esc/Ctrl+C cancels.
//
// The markdown is produced by a render callback so the program stays decoupled
// from glamour: the caller (iostream) supplies a closure that renders the sample
// in a given theme, wrapped to a given width.
type ThemePickerModel struct {
	selector components.SelectModel
	preview  components.PreviewModel

	// themes holds the raw theme names parallel to the selector's display
	// labels, so the highlighted index maps back to a name to render.
	themes []string

	// render returns the sample markdown rendered in theme, word-wrapped to
	// width columns. It is run off the Update loop (in a tea.Cmd) because glamour
	// rendering is slow enough to make per-keystroke navigation feel laggy.
	render func(theme string, width int) string

	// cache memoizes rendered previews keyed by theme+width so revisiting a theme
	// (or holding an arrow key) is instant and never re-renders. Shared by
	// reference across the value-copied model, so async results warm it for every
	// future copy.
	cache map[string]string

	// spinner animates the "Loading" placeholder shown while a cache-miss render
	// is in flight; loading tracks whether that placeholder is currently showing.
	// animate is false when CIRCLE_SPINNER_DISABLED (or a non-TTY) suppresses
	// animation: the placeholder is then static, avoiding continuous repaints
	// that would scramble scripted/PTY-driven sessions.
	spinner spinner.Model
	loading bool
	animate bool

	width     int
	height    int
	ready     bool
	cancelled bool
}

// previewRenderedMsg carries the result of an async preview render back into the
// Update loop. key identifies the (theme, width) it was rendered for so a stale
// result from rapid navigation is cached but only applied when still current.
type previewRenderedMsg struct {
	key     string
	content string
}

func previewKey(theme string, width int) string {
	return fmt.Sprintf("%s@%d", theme, width)
}

// NewThemePickerModel builds a theme picker. labels are the display strings for
// the list (e.g. with a "(default)" marker); themes are the matching raw theme
// names, same length and order. render produces the preview markdown for a theme
// at a given width.
// animate reports whether the loading spinner should be animated. Pass false
// when CIRCLE_SPINNER_DISABLED is set or output is non-interactive, so the
// placeholder stays static.
func NewThemePickerModel(prompt string, labels, themes []string, render func(theme string, width int) string, color, animate bool) ThemePickerModel {
	return ThemePickerModel{
		// The hint is cleared here and shown as a full-width footer below the
		// split so a long help line doesn't widen the left column at the
		// preview's expense.
		selector: components.NewSelectModel(prompt, labels).WithKeys(),
		preview:  components.NewPreviewModel(),
		themes:   themes,
		render:   render,
		cache:    make(map[string]string),
		spinner:  components.NewSpinner(color),
		animate:  animate,
	}
}

// WithCursor returns a copy with the initial highlighted option at index i. Use
// this to start on the current theme.
func (m ThemePickerModel) WithCursor(i int) ThemePickerModel {
	m.selector = m.selector.WithCursor(i)
	return m
}

// Selected returns the index of the chosen theme. Only valid when !Cancelled().
func (m ThemePickerModel) Selected() int { return m.selector.Selected() }

// Cancelled reports whether the user quit without selecting.
func (m ThemePickerModel) Cancelled() bool { return m.cancelled }

// Init starts the spinner ticking so the loading placeholder animates. The
// spinner self-schedules its next tick, so a single Tick here keeps it running
// for the lifetime of the program. When animation is disabled it stays still,
// so scripted or non-TTY sessions see a stable screen.
func (m ThemePickerModel) Init() tea.Cmd {
	if !m.animate {
		return nil
	}
	return m.spinner.Tick
}

func (m ThemePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, m.updatePreview()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Re-render the placeholder with the advanced spinner frame.
		if m.loading {
			m.preview = m.preview.WithContent(m.loadingView())
		}
		return m, cmd

	case previewRenderedMsg:
		// Cache every result (even stale ones, to warm the cache), but only swap
		// it into view when it still matches the highlighted theme — otherwise
		// rapid navigation could flash an out-of-date preview.
		m.cache[msg.key] = msg.content
		if msg.key == m.currentKey() {
			m.loading = false
			m.preview = m.preview.WithContent(msg.content)
		}
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, components.KeyEsc, components.KeyCtrlC):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, components.KeyEnter):
			updated, _ := m.selector.Update(msg)
			m.selector = updated.(components.SelectModel)
			return m, tea.Quit
		}
		// Forward navigation to the list, then refresh the preview for the
		// newly-highlighted theme. SelectModel never emits a command, so the
		// preview command is the only one to return.
		updated, _ := m.selector.Update(msg)
		m.selector = updated.(components.SelectModel)
		return m, m.updatePreview()
	}
	return m, nil
}

// currentKey is the cache key for the highlighted theme at the current preview
// width, or "" before a size is known.
func (m ThemePickerModel) currentKey() string {
	idx := m.selector.Selected()
	if !m.ready || idx < 0 || idx >= len(m.themes) {
		return ""
	}
	return previewKey(m.themes[idx], m.preview.ContentWidth())
}

// updatePreview resizes the preview pane and refreshes its content for the
// highlighted theme. A cached render is applied immediately; a cache miss leaves
// the previous content in place and returns a command that renders off the
// Update loop, so navigation never blocks on glamour. Returns nil until the
// first window size arrives.
func (m *ThemePickerModel) updatePreview() tea.Cmd {
	if !m.ready {
		return nil
	}
	leftWidth := lipgloss.Width(m.selector.View().Content)
	previewWidth := m.width - leftWidth - previewGap
	if previewWidth < minPreviewWidth {
		previewWidth = minPreviewWidth
	}

	// Reserve one row for the footer hint line so the alt-screen does not scroll.
	boxHeight := m.height - 1
	if boxHeight < 1 {
		boxHeight = 1
	}
	m.preview = m.preview.WithSize(previewWidth, boxHeight)

	idx := m.selector.Selected()
	if idx < 0 || idx >= len(m.themes) {
		return nil
	}
	theme := m.themes[idx]
	width := m.preview.ContentWidth()
	key := previewKey(theme, width)

	if content, ok := m.cache[key]; ok {
		m.loading = false
		m.preview = m.preview.WithContent(content)
		return nil
	}

	// When animation is disabled (CIRCLE_SPINNER_DISABLED, non-TTY, or scripted
	// sessions) render synchronously. Async rendering and the spinner placeholder
	// exist only to keep an interactive human terminal responsive; in a captured
	// or piped session the extra repaints they trigger bloat output and can stall
	// the event loop, so we keep output deterministic and bounded instead.
	if !m.animate {
		content := m.render(theme, width)
		m.cache[key] = content
		m.loading = false
		m.preview = m.preview.WithContent(content)
		return nil
	}

	// Cache miss: show the animated loading placeholder until the async render
	// lands, rather than blank space or a stale preview.
	m.loading = true
	m.preview = m.preview.WithContent(m.loadingView())

	render := m.render
	return func() tea.Msg {
		return previewRenderedMsg{key: key, content: render(theme, width)}
	}
}

// loadingView is the placeholder shown while a preview render is in flight,
// centered in the preview pane. The animated spinner is prepended only when
// animation is enabled; otherwise just "Loading" is shown so the screen stays
// static.
func (m ThemePickerModel) loadingView() string {
	label := theme.HelperStyle.Render("Loading")
	if m.animate {
		label = m.spinner.View() + " " + label
	}
	return lipgloss.Place(
		m.preview.ContentWidth(), m.preview.ContentHeight(),
		lipgloss.Center, lipgloss.Center,
		label,
	)
}

func (m ThemePickerModel) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	left := lipgloss.NewStyle().MarginRight(previewGap).Render(m.selector.View().Content)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, m.preview.View())
	footer := components.Hints(
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "preview")),
		components.BindSelect,
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	)

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, footer))
	v.AltScreen = true
	return v
}
