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

package configaudit

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "empty",
			value: "",
			want:  "…",
		},
		{
			name:  "four characters",
			value: "abcd",
			want:  "…",
		},
		{
			name:  "eleven characters",
			value: "abcdefghijk",
			want:  "…",
		},
		{
			name:  "twelve characters",
			value: "abcdefghijkl",
			want:  "abcd…",
		},
		{
			name:  "forty characters",
			value: "ghp_" + strings.Repeat("x", 36),
			want:  "ghp_…",
		},
		{
			name:  "multi-byte characters",
			value: "ключключключ",
			want:  "ключ…",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(redact(tc.value), tc.want))
		})
	}
}

func TestRedactLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		rs   []redaction
		want string
	}{
		{
			name: "one span",
			line: "  KEY: abcdefghijklmnop",
			rs:   []redaction{{line: 1, col: 8, endCol: 24, with: "abcd…"}},
			want: "  KEY: abcd…",
		},
		{
			name: "two spans on one line",
			line: "{A: abcdefghijkl, B: mnopqrstuvwx}",
			rs: []redaction{
				{line: 1, col: 5, endCol: 17, with: "abcd…"},
				{line: 1, col: 22, endCol: 34, with: "mnop…"},
			},
			want: "{A: abcd…, B: mnop…}",
		},
		{
			name: "span past the end of the line",
			line: "K: abc",
			rs:   []redaction{{line: 1, col: 4, endCol: 40, with: "…"}},
			want: "K: …",
		},
		{
			name: "multi-byte text before the span",
			line: "ключ: secretvalue",
			rs:   []redaction{{line: 1, col: 7, endCol: 18, with: "…"}},
			want: "ключ: …",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(redactLine(tc.line, tc.rs), tc.want))
		})
	}
}
