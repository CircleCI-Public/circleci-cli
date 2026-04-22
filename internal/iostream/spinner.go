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
	"sync"
	"time"
)

// Spinner is a progress indicator. Call Stop when the operation completes.
// It is safe to call Stop on a nil or no-op Spinner, and safe to call it
// more than once.
type Spinner struct {
	done   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
	active bool // true when the animation goroutine is running
}

// Spinner creates and starts a progress indicator for msg.
//
// Pass active=false (e.g. !jsonOut) to get a no-op Spinner with no output.
// When quiet mode is on, the Spinner is also a no-op.
// In a non-interactive session (no TTY, CI=true, spinner disabled) a plain
// "msg...\n" line is written to stderr instead of animating.
//
// Always call Stop() when the operation completes.
func (s Streams) Spinner(active bool, msg string) *Spinner {
	if !active || s.Quiet {
		return &Spinner{}
	}

	if !s.IsInteractive() || spinnerDisabled() {
		// Non-TTY or explicitly disabled: static one-liner, no animation.
		_, _ = fmt.Fprintf(s.Err, "%s...\n", msg)
		return &Spinner{}
	}

	sp := &Spinner{
		done:   make(chan struct{}),
		active: true,
	}

	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		var frames []string
		if s.ColorEnabled() {
			frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		} else {
			frames = []string{"|", "/", "-", "\\"}
		}
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		i := 0
		for {
			select {
			case <-sp.done:
				_, _ = fmt.Fprintf(s.Err, "\r\033[K") // erase spinner line
				return
			case <-tick.C:
				_, _ = fmt.Fprintf(s.Err, "\r%s %s", frames[i%len(frames)], msg)
				i++
			}
		}
	}()

	return sp
}

// Stop halts the spinner and clears its line. It is safe to call on a nil or
// no-op Spinner and safe to call more than once.
func (sp *Spinner) Stop() {
	if sp == nil || !sp.active {
		return
	}
	sp.once.Do(func() {
		close(sp.done)
		sp.wg.Wait()
	})
}

// spinnerDisabled reports whether animation should be suppressed even in a TTY.
func spinnerDisabled() bool {
	return os.Getenv("CIRCLECI_SPINNER_DISABLED") != ""
}
