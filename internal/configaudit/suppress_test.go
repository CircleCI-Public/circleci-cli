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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestParseSuppressions(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		wantIgnored  map[int][]string
		wantWarnings []string
	}{
		{
			name:        "comment above applies to the next line",
			lines:       []string{"# circleci-audit: ignore[unpinned-orb]", "node: circleci/node@5"},
			wantIgnored: map[int][]string{2: {"unpinned-orb"}},
		},
		{
			name:        "comment at the end of a line applies to that line",
			lines:       []string{"node: circleci/node@5 # circleci-audit: ignore[unpinned-orb]"},
			wantIgnored: map[int][]string{1: {"unpinned-orb"}},
		},
		{
			name:        "indented comment alone on a line",
			lines:       []string{"  # circleci-audit: ignore[unpinned-orb]", "  node: circleci/node@5"},
			wantIgnored: map[int][]string{2: {"unpinned-orb"}},
		},
		{
			name:        "list of ids",
			lines:       []string{"x # circleci-audit: ignore[unpinned-orb,unpinned-image]"},
			wantIgnored: map[int][]string{1: {"unpinned-orb", "unpinned-image"}},
		},
		{
			name:        "spaces around ids and the colon",
			lines:       []string{"x #   circleci-audit:   ignore[ unpinned-orb , unpinned-image ]"},
			wantIgnored: map[int][]string{1: {"unpinned-orb", "unpinned-image"}},
		},
		{
			name:        "no space after the hash",
			lines:       []string{"x #circleci-audit: ignore[unpinned-orb]"},
			wantIgnored: map[int][]string{1: {"unpinned-orb"}},
		},
		{
			name:        "unknown id warns and the known one still applies",
			lines:       []string{"ok", "x # circleci-audit: ignore[no-such-rule, unpinned-orb]"},
			wantIgnored: map[int][]string{2: {"unpinned-orb"}},
			wantWarnings: []string{
				`unknown rule "no-such-rule" in ignore comment on line 2`,
			},
		},
		{
			name:        "only an unknown id",
			lines:       []string{"# circleci-audit: ignore[nope]", "x"},
			wantIgnored: map[int][]string{},
			wantWarnings: []string{
				`unknown rule "nope" in ignore comment on line 1`,
			},
		},
		{
			name:        "not an ignore comment",
			lines:       []string{"# circleci-audit: something-else", "# ignore[unpinned-orb]"},
			wantIgnored: map[int][]string{},
		},
		{
			name: "comment two lines above does not apply to the line",
			lines: []string{
				"# circleci-audit: ignore[unpinned-orb]",
				"other: value",
				"node: circleci/node@5",
			},
			wantIgnored: map[int][]string{2: {"unpinned-orb"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotIgnored, gotWarnings := parseSuppressions(tc.lines)
			assert.Check(t, cmp.DeepEqual(gotIgnored, tc.wantIgnored))
			assert.Check(t, cmp.DeepEqual(gotWarnings, append([]string{}, tc.wantWarnings...)))
		})
	}
}
