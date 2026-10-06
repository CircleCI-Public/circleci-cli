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
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

// insertSplices adds one `key: value` line to a mapping that does not yet
// have the key: directly below the mapping's own key line, at the
// indentation of its first child, with that line's own line ending. Nothing
// else moves, so a comment above the first child stays with it. Allowed only
// for a block mapping with no anchor, alias or tag, whose key line holds
// nothing but the key (and a comment), and whose children are indented with
// spaces. The value must be a plain identifier; the key must be absent, even
// beside a merge key, which it then overrides.
func insertSplices(a *pipelineconfig.Authored, src []byte, e plan.Edit) ([]splice, error) {
	if len(e.Path) < len([]string{"parent", "key"}) {
		return nil, refuse(e.Path, "nothing to insert into")
	}
	key := e.Path[len(e.Path)-1]
	value, ok := e.Value.(string)
	if !ok || !plainString(value) {
		return nil, refuse(e.Path, "only a plain identifier value is inserted")
	}
	if !plainString(key) {
		return nil, refuse(e.Path, "only a plain identifier key is inserted")
	}
	loc, err := locate(a, e.Path[:len(e.Path)-1])
	if err != nil {
		return nil, err
	}
	m := loc.value
	if err := checkInsertTarget(e.Path, loc); err != nil {
		return nil, err
	}
	first := m.Content[0]
	keyOff, err := a.Offset(loc.key)
	if err != nil {
		return nil, refuse(e.Path, "cannot locate the key: %v", err)
	}
	_, keyLineEnd := lineBounds(src, keyOff)
	afterKey := keyOff + len(loc.key.Value)
	if afterKey > keyLineEnd || string(src[keyOff:afterKey]) != loc.key.Value {
		return nil, refuse(e.Path, "the key is not written as plain text")
	}
	rest := strings.TrimLeft(strings.TrimRight(string(src[afterKey:keyLineEnd]), "\r\n"), " ")
	if !strings.HasPrefix(rest, ":") {
		return nil, refuse(e.Path, "the key line is not `%s:`", loc.key.Value)
	}
	if tail := strings.TrimLeft(rest[1:], " "); tail != "" && !strings.HasPrefix(tail, "#") {
		return nil, refuse(e.Path, "the key line holds more than the key")
	}
	firstOff, err := a.Offset(first)
	if err != nil {
		return nil, refuse(e.Path, "cannot locate the first entry: %v", err)
	}
	firstLineStart, _ := lineBounds(src, firstOff)
	indent := string(src[firstLineStart:firstOff])
	if indent == "" || strings.Trim(indent, " ") != "" {
		return nil, refuse(e.Path, "the entries are not indented with spaces only")
	}
	if keyLineEnd == len(src) || src[keyLineEnd-1] != '\n' {
		return nil, refuse(e.Path, "the key line has no line ending")
	}
	newline := "\n"
	if keyLineEnd >= len("\r\n") && src[keyLineEnd-len("\r\n")] == '\r' {
		newline = "\r\n"
	}
	return []splice{{start: keyLineEnd, end: keyLineEnd, text: indent + key + ": " + value + newline}}, nil
}

// checkInsertTarget refuses a mapping the new key cannot be added to safely.
func checkInsertTarget(path []string, loc entry) error {
	m, key := loc.value, path[len(path)-1]
	switch {
	case m.Kind == yaml.AliasNode:
		return refuse(path, "the mapping is an alias")
	case m.Kind != yaml.MappingNode || len(m.Content) == 0:
		return refuse(path, "only a non-empty mapping gets a new key")
	case m.Style&yaml.FlowStyle != 0:
		return refuse(path, "the mapping is in flow style")
	case m.Anchor != "" || loc.key.Anchor != "":
		return refuse(path, "the mapping carries an anchor")
	case m.Style&yaml.TaggedStyle != 0 || loc.key.Style&yaml.TaggedStyle != 0:
		return refuse(path, "the mapping carries an explicit tag")
	case loc.key.Kind != yaml.ScalarNode || loc.key.Style != 0:
		return refuse(path, "the mapping's key is not a plain scalar")
	case m.Content[0].Line <= loc.key.Line:
		return refuse(path, "the mapping starts on its key's line")
	}
	if k, _ := mappingEntry(m, key); k != nil {
		return refuse(path, "the mapping already has %s", key)
	}
	return nil
}

// identifier is what may be written unquoted as an inserted key or value.
var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// plainString reports whether v can be written without quotes and still read
// back as exactly the string v: `2xlarge` can, `true`, `1e3` and `0x10` cannot.
func plainString(v string) bool {
	if !identifier.MatchString(v) {
		return false
	}
	var back any
	if err := yaml.Unmarshal([]byte(v), &back); err != nil {
		return false
	}
	s, ok := back.(string)
	return ok && s == v
}
