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
	"fmt"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui"
)

// Spin is a progress indicator. Call Stop when the operation completes.
// It is safe to call Stop on a nil or no-op Spin, and safe to call it
// more than once.
type Spin struct {
	program *tea.Program
	once    sync.Once
	active  bool
}

// Spinner creates and starts a progress indicator for msg.
//
// Pass active=false (e.g. !jsonOut) to get a no-op Spin with no output.
// When quiet mode is on, the Spin is also a no-op.
// In a non-interactive session (no TTY, CI=true, spinner disabled) a plain
// "msg...\n" line is written to stderr instead of animating.
//
// Always call Stop() when the operation completes.
func (s Streams) Spinner(active bool, msg string) *Spin {
	if !active || s.Quiet {
		return &Spin{}
	}

	if !s.IsInteractive() || spinnerDisabled() {
		// Non-TTY or explicitly disabled: static one-liner, no animation.
		_, _ = fmt.Fprintf(s.Err, "%s...\n", msg)
		return &Spin{}
	}

	p := tea.NewProgram(
		ui.NewSpinnerModel(msg, s.ColorEnabled()),
		tea.WithOutput(s.Err),
		tea.WithInput(nil),           // keep stdin out of raw mode so Ctrl+C still generates SIGINT
		tea.WithEnvironment(noQueryEnviron()), // prevent terminal capability queries (see below)
	)

	sp := &Spin{program: p, active: true}
	go func() {
		_, _ = p.Run()
	}()
	return sp
}

// Stop halts the spinner and clears its line. It is safe to call on a nil or
// no-op Spin and safe to call more than once.
func (sp *Spin) Stop() {
	if sp == nil || !sp.active {
		return
	}
	sp.once.Do(func() {
		sp.program.Quit()
		sp.program.Wait()
		drainStdinBuffer()
	})
}

// spinnerDisabled reports whether animation should be suppressed even in a TTY.
func spinnerDisabled() bool {
	return os.Getenv("CIRCLECI_SPINNER_DISABLED") != ""
}

// noQueryEnviron returns a copy of the process environment adjusted to prevent
// bubbletea from sending DECRQM terminal capability queries (modes 2026 and
// 2027). When input is disabled via tea.WithInput(nil), bubbletea cannot
// consume the terminal's responses; those bytes land in stdin and appear as
// garbage on the shell prompt after the program exits.
//
// bubbletea's shouldQuerySynchronizedOutput function has several trigger
// conditions we need to suppress:
//
//  1. (no TERM_PROGRAM && no SSH_TTY) — fires on plain xterm-like sessions.
//     Defeated by injecting SSH_TTY=1.
//
//  2. (TERM_PROGRAM set && not "Apple" && no SSH_TTY) — fires for Ghostty,
//     WezTerm, etc. that set TERM_PROGRAM. Defeated by SSH_TTY=1.
//
//  3. TERM contains "ghostty", "wezterm", "alacritty", "kitty", or "rio" —
//     fires regardless of SSH_TTY. Defeated by replacing the matching TERM
//     value with "xterm-256color".
//
// SSH_TTY=1 suppresses conditions 1 and 2 in one shot. The TERM substitution
// handles condition 3.  Our theme uses 256-color codes, so downgrading the
// color profile from TrueColor to 256-color has no visible effect on the
// spinner.
func noQueryEnviron() []string {
	triggerTerms := []string{"ghostty", "wezterm", "alacritty", "kitty", "rio"}
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TERM=") {
			val := strings.TrimPrefix(e, "TERM=")
			for _, name := range triggerTerms {
				if strings.Contains(val, name) {
					val = "xterm-256color"
					break
				}
			}
			e = "TERM=" + val
		}
		env = append(env, e)
	}
	env = append(env, "SSH_TTY=1")
	return env
}
