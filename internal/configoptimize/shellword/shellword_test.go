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

package shellword

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"mvdan.cc/sh/v3/syntax"
)

func TestLiteral(t *testing.T) {
	tests := []struct {
		src  string
		want string
		ok   bool
	}{
		{src: `npm`, want: "npm", ok: true},
		{src: `'a b'`, want: "a b", ok: true},
		{src: `"a"b'c'`, want: "abc", ok: true},
		{src: `$HOME`, ok: false},
		{src: `"$HOME/x"`, ok: false},
		{src: `$(id)`, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			f, err := syntax.NewParser().Parse(strings.NewReader(tt.src), "")
			assert.NilError(t, err)
			call := f.Stmts[0].Cmd.(*syntax.CallExpr)
			got, ok := Literal(call.Args[0])
			assert.Check(t, cmp.Equal(ok, tt.ok))
			assert.Check(t, cmp.Equal(got, tt.want))
		})
	}
}
