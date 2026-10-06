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

package patch

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

// replaceSplices replaces one scalar. Only the scalar's own bytes
// change, so comments, layout and the rest of the line stay as authored.
// Allowed for a one-line plain, single- or double-quoted scalar, as a mapping
// value or a sequence item, in block or flow style, with no anchor, alias or
// tag. The new value is written plain when that is unambiguous, otherwise
// single-quoted.
func replaceSplices(a *pipelineconfig.Authored, src []byte, e plan.Edit) ([]splice, error) {
	n, flow, err := locateScalar(a, e.Path)
	if err != nil {
		return nil, err
	}
	switch {
	case n.Anchor != "" || n.Style&yaml.TaggedStyle != 0:
		return nil, refuse(e.Path, "the value carries an anchor or tag")
	case n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 || strings.Contains(n.Value, "\n"):
		return nil, refuse(e.Path, "the value spans more than one line")
	}
	var current any
	if err := n.Decode(&current); err != nil || !reflect.DeepEqual(current, e.Expect) {
		return nil, refuse(e.Path, "the value is %v, not the %v that was analysed", current, e.Expect)
	}
	value, ok := e.Value.(string)
	if !ok {
		return nil, refuse(e.Path, "only a string value is written")
	}
	start, err := a.Offset(n)
	if err != nil {
		return nil, refuse(e.Path, "cannot locate the value: %v", err)
	}
	end, err := scalarEnd(src, start, n.Style, flow)
	if err != nil {
		return nil, refuse(e.Path, "%v", err)
	}
	return []splice{{start: start, end: end, text: yamlScalar(value, flow)}}, nil
}

// locateScalar finds the scalar at an authored path, which may end at a
// mapping value or a sequence item, and says whether its parent is a flow
// collection.
func locateScalar(a *pipelineconfig.Authored, path []string) (*yaml.Node, bool, error) {
	n := a.Root()
	if n == nil || len(path) == 0 {
		return nil, false, refuse(path, "nothing to replace")
	}
	flow := false
	for i, part := range path {
		switch n.Kind {
		case yaml.MappingNode:
			_, v := mappingEntry(n, part)
			if v == nil {
				return nil, false, refuse(path, "%s is not in the config", strings.Join(path[:i+1], "."))
			}
			flow = n.Style&yaml.FlowStyle != 0
			n = v
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(n.Content) {
				return nil, false, refuse(path, "%s is not a valid index", strings.Join(path[:i+1], "."))
			}
			flow = n.Style&yaml.FlowStyle != 0
			n = n.Content[idx]
		case yaml.AliasNode, yaml.DocumentNode, yaml.ScalarNode:
			return nil, false, refuse(path, "%s is not a mapping or sequence", strings.Join(path[:i], "."))
		}
		if n.Kind == yaml.AliasNode {
			return nil, false, refuse(path, "%s goes through an alias", strings.Join(path[:i+1], "."))
		}
	}
	if n.Kind != yaml.ScalarNode {
		return nil, false, refuse(path, "only a scalar is replaced")
	}
	return n, flow, nil
}

// scalarEnd returns the offset just past a scalar that starts at start.
func scalarEnd(src []byte, start int, style yaml.Style, flow bool) (int, error) {
	switch {
	case style&yaml.DoubleQuotedStyle != 0:
		for i := start + 1; i < len(src); i++ {
			switch src[i] {
			case '\\':
				i++
			case '"':
				return i + 1, nil
			case '\n':
				return 0, errMultiLine
			}
		}
	case style&yaml.SingleQuotedStyle != 0:
		for i := start + 1; i < len(src); i++ {
			switch {
			case src[i] == '\'' && i+1 < len(src) && src[i+1] == '\'':
				i++
			case src[i] == '\'':
				return i + 1, nil
			case src[i] == '\n':
				return 0, errMultiLine
			}
		}
	default:
		// A plain scalar ends at the line end, at " #", or in flow style at
		// a flow indicator. Trailing spaces are not part of it.
		i := start
		for i < len(src) && src[i] != '\n' && src[i] != '\r' {
			if flow && strings.IndexByte(",]}", src[i]) >= 0 {
				break
			}
			if src[i] == '#' && i > start && (src[i-1] == ' ' || src[i-1] == '\t') {
				break
			}
			i++
		}
		for i > start && (src[i-1] == ' ' || src[i-1] == '\t') {
			i--
		}
		return i, nil
	}
	return 0, errMultiLine
}

var errMultiLine = refuse(nil, "the quoted value is not closed on its line")

// plainSafe are values written without quotes: no YAML indicator, no space,
// and not a word YAML reads as another type.
var plainSafe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._+/-]*$`)

func yamlScalar(v string, flow bool) string {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "y", "n":
	default:
		if plainSafe.MatchString(v) && (!flow || !strings.ContainsAny(v, ",[]{}")) {
			return v
		}
	}
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

// set assigns a decoded value at path, returning whether the path existed.
func set(root any, path []string, value any) bool {
	if len(path) == 0 {
		return false
	}
	parent, ok := lookup(root, path[:len(path)-1])
	if !ok {
		return false
	}
	last := path[len(path)-1]
	switch p := parent.(type) {
	case map[string]any:
		if _, exists := p[last]; !exists {
			return false
		}
		p[last] = value
		return true
	case []any:
		i, err := strconv.Atoi(last)
		if err != nil || i < 0 || i >= len(p) {
			return false
		}
		p[i] = value
		return true
	}
	return false
}

// upsert assigns a decoded value at path, adding the key when it is absent,
// and returns whether the parent mapping existed.
func upsert(root any, path []string, value any) bool {
	if len(path) == 0 {
		return false
	}
	parent, ok := lookup(root, path[:len(path)-1])
	if !ok {
		return false
	}
	m, ok := parent.(map[string]any)
	if !ok {
		return false
	}
	m[path[len(path)-1]] = value
	return true
}
