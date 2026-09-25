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

package function_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

func TestFlags(t *testing.T) {
	t.Run("A published descriptor yields its arguments", func(t *testing.T) {
		flags := function.Flags(map[string]any{
			"flags": []any{
				map[string]any{
					"name": "version", "type": "string", "default": "stable",
					"description": "Go version spec.",
				},
				// A bool default arrives typed from some publishers.
				map[string]any{"name": "cache", "type": "bool", "default": true},
				// JSON numbers decode as float64.
				map[string]any{"name": "retries", "type": "int", "default": float64(1000000)},
			},
		})
		assert.Check(t, cmp.DeepEqual(flags, []function.Flag{
			{Name: "version", Type: "string", Default: "stable", Description: "Go version spec."},
			{Name: "cache", Type: "bool", Default: "true"},
			{Name: "retries", Type: "int", Default: "1000000"},
		}))
	})

	// The shape can differ between versions, so each of these must degrade
	// rather than error.
	for what, content := range map[string]map[string]any{
		"nil content":          nil,
		"no flags key":         {"name": "setup-go"},
		"flags is a string":    {"flags": "nope"},
		"entry not a map":      {"flags": []any{"version"}},
		"entry has no name":    {"flags": []any{map[string]any{"type": "string"}}},
		"name is not a string": {"flags": []any{map[string]any{"name": 42}}},
	} {
		t.Run(what, func(t *testing.T) {
			assert.Check(t, cmp.Len(function.Flags(content), 0))
		})
	}
}
