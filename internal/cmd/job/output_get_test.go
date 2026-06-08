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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			renderTerminal(&out, []byte(tt.in))
			assert.Check(t, cmp.Equal(out.String(), tt.want))
		})
	}
}
