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

package jsoncolor_test

import (
	"bytes"
	"io"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/jsoncolor"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name         string
		r            io.Reader
		indent       string
		expectOutput string
		expectErr    string
	}{
		{
			name:   "blank",
			r:      bytes.NewBufferString(""),
			indent: "",
		},
		{
			name:   "empty object",
			r:      bytes.NewBufferString("{}"),
			indent: "",
		},
		{
			name:   "nested object",
			r:      bytes.NewBufferString(`{"hash":{"a":1,"b":2},"array":[3,4]}`),
			indent: "\t",
		},
		{
			name:   "string",
			r:      bytes.NewBufferString(`"foo"`),
			indent: "",
		},
		{
			name:      "error",
			r:         bytes.NewBufferString("{{"),
			indent:    "",
			expectErr: "invalid character '{'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			err := jsoncolor.Write(w, tt.r, tt.indent)

			if tt.expectErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, cmp.ErrorContains(err, tt.expectErr))
			}

			assert.Check(t, golden.String(w.String(), tt.name+".txt"))
		})
	}
}
