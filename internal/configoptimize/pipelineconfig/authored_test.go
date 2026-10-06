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

package pipelineconfig_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// TestRoundTrip covers what our own code adds on top of yaml.v3: the
// authored bytes come back unchanged, and Offset — our line index, which skips
// a byte order mark and counts columns in runes — puts every node at its own
// text in the source, which is what a surgical edit depends on. The inputs in
// testdata/roundtrip are the shapes that index could get wrong (comments,
// anchors and merge keys, several documents, non-ASCII text); each also runs
// with CRLF line endings, a leading byte order mark and no final newline,
// generated from its LF form.
func TestRoundTrip(t *testing.T) {
	variants := []struct {
		name string
		of   func([]byte) []byte
	}{
		{"lf", func(b []byte) []byte { return b }},
		{"crlf", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n")) }},
		{"bom", func(b []byte) []byte { return append(slices.Clone(pipelineconfig.UTF8BOM), b...) }},
		{"no final newline", func(b []byte) []byte { return bytes.TrimSuffix(b, []byte("\n")) }},
	}

	files, err := filepath.Glob(filepath.Join("testdata", "roundtrip", "*.yml"))
	assert.NilError(t, err)
	assert.Assert(t, len(files) > 0)

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".yml")
		lf, err := os.ReadFile(file) //#nosec:G304 // test fixture
		assert.NilError(t, err)
		assert.Assert(t, !bytes.Contains(lf, []byte("\r")), "%s must be LF only; the CRLF variant is generated", file)

		for _, v := range variants {
			t.Run(name+"/"+v.name, func(t *testing.T) {
				src := v.of(lf)
				if v.name == "crlf" {
					assert.Assert(t, bytes.Contains(src, []byte("\r\n")), "the CRLF variant must contain CRLF")
				}
				authored, err := pipelineconfig.Parse(src)
				assert.NilError(t, err)

				assert.Check(t, cmp.Equal(string(authored.Bytes()), string(src)), "round trip changed %s", file)
				checked := 0
				for _, doc := range authored.Documents() {
					checked += checkOffsets(t, authored, src, doc)
				}
				assert.Check(t, checked > 0, "no node had a fixed prefix to check")
			})
		}
	}
}

// checkOffsets checks that the source at each node's offset starts with the
// text that node must begin with, and returns how many nodes it checked.
func checkOffsets(t *testing.T, a *pipelineconfig.Authored, src []byte, n *yaml.Node) int {
	t.Helper()
	checked := 0
	if n.Kind != yaml.DocumentNode {
		off, err := a.Offset(n)
		assert.Check(t, err)
		if want := expectedPrefix(n); want != "" && err == nil {
			checked++
			assert.Check(t, cmp.Regexp("^"+regexp.QuoteMeta(want), snippetFor(src[off:], want)),
				"node %q at %d:%d: source at offset %d does not start with %q",
				n.Value, n.Line, n.Column, off, want)
		}
	}
	for _, c := range n.Content {
		checked += checkOffsets(t, a, src, c)
	}
	return checked
}

// expectedPrefix is the text a node's source must start with, or "" when the
// node kind has no fixed prefix (a block mapping starts at its first key).
func expectedPrefix(n *yaml.Node) string {
	if n.Anchor != "" {
		return "&" + n.Anchor
	}
	switch n.Kind {
	case yaml.AliasNode:
		return "*" + n.Value
	case yaml.ScalarNode:
		switch n.Style {
		case yaml.DoubleQuotedStyle:
			return `"`
		case yaml.SingleQuotedStyle:
			return "'"
		case yaml.LiteralStyle:
			return "|"
		case yaml.FoldedStyle:
			return ">"
		case yaml.TaggedStyle, yaml.FlowStyle:
			return ""
		}
		if n.Value == "" || strings.Contains(n.Value, "\n") {
			return ""
		}
		return n.Value
	case yaml.MappingNode:
		if n.Style == yaml.FlowStyle {
			return "{"
		}
	case yaml.SequenceNode:
		if n.Style == yaml.FlowStyle {
			return "["
		}
	case yaml.DocumentNode:
	}
	return ""
}

// snippetFor is the start of b, long enough to hold want plus some context,
// so a failed prefix check prints a short excerpt rather than the whole file.
func snippetFor(b []byte, want string) string {
	if n := len(want) + 20; len(b) > n {
		b = b[:n]
	}
	return string(b)
}

func TestParseRejectsInvalidYAML(t *testing.T) {
	_, err := pipelineconfig.Parse([]byte("jobs:\n  build: [unclosed\n"))
	_, ok := errors.AsType[*pipelineconfig.ParseError](err)
	assert.Check(t, ok, "want a *pipelineconfig.ParseError, got %v", err)
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	// yaml.v3 keeps both; a path match would then pick one of two nodes.
	_, err := pipelineconfig.Parse([]byte("jobs:\n  build:\n    machine:\n      docker_layer_caching: false\n      docker_layer_caching: true\n"))
	_, ok := errors.AsType[*pipelineconfig.ParseError](err)
	assert.Check(t, ok, "want a *pipelineconfig.ParseError, got %v", err)
	assert.Check(t, cmp.ErrorContains(err, `duplicate key "docker_layer_caching"`))

	_, err = pipelineconfig.Parse([]byte("a: &x {k: 1}\nb:\n  <<: *x\n  <<: *x\n"))
	assert.Check(t, cmp.Nil(err), "repeated merge keys are not duplicates")
}
