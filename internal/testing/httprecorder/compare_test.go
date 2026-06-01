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

package httprecorder

import (
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestIgnoreHeaders(t *testing.T) {
	assert.Check(t, cmp.DeepEqual(
		http.Header{
			"a": []string{"a"},
			"b": []string{"b"},
			"c": []string{"c1", "c2"},
		},
		http.Header{
			"a": []string{"a"},
			"b": []string{"difference-ignored"},
			"c": []string{"c1", "c2"},
		},
		IgnoreHeaders("b"),
	))
}

func TestOnlyHeaders(t *testing.T) {
	assert.Check(t, cmp.DeepEqual(
		http.Header{
			"a": []string{"ignored"},
			"b": []string{"b"},
			"c": []string{"c1", "c2"},
			"d": []string{"ignored"},
			"e": []string{"ignored"},
		},
		http.Header{
			"b": []string{"b"},
			"c": []string{"c1", "c2"},
		},
		OnlyHeaders("b", "c"),
	))
}
