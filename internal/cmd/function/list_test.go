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

package cmdfunction

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestSummarize(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{
			name:  "first line only",
			given: "Install a Go toolchain.\n\nExample:\n  circleci run setup-go",
			want:  "Install a Go toolchain.",
		},
		{
			name:  "pipe escaped so the cell does not split the row",
			given: "Install browsers | drivers.",
			want:  `Install browsers \| drivers.`,
		},
		{
			name:  "capped with an ellipsis",
			given: strings.Repeat("a", descriptionMax+10),
			want:  strings.Repeat("a", descriptionMax) + "…",
		},
		{
			name:  "empty in, empty out",
			given: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// escape=false: the no-color path, where summarize's own job (first
			// line, pipe, truncation) is all that runs. Escaping is covered by
			// TestEscapeMarkdown.
			assert.Check(t, cmp.Equal(summarize(tt.given, false), tt.want))
		})
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{
			name:  "placeholder kept rather than read as an HTML tag",
			given: "Run with <name>.",
			want:  `Run with \<name\>.`,
		},
		{
			name:  "snake_case not italicized",
			given: "Set my_var_name or *.go",
			want:  `Set my\_var\_name or \*.go`,
		},
		{
			name:  "links, code spans and backslashes escaped so they print literally",
			given: "[a](b) `code` C:\\dir",
			want:  "\\[a\\](b) \\`code\\` C:\\\\dir",
		},
		{
			name:  "plain text unchanged",
			given: "Install a Go toolchain (1.22).",
			want:  "Install a Go toolchain (1.22).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(escapeMarkdown(tt.given), tt.want))
		})
	}
}
