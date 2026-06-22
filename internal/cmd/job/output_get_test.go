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

package job

import (
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func Test_renderTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "plain lines unchanged",
			in:   "one\ntwo\n",
			want: "one\ntwo\n",
		},
		{
			name: "missing trailing newline is added",
			in:   "abc",
			want: "abc\n",
		},
		{
			name: "ansi color is stripped",
			in:   "\x1b[32mgreen\x1b[0m\n",
			want: "green\n",
		},
		{
			name: "carriage-return progress collapses to final state",
			in:   "layer: Downloading [==>]\r\x1b[Klayer: Downloading [====>]\r\x1b[Klayer: Download complete\n",
			want: "layer: Download complete\n",
		},
		{
			name: "cursor up and erase redraws the targeted line",
			in:   "a: pending\nb: pending\n\x1b[1A\x1b[2K\ra: done\n",
			want: "a: pending\na: done\n",
		},
		{
			// Device Attributes / OSC color / DECRQM queries expect a reply from
			// the terminal. The emulator generates one and writes it to its input
			// pipe; we must not block on that (see the query-drain regression
			// test below) and the query bytes must not leak into the output.
			name: "terminal queries are consumed, surrounding text preserved",
			in:   "before\x1b]11;?\x07\x1b[c\x1b[?2026$pafter\n",
			want: "beforeafter\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			renderTerminal(&out, []byte(tt.in))
			assert.Check(t, cmp.Equal(out.String(), tt.want))
		})
	}
}

// Regression: captured output that contains a terminal query (e.g. goreleaser
// emits an OSC 11 background-color query and a Device Attributes request) used
// to hang renderTerminal forever. The emulator replies on an unbuffered pipe,
// so an undrained reply blocked the write — and the command could not be
// interrupted. Guard with a timeout so a regression fails fast instead of
// hanging the whole suite.
func Test_renderTerminal_terminalQueriesDoNotBlock(t *testing.T) {
	// OSC 11 background-color query, Device Attributes, and a DECRQM mode query
	// — all reply-eliciting — surrounded by real content.
	in := []byte("building\x1b]11;?\x07 packages\x1b[c done\x1b[?2026$p\n")

	done := make(chan string, 1)
	go func() {
		var out strings.Builder
		renderTerminal(&out, in)
		done <- out.String()
	}()

	select {
	case got := <-done:
		assert.Check(t, cmp.Equal(got, "building packages done\n"))
	case <-time.After(10 * time.Second):
		t.Fatal("renderTerminal did not return: a terminal query reply blocked the emulator's pipe")
	}
}
