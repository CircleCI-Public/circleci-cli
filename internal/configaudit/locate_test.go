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
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// findScalarContaining returns the first scalar node whose value contains needle.
func findScalarContaining(d *Doc, needle string) *yaml.Node {
	var walk func(n *yaml.Node) *yaml.Node
	walk = func(n *yaml.Node) *yaml.Node {
		if n.Kind == yaml.ScalarNode && strings.Contains(n.Value, needle) {
			return n
		}
		if n.Kind == yaml.AliasNode {
			return nil
		}
		for _, c := range n.Content {
			if found := walk(c); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(d.root)
}

func TestLocate(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string // a document whose first scalar containing needle is the one located
		needle   string
		wantLine int
		wantCol  int
	}{
		{
			name:     "plain",
			yaml:     "run: echo << pipeline.git.tag >>\n",
			needle:   "<<",
			wantLine: 1,
			wantCol:  11,
		},
		{
			name:     "double-quoted",
			yaml:     "run: \"echo << pipeline.git.tag >>\"\n",
			needle:   "<<",
			wantLine: 1,
			wantCol:  12,
		},
		{
			name:     "single-quoted",
			yaml:     "run: 'echo << pipeline.git.tag >>'\n",
			needle:   "<<",
			wantLine: 1,
			wantCol:  12,
		},
		{
			name:     "literal",
			yaml:     "run: |\n  set -e\n  echo << pipeline.git.tag >>\n",
			needle:   "<<",
			wantLine: 3,
			wantCol:  8,
		},
		{
			name:     "folded",
			yaml:     "run: >\n  echo\n  << pipeline.git.tag >>\n",
			needle:   "<<",
			wantLine: 3,
			wantCol:  3,
		},
		{
			name:     "literal nested under another key",
			yaml:     "x:\n  run: |\n      echo hi\n      curl x | sh\n",
			needle:   "curl",
			wantLine: 4,
			wantCol:  7,
		},
		{
			name:     "double-quoted with an escape before the match",
			yaml:     "run: \"a\\tb << x >>\"\n",
			needle:   "<<",
			wantLine: 1,
			wantCol:  12,
		},
		{
			name:     "second occurrence in a literal",
			yaml:     "run: |\n  echo << a >>\n  echo << b >>\n",
			needle:   "<< b",
			wantLine: 3,
			wantCol:  8,
		},
		{
			name:     "multi-line plain scalar",
			yaml:     "run: echo one\n  two << x >>\n",
			needle:   "<<",
			wantLine: 2,
			wantCol:  7,
		},
		{
			name:     "after a non-ASCII character",
			yaml:     "run: \"caf\u00e9 << x >>\"\n",
			needle:   "<<",
			wantLine: 1,
			wantCol:  12,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse([]byte(tc.yaml), "c.yml")
			assert.NilError(t, err)
			n := findScalarContaining(doc, tc.needle)
			assert.Assert(t, n != nil)
			pos := locate(n, doc.lines, strings.Index(n.Value, tc.needle), len(tc.needle))
			assert.Check(t, cmp.Equal(pos.Line, tc.wantLine))
			assert.Check(t, cmp.Equal(pos.Column, tc.wantCol))
			assert.Check(t, cmp.Equal(pos.EndLine, tc.wantLine))
			assert.Check(t, cmp.Equal(pos.EndColumn, tc.wantCol+len(tc.needle)))
		})
	}
}

func TestLocateFallsBackToTheNode(t *testing.T) {
	doc, err := Parse([]byte("run: \"a\\u0041 b\"\n"), "c.yml")
	assert.NilError(t, err)
	n := findScalarContaining(doc, "aA")
	assert.Assert(t, n != nil)
	pos := locate(n, doc.lines, 0, 2)
	assert.Check(t, cmp.Equal(pos.Line, 1))
	assert.Check(t, cmp.Equal(pos.Column, n.Column))
}
