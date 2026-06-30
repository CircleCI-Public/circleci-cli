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
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestTailLines(t *testing.T) {
	t.Run("fewer than max returned unchanged", func(t *testing.T) {
		in := "a\nb\nc\n"
		tail, omitted := tailLines(in, 200)
		assert.Check(t, cmp.Equal(tail, in))
		assert.Check(t, cmp.Equal(omitted, 0))
	})

	t.Run("exactly max returned unchanged", func(t *testing.T) {
		in := "a\nb\nc\n"
		tail, omitted := tailLines(in, 3)
		assert.Check(t, cmp.Equal(tail, in))
		assert.Check(t, cmp.Equal(omitted, 0))
	})

	t.Run("more than max keeps the tail", func(t *testing.T) {
		in := "a\nb\nc\nd\ne\n"
		tail, omitted := tailLines(in, 2)
		assert.Check(t, cmp.Equal(tail, "d\ne\n"))
		assert.Check(t, cmp.Equal(omitted, 3))
	})

	t.Run("large input keeps exactly max lines", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&b, "line %d\n", i)
		}
		tail, omitted := tailLines(b.String(), 200)
		assert.Check(t, cmp.Equal(omitted, 800))
		assert.Check(t, cmp.Equal(strings.Count(tail, "\n"), 200))
		assert.Check(t, cmp.Equal(strings.HasPrefix(tail, "line 800\n"), true))
		assert.Check(t, cmp.Equal(strings.HasSuffix(tail, "line 999\n"), true))
	})
}
