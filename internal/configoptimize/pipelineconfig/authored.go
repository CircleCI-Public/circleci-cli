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

package pipelineconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Authored is the config exactly as the user wrote it. It is the only copy
// config optimize ever edits; analysis reads only the compiled config.
//
// TODO: can any YAML library re-serialize byte-identically? None does across
// the YAML features configs use, so the authored copy is never re-serialized.
// The source bytes are the authored copy; the parsed tree is used only to find
// where nodes are. A no-op round trip is identical by construction, and
// an edit becomes a splice at a node's byte offset followed by a re-parse,
// which keeps diffs to the lines that changed.
type Authored struct {
	src        []byte
	docs       []*yaml.Node
	lineStarts []int
}

// ParseError reports YAML the parser rejected.
type ParseError struct {
	Err error
}

func (e *ParseError) Error() string { return "invalid YAML: " + e.Err.Error() }

func (e *ParseError) Unwrap() error { return e.Err }

// Parse reads every YAML document in src.
func Parse(src []byte) (*Authored, error) {
	a := &Authored{src: slices.Clone(src), lineStarts: lineStarts(src)}
	dec := yaml.NewDecoder(bytes.NewReader(a.src))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, &ParseError{Err: err}
		}
		if err := checkDuplicateKeys(&doc); err != nil {
			return nil, &ParseError{Err: err}
		}
		a.docs = append(a.docs, &doc)
	}
	return a, nil
}

// checkDuplicateKeys rejects a mapping with the same key twice. yaml.v3 keeps
// both, and a path match would then pick one of two nodes.
func checkDuplicateKeys(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag == "!!merge" {
				continue
			}
			if seen[k.Value] {
				return fmt.Errorf("line %d: duplicate key %q", k.Line, k.Value)
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if err := checkDuplicateKeys(c); err != nil {
			return err
		}
	}
	return nil
}

// Bytes returns the authored bytes. With no edits they are identical to what
// was parsed.
func (a *Authored) Bytes() []byte { return slices.Clone(a.src) }

// Documents returns the parsed document nodes, in order.
func (a *Authored) Documents() []*yaml.Node { return a.docs }

// Root returns the top-level mapping of the first document, or nil.
func (a *Authored) Root() *yaml.Node {
	if len(a.docs) == 0 || len(a.docs[0].Content) == 0 {
		return nil
	}
	root := a.docs[0].Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

// Offset returns the byte offset of a node's position in the source. yaml.v3
// reports 1-based lines and 1-based columns counted in runes; a node with an
// anchor is positioned at the anchor.
func (a *Authored) Offset(n *yaml.Node) (int, error) {
	if n.Line < 1 || n.Line > len(a.lineStarts) {
		return 0, fmt.Errorf("line %d is outside the source", n.Line)
	}
	off := a.lineStarts[n.Line-1]
	for col := 1; col < n.Column; col++ {
		if off >= len(a.src) || a.src[off] == '\n' {
			return 0, fmt.Errorf("column %d is past the end of line %d", n.Column, n.Line)
		}
		_, size := utf8.DecodeRune(a.src[off:])
		off += size
	}
	return off, nil
}

// UTF8BOM is the byte order mark yaml.v3 skips without counting it as a
// column, so line 1 starts after it.
var UTF8BOM = []byte{0xEF, 0xBB, 0xBF}

func lineStarts(src []byte) []int {
	first := 0
	if bytes.HasPrefix(src, UTF8BOM) {
		first = len(UTF8BOM)
	}
	starts := []int{first}
	for i, b := range src {
		if b == '\n' && i+1 <= len(src) {
			starts = append(starts, i+1)
		}
	}
	return starts
}
