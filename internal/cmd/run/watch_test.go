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

package run

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestIsHexSHA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"full lowercase SHA", "1234567890abcdef1234567890abcdef12345678", true},
		{"abbreviated SHA", "abc1234", true},
		{"uppercase hex", "ABC1234", true},
		{"mixed case hex", "AbC1234", true},
		{"single character", "a", true},
		// Rejected so gitremote never sees a revision expression it would
		// happily resolve to the wrong commit.
		{"branch name", "main", false},
		{"HEAD", "HEAD", false},
		{"HEAD with offset", "HEAD~3", false},
		{"tag-like name", "v1.2.3", false},
		{"non-hex letter past f", "abcg123", false},
		{"leading whitespace", " abc1234", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Check(t, cmp.Equal(isHexSHA(tt.in), tt.want), "input: %q", tt.in)
		})
	}
}

// A full SHA must survive validation unchanged in either case, since the
// scripted path passes whatever git printed.
func TestIsHexSHA_FullSHACaseInsensitive(t *testing.T) {
	t.Parallel()

	full := "1234567890abcdef1234567890abcdef12345678"
	assert.Check(t, isHexSHA(full))
	assert.Check(t, isHexSHA(strings.ToUpper(full)))
}
