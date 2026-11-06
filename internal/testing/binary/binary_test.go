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

package binary

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestCollapsePTYCarriageReturns(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single CRLF untouched",
			in:   "a\r\nb\r\n",
			want: "a\r\nb\r\n",
		},
		{
			name: "doubled CR collapsed",
			in:   "a\r\r\nb\r\n",
			want: "a\r\nb\r\n",
		},
		{
			name: "tripled CR collapsed",
			in:   "a\r\r\r\n",
			want: "a\r\n",
		},
		{
			name: "lone CR redraw untouched",
			in:   "50%\r100%\r\n",
			want: "50%\r100%\r\n",
		},
		{
			name: "bare LF untouched",
			in:   "a\nb",
			want: "a\nb",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(collapsePTYCarriageReturns(tc.in), tc.want))
		})
	}
}
