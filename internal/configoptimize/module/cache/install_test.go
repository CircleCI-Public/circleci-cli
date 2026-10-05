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

package cache

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// env -S is split the way GNU env does; input it would reject gives no
// words, so no command is claimed from it.
func TestSplitString(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    []string
		refused bool // env refuses it
	}{
		{name: "npm ci", text: "npm ci", want: []string{"npm", "ci"}},
		{name: `"npm" "ci"`, text: `"npm" "ci"`, want: []string{"npm", "ci"}},
		{name: "npm ci; --version", text: "npm ci; --version", want: []string{"npm", "ci;", "--version"}},
		{name: `npm\_ci`, text: `npm\_ci`, want: []string{"npm", "ci"}},
		{name: `a\tb`, text: `a\tb`, want: []string{"a\tb"}},
		{name: `'a b' c`, text: `'a b' c`, want: []string{"a b", "c"}},
		{name: "an unknown escape", text: `n\pm ci`, refused: true},
		{name: "a trailing backslash", text: `npm ci \`, refused: true},
		{name: "an unclosed quote", text: `'npm ci`, refused: true},
		{name: "the empty string", text: "", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := splitString(tc.text)
			if tc.refused {
				assert.Check(t, !ok, "%q should be refused", tc.text)
				return
			}
			assert.Check(t, ok, "%q", tc.text)
			assert.Check(t, cmp.DeepEqual(got, tc.want), "%q", tc.text)
		})
	}
}

func TestEnvEmptySplitString(t *testing.T) {
	assert.Check(t, cmp.Equal(installOf(stripWrappers([]string{"env", "-S", "", "npm", "ci"})), "npm ci"))
	// env refuses an invalid split string, so the command after it never runs.
	assert.Check(t, cmp.Equal(installOf(stripWrappers([]string{"env", "-S", `bad\q`, "npm", "ci"})), ""))
}
