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

//go:build !windows

// These tests need a real terminal-backed file descriptor (the regression only
// manifests when term.IsTerminal reports true). The expect console exposes
// Tty() only on Unix — Windows uses ConPTY and has no equivalent — so this file
// is constrained to non-Windows builds, matching the acceptance harness which
// skips TTY snapshots on Windows.
package iostream

import (
	"os"
	"testing"
	"time"

	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/term"
	"github.com/pete-woods/go-expect"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// newTTY returns a real terminal-backed *os.File using the same expect/pty
// machinery the acceptance tests use (see internal/testing/binary).
func newTTY(t *testing.T) *os.File {
	t.Helper()

	c, err := expect.NewConsole()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	tty := c.Tty()
	assert.Assert(t, term.IsTerminal(tty.Fd()), "expected the expect console Tty to be a terminal")
	return tty
}

// TestBackgroundQueryable is the core regression guard: the auto-theme path
// must only probe the terminal background (an OSC 11 query that blocks until a
// 2s-per-stream timeout when unanswered) for a real, interactive terminal.
func TestBackgroundQueryable(t *testing.T) {
	tty := newTTY(t)

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "interactive terminal", env: nil, want: true},
		{name: "CI set", env: map[string]string{"CI": "true"}, want: false},
		{name: "CIRCLE_NO_INTERACTIVE set", env: map[string]string{"CIRCLE_NO_INTERACTIVE": "1"}, want: false},
		{name: "NO_COLOR set", env: map[string]string{"NO_COLOR": "1"}, want: false},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearTerminalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			assert.Check(t, cmp.Equal(backgroundQueryable(tty, tty), tc.want))
		})
	}
}

// TestTerminalPropertiesAutoNoQueryInCI asserts the end-to-end behavior: with a
// real PTY but CI set, resolving the "auto" theme must return promptly instead
// of blocking on the (never-answered) background query. The pre-fix code took
// ~4s here (two 2s timeouts); the guard makes it instant.
func TestTerminalPropertiesAutoNoQueryInCI(t *testing.T) {
	clearTerminalEnv(t)
	t.Setenv("CI", "true")

	tty := newTTY(t)

	done := make(chan string, 1)
	go func() {
		_, style := terminalProperties(themeAuto, tty, tty)
		done <- style
	}()

	select {
	case style := <-done:
		// The skip path defaults to the dark style, matching lipgloss's own
		// on-error default.
		assert.Check(t, cmp.Equal(style, styles.DarkStyle))
	case <-time.After(1 * time.Second):
		t.Fatal("terminalProperties blocked on a terminal background query in CI; the backgroundQueryable guard has regressed")
	}
}
