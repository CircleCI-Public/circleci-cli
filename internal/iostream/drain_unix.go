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

package iostream

import (
	"os"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
	xterm "golang.org/x/term"
)

// drainStdinBuffer discards any terminal query responses that bubbletea left
// unread in stdin.
//
// Even with noQueryEnviron() preventing the mode-2026/2027 queries, the
// renderer always sends ansi.RequestKittyKeyboard ("\x1b[?u") on its first
// frame to probe Kitty keyboard support. Terminals like Ghostty respond with
// "\x1b[?1u". Because the spinner uses tea.WithInput(nil) (input disabled),
// bubbletea never reads that response; it sits in stdin and the shell reads it
// as typed input — the "\x1b[?" prefix is swallowed by the terminal line
// editor and the trailing "1u" appears as garbage on the prompt.
//
// The critical subtlety: tea.WithInput(nil) leaves stdin in canonical mode
// (ICANON). In canonical mode, select(2) on a TTY returns readable only when
// a complete line (ending in \n) is buffered. The Kitty keyboard reply has no
// newline, so select blocks until the 100 ms timeout — the bytes are never
// drained. We must switch to raw mode first so select sees any buffered bytes.
func drainStdinBuffer() {
	rawFd := os.Stdin.Fd()
	if !term.IsTerminal(rawFd) {
		return
	}
	fd := int(rawFd) //nolint:gosec // fd value is always small; overflow impossible

	// Switch to raw mode so select(2) can see non-newline-terminated responses.
	// Restore the original state on return regardless of outcome.
	state, err := xterm.MakeRaw(fd)
	if err != nil {
		return
	}
	defer xterm.Restore(fd, state) //nolint:errcheck

	// Wait up to 100 ms for data. For Ghostty the response is already buffered
	// so Select returns immediately; the timeout handles terminals that don't
	// respond to the Kitty keyboard query.
	var rfds unix.FdSet
	rfds.Set(fd)
	tv := unix.NsecToTimeval(int64(100 * time.Millisecond))
	n, _ := unix.Select(fd+1, &rfds, nil, nil, &tv)
	if n <= 0 {
		return
	}

	// Data is available; drain it all non-blocking so we never block on
	// user input that arrives concurrently.
	if err := syscall.SetNonblock(fd, true); err != nil {
		return
	}
	defer syscall.SetNonblock(fd, false) //nolint:errcheck
	buf := make([]byte, 256)
	for {
		r, err := syscall.Read(fd, buf)
		if r <= 0 || err != nil {
			return
		}
	}
}
