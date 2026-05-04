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

//go:build linux

package iostream

import "golang.org/x/sys/unix"

// drainStdin discards bytes pending in stdin's line discipline buffer.
//
// Bubbletea v2 writes capability queries (mode 2026 synchronized output, mode
// 2027 unicode core) to the output at startup. The terminal responds on stdin.
// Because the spinner uses WithInput(nil) the input loop never runs, so those
// responses accumulate in the TTY line discipline buffer. A plain read() cannot
// drain them in canonical mode (no newline, so the line discipline won't
// deliver the data). TCFLSH/TCIFLUSH discards the buffer directly.
//
// We loop — flush then poll — to handle SSH round-trip latency where responses
// may arrive after the first flush. On local terminals the poll returns
// immediately with no data. See: charmbracelet/bubbletea#1590.
func drainStdin() {
	fds := []unix.PollFd{{Fd: 0, Events: unix.POLLIN}} // stdin is always fd 0
	for {
		_ = unix.IoctlSetInt(0, unix.TCFLSH, 0) // 0 = TCIFLUSH: discard received, unread data
		n, _ := unix.Poll(fds, 200)
		if n <= 0 {
			return
		}
	}
}
