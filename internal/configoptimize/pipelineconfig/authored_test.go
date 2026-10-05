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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// TestRoundTrip checks that, for every YAML feature in
// testdata/roundtrip, parsing then writing back gives identical bytes, and
// every node's recorded position points at that node's own text in the source
// — which is what a surgical edit depends on. The position dump is a
// generated golden: `task test -- ./internal/configoptimize/config/... -update`.
func TestRoundTrip(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "roundtrip", "*.yml"))
	assert.NilError(t, err)
	assert.Assert(t, len(files) > 0)

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".yml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file) //#nosec:G304 // test fixture
			assert.NilError(t, err)

			authored, err := pipelineconfig.Parse(src)
			assert.NilError(t, err)

			t.Run("bytes are identical", func(t *testing.T) {
				assert.Check(t, cmp.Equal(string(authored.Bytes()), string(src)), "round trip changed %s", file)
			})

			t.Run("positions point at their own text", func(t *testing.T) {
				var dump strings.Builder
				for i, doc := range authored.Documents() {
					_, _ = fmt.Fprintf(&dump, "document %d\n", i)
					walk(t, authored, src, doc, 0, &dump)
				}
				assert.Check(t, golden.String(dump.String(), filepath.Join("roundtrip", name+".nodes.txt")))
			})
		})
	}
}

// walk writes one line per node and checks that the source at the node's
// offset starts with the text that node must begin with.
func walk(t *testing.T, a *pipelineconfig.Authored, src []byte, n *yaml.Node, depth int, out *strings.Builder) {
	t.Helper()
	if n.Kind != yaml.DocumentNode {
		off, err := a.Offset(n)
		assert.Check(t, err)
		_, _ = fmt.Fprintf(out, "%s%s %d:%d @%d %q\n", strings.Repeat("  ", depth), kindName(n), n.Line, n.Column, off, n.Value)
		if want := expectedPrefix(n); want != "" {
			assert.Check(t, cmp.Regexp("^"+regexp.QuoteMeta(want), snippetFor(src[off:], want)),
				"node %q at %d:%d: source at offset %d does not start with %q",
				n.Value, n.Line, n.Column, off, want)
		}
	}
	for _, c := range n.Content {
		walk(t, a, src, c, depth+1, out)
	}
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

func kindName(n *yaml.Node) string {
	switch n.Kind {
	case yaml.DocumentNode:
		return "doc"
	case yaml.MappingNode:
		return "map"
	case yaml.SequenceNode:
		return "seq"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	}
	return "unknown"
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
