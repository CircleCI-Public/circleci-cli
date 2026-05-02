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

package iostream

import (
	"bytes"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestNoQueryEnviron_SuppressesTerminalQueries guards against regressions in
// the environment manipulation that prevents bubbletea from sending DECRQM
// terminal capability queries (modes 2026/2027) and Kitty keyboard queries
// whose responses land as garbage on the shell prompt when stdin is in
// non-reading mode (tea.WithInput(nil)).
func TestNoQueryEnviron_SuppressesTerminalQueries(t *testing.T) {
	t.Run("appends SSH_TTY to suppress mode-2026/2027 queries", func(t *testing.T) {
		// bubbletea's shouldQuerySynchronizedOutput fires unless SSH_TTY is set.
		// Without it, terminals receive "\x1b[?2026$p\x1b[?2027$p" and their
		// responses (e.g. "2026;2$y") end up on the shell prompt.
		env := noQueryEnviron()
		assert.Check(t, is.Contains(env, "SSH_TTY=1"),
			"SSH_TTY=1 must be present to suppress bubbletea mode-2026/2027 queries")
	})

	t.Run("SSH_TTY is always the last entry", func(t *testing.T) {
		env := noQueryEnviron()
		assert.Check(t, is.Equal(env[len(env)-1], "SSH_TTY=1"))
	})

	t.Run("normalizes ghostty TERM to suppress Kitty query path", func(t *testing.T) {
		// bubbletea also queries when TERM contains "ghostty". It sends
		// RequestKittyKeyboard ("\x1b[?u"); Ghostty responds with "\x1b[?1u"
		// which appears as "1u" garbage on the prompt.
		t.Setenv("TERM", "xterm-ghostty")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("normalizes wezterm TERM", func(t *testing.T) {
		t.Setenv("TERM", "wezterm")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("normalizes alacritty TERM", func(t *testing.T) {
		t.Setenv("TERM", "alacritty")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("normalizes kitty TERM", func(t *testing.T) {
		t.Setenv("TERM", "xterm-kitty")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("normalizes rio TERM", func(t *testing.T) {
		t.Setenv("TERM", "rio")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("preserves non-trigger TERM unchanged", func(t *testing.T) {
		t.Setenv("TERM", "xterm-256color")
		assertTERMNormalized(t, "TERM=xterm-256color")
	})

	t.Run("preserves plain xterm TERM", func(t *testing.T) {
		t.Setenv("TERM", "xterm")
		assertTERMNormalized(t, "TERM=xterm")
	})

	t.Run("preserves unrelated env vars", func(t *testing.T) {
		t.Setenv("CIRCLECI_TEST_SENTINEL", "preserve-me")
		env := noQueryEnviron()
		found := false
		for _, e := range env {
			if e == "CIRCLECI_TEST_SENTINEL=preserve-me" {
				found = true
				break
			}
		}
		assert.Check(t, found, "expected CIRCLECI_TEST_SENTINEL to be preserved in env")
	})
}

// assertTERMNormalized checks that the first TERM= entry in noQueryEnviron()
// equals want.
func assertTERMNormalized(t *testing.T, want string) {
	t.Helper()
	env := noQueryEnviron()
	for _, e := range env {
		if strings.HasPrefix(e, "TERM=") {
			assert.Check(t, is.Equal(e, want))
			return
		}
	}
	t.Errorf("TERM not found in environment returned by noQueryEnviron()")
}

// TestSpinner_NoopPaths verifies that the spinner returns an inactive Spin
// (no bubbletea program, no goroutines) in all non-animating scenarios.
// This guards against the bubbletea program accidentally being started in
// contexts where stdin is unread — which is how the terminal garbage first
// appeared.
func TestSpinner_NoopPaths(t *testing.T) {
	makeStreams := func() (Streams, *bytes.Buffer) {
		errBuf := &bytes.Buffer{}
		return Streams{Out: &bytes.Buffer{}, Err: errBuf}, errBuf
	}

	t.Run("active=false returns inactive spin", func(t *testing.T) {
		s, _ := makeStreams()
		sp := s.Spinner(false, "working")
		assert.Check(t, !sp.active)
		sp.Stop() // must not panic or block
	})

	t.Run("quiet=true returns inactive spin even when active=true", func(t *testing.T) {
		s, _ := makeStreams()
		s.Quiet = true
		sp := s.Spinner(true, "working")
		assert.Check(t, !sp.active)
		sp.Stop()
	})

	t.Run("non-TTY prints static line and returns inactive spin", func(t *testing.T) {
		// Out is a bytes.Buffer (not *os.File), so IsInteractive() is false.
		// The spinner must fall back to a plain "msg...\n" on stderr.
		s, errBuf := makeStreams()
		sp := s.Spinner(true, "loading data")
		assert.Check(t, !sp.active, "expected inactive spin for non-TTY output")
		assert.Check(t, is.Contains(errBuf.String(), "loading data..."),
			"expected static progress line on stderr for non-TTY session")
		sp.Stop()
	})

	t.Run("Stop on nil Spin does not panic", func(t *testing.T) {
		var sp *Spin
		sp.Stop()
	})

	t.Run("Stop called twice does not panic", func(t *testing.T) {
		s, _ := makeStreams()
		sp := s.Spinner(false, "x")
		sp.Stop()
		sp.Stop()
	})
}
