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

package oauth

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func Test_renderPage(t *testing.T) {
	tests := []struct {
		name  string
		state browserState
	}{
		{
			name: "success",
			state: browserState{
				Title:   "Authorization successful",
				Body:    "You can close this window and return to your terminal.",
				Success: true,
			},
		},
		{
			name: "failure",
			state: browserState{
				Title:   "Authorization failed",
				Body:    "The authorization response did not include a code.",
				Success: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			renderPage(w, tt.state)
			assert.Check(t, golden.String(w.String(), tt.name+".html"))
		})
	}
}
