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

// Package patch is the editing kernel: it turns one plan.Group into
// candidate bytes, and checks a candidate's compiled change against the
// group's expected delta. It knows nothing about modules. It deletes,
// replaces and inserts only the narrow shapes it can prove it edits
// correctly, and refuses everything else.
package patch

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

// RefusedError reports an edit the kernel will not make. The group is
// rejected; nothing is guessed.
type RefusedError struct {
	Path   []string
	Reason string
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("%s: %s", strings.Join(e.Path, "."), e.Reason)
}

func refuse(path []string, format string, args ...any) error {
	return &RefusedError{Path: slices.Clone(path), Reason: fmt.Sprintf(format, args...)}
}

// splice replaces src[start:end] with text.
type splice struct {
	start, end int
	text       string
}

// Candidate applies every edit in the group to src, all or nothing, and
// returns the new bytes. The candidate is re-parsed before it is returned.
func Candidate(src []byte, group plan.Group) ([]byte, error) {
	a, err := pipelineconfig.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse current config: %w", err)
	}
	if len(a.Documents()) != 1 {
		return nil, refuse(nil, "a config with %d YAML documents is not edited", len(a.Documents()))
	}
	if loneCR(src) {
		// YAML treats a lone CR as a line break; the line arithmetic here
		// does not, so it would delete the wrong text.
		return nil, refuse(nil, "the config has a carriage return without a line feed")
	}
	var (
		splices []splice
		bare    [][]string // step paths rewritten to their bare name
	)
	for _, e := range group.Edits {
		s, isBare, err := editSplices(a, src, e)
		if err != nil {
			return nil, err
		}
		splices = append(splices, s...)
		if isBare {
			bare = append(bare, e.Path[:len(e.Path)-1])
		}
	}
	out, err := apply(src, splices)
	if err != nil {
		return nil, err
	}
	if _, err := pipelineconfig.Parse(out); err != nil {
		return nil, fmt.Errorf("the edited config does not parse: %w", err)
	}
	if err := sameExceptEdits(src, out, group.Edits, bare); err != nil {
		return nil, err
	}
	return out, nil
}

// sameExceptEdits is the kernel's own guard: decoded, the candidate must
// equal the original with exactly the group's edits made (and a step
// emptied by its last parameter written as its bare name). It catches any
// text the line arithmetic got wrong: a folded continuation line, a
// multi-line quoted value, an emptied mapping left as null.
func sameExceptEdits(src, out []byte, edits []plan.Edit, bare [][]string) error {
	var want, got any
	if err := yaml.Unmarshal(src, &want); err != nil {
		return err
	}
	if err := yaml.Unmarshal(out, &got); err != nil {
		return err
	}
	for _, e := range edits {
		switch e.Op {
		case plan.Replace:
			if !set(want, e.Path, e.Value) {
				return refuse(e.Path, "the value to replace is not in the decoded config")
			}
		case plan.Delete:
			if !remove(&want, e.Path) {
				return refuse(e.Path, "the entry to delete is not in the decoded config")
			}
		case plan.Insert:
			// The key may already be in the decoded config, supplied by a
			// merge key; the new key of the mapping's own wins either way.
			if !upsert(want, e.Path, e.Value) {
				return refuse(e.Path, "the mapping to insert into is not in the decoded config")
			}
		}
	}
	for _, p := range bare {
		if !bareStep(want, p) {
			return refuse(p, "the step could not be written as its bare name")
		}
	}
	if !reflect.DeepEqual(want, got) {
		return refuse(nil, "the edit changed more of the config than the entries it edits")
	}
	return nil
}

// bareStep replaces the step at path (…, steps, i, name) with "name".
func bareStep(root any, path []string) bool {
	seq, ok := lookup(root, path[:len(path)-2])
	if !ok {
		return false
	}
	items, ok := seq.([]any)
	if !ok {
		return false
	}
	i, err := strconv.Atoi(path[len(path)-2])
	if err != nil || i < 0 || i >= len(items) {
		return false
	}
	items[i] = path[len(path)-1]
	return true
}

func loneCR(src []byte) bool {
	for i, b := range src {
		if b == '\r' && (i+1 == len(src) || src[i+1] != '\n') {
			return true
		}
	}
	return false
}

func editSplices(a *pipelineconfig.Authored, src []byte, e plan.Edit) ([]splice, bool, error) {
	switch e.Op {
	case plan.Delete:
		return deleteSplices(a, src, e)
	case plan.Replace:
		s, err := replaceSplices(a, src, e)
		return s, false, err
	case plan.Insert:
		s, err := insertSplices(a, src, e)
		return s, false, err
	}
	return nil, false, refuse(e.Path, "unknown operation %d", e.Op)
}

// entry is a located mapping entry.
type entry struct {
	parent *yaml.Node // the mapping holding the entry
	key    *yaml.Node
	value  *yaml.Node
	// holder is the node containing parent (a mapping or sequence), and
	// holderKey the key under which parent sits when holder is a mapping.
	holder    *yaml.Node
	holderKey *yaml.Node
}

// locate walks an authored path: mapping keys, and sequence indexes.
// Aliases are not followed: an edit through an alias would change every
// place the anchor is used.
func locate(a *pipelineconfig.Authored, path []string) (entry, error) {
	var loc entry
	n := a.Root()
	if n == nil {
		return loc, refuse(path, "the config has no top-level mapping")
	}
	var holder, holderKey *yaml.Node
	for i, part := range path {
		last := i == len(path)-1
		switch n.Kind {
		case yaml.MappingNode:
			k, v := mappingEntry(n, part)
			if k == nil {
				return loc, refuse(path, "%s is not in the config", strings.Join(path[:i+1], "."))
			}
			if last {
				return entry{parent: n, key: k, value: v, holder: holder, holderKey: holderKey}, nil
			}
			holder, holderKey, n = n, k, v
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(n.Content) {
				return loc, refuse(path, "%s is not a valid index", strings.Join(path[:i+1], "."))
			}
			if last {
				return loc, refuse(path, "only mapping entries can be deleted")
			}
			holder, holderKey, n = n, nil, n.Content[idx]
		case yaml.AliasNode:
			return loc, refuse(path, "%s goes through an alias", strings.Join(path[:i], "."))
		case yaml.DocumentNode, yaml.ScalarNode:
			return loc, refuse(path, "%s is not a mapping or sequence", strings.Join(path[:i], "."))
		}
	}
	return loc, refuse(path, "empty path")
}

func mappingEntry(n *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key && k.Tag != "!!merge" {
			return k, n.Content[i+1]
		}
	}
	return nil, nil
}

// deleteSplices deletes one mapping entry. Allowed only for a direct block
// entry whose scalar key and scalar value share one physical line, with only
// spaces before the key and nothing after the value: no anchor, alias, tag,
// merge, flow style, multi-line token or inline comment.
func deleteSplices(a *pipelineconfig.Authored, src []byte, e plan.Edit) ([]splice, bool, error) {
	loc, err := locate(a, e.Path)
	if err != nil {
		return nil, false, err
	}
	if err := checkPlain(e.Path, loc); err != nil {
		return nil, false, err
	}
	var current any
	if err := loc.value.Decode(&current); err != nil || !reflect.DeepEqual(current, e.Expect) {
		return nil, false, refuse(e.Path, "the value is %v, not the %v that was analysed", current, e.Expect)
	}
	keyOff, err := a.Offset(loc.key)
	if err != nil {
		return nil, false, refuse(e.Path, "cannot locate the key: %v", err)
	}
	lineStart, lineEnd := lineBounds(src, keyOff)
	if strings.TrimLeft(string(src[lineStart:keyOff]), " ") != "" {
		return nil, false, refuse(e.Path, "something other than spaces precedes the key on its line")
	}
	valOff, err := a.Offset(loc.value)
	if err != nil {
		return nil, false, refuse(e.Path, "cannot locate the value: %v", err)
	}
	rest := strings.TrimRight(string(src[valOff:lineEnd]), "\r\n")
	if strings.Contains(rest, "#") {
		return nil, false, refuse(e.Path, "the line has an inline comment, which would be deleted with it")
	}
	out := []splice{{start: lineStart, end: lineEnd}}

	if len(loc.parent.Content) == len([]*yaml.Node{loc.key, loc.value}) {
		// The entry is the mapping's only child.
		s, err := soleChild(a, src, e.Path, loc)
		if err != nil {
			return nil, false, err
		}
		return append(out, s), true, nil
	}
	return out, false, nil
}

func checkPlain(path []string, loc entry) error {
	switch {
	case loc.parent.Style&yaml.FlowStyle != 0:
		return refuse(path, "the mapping is in flow style")
	case loc.key.Kind != yaml.ScalarNode || loc.value.Kind != yaml.ScalarNode:
		return refuse(path, "only a scalar key with a scalar value is deleted")
	case loc.key.Anchor != "" || loc.value.Anchor != "":
		return refuse(path, "the entry carries an anchor")
	case loc.key.Style&yaml.TaggedStyle != 0 || loc.value.Style&yaml.TaggedStyle != 0:
		return refuse(path, "the entry carries an explicit tag")
	case loc.value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 || strings.Contains(loc.value.Value, "\n"):
		return refuse(path, "the value spans more than one line")
	case loc.key.Line != loc.value.Line:
		return refuse(path, "the key and value are on different lines")
	case loc.key.LineComment != "" || loc.value.LineComment != "":
		return refuse(path, "the line has an inline comment, which would be deleted with it")
	case loc.key.FootComment != "" || loc.value.FootComment != "" || loc.value.HeadComment != "":
		return refuse(path, "a comment is attached to the entry, which would be left describing nothing")
	case loc.key.HeadComment != "":
		// A comment directly above the entry most likely describes it;
		// deleting the entry would leave the note describing nothing.
		return refuse(path, "the entry has a comment above it, which would be left describing nothing")
	}
	return nil
}

// soleChild handles deleting a mapping's only entry. A step whose only
// parameter is removed becomes the bare step name (`- setup_remote_docker`);
// the compiler rejects a null step. Any other emptied mapping is refused:
// the compiler rejects `machine:` with nothing under it.
func soleChild(a *pipelineconfig.Authored, src []byte, path []string, loc entry) (splice, error) {
	// Only a step in a job's or command's steps list may become bare:
	// jobs.<job>.steps.<i>.<step>.<key> or commands.<cmd>.steps.<i>.<step>.<key>.
	if len(path) != len([]string{"jobs", "j", "steps", "i", "step", "key"}) ||
		(path[0] != "jobs" && path[0] != "commands") || path[2] != "steps" {
		return splice{}, refuse(path, "deleting the only key would leave an empty mapping")
	}
	step := loc.holder
	if step == nil || loc.holderKey == nil || step.Kind != yaml.MappingNode ||
		len(step.Content) != len([]*yaml.Node{loc.holderKey, loc.parent}) || !isSequenceItem(a.Root(), step) {
		return splice{}, refuse(path, "deleting the only key would leave an empty mapping")
	}
	name := loc.holderKey
	if name.Kind != yaml.ScalarNode || name.Style != 0 || name.Anchor != "" || name.LineComment != "" {
		return splice{}, refuse(path, "the step name is not a plain scalar")
	}
	nameOff, err := a.Offset(name)
	if err != nil {
		return splice{}, refuse(path, "cannot locate the step name: %v", err)
	}
	_, lineEnd := lineBounds(src, nameOff)
	afterName := nameOff + len(name.Value)
	if afterName > lineEnd || string(src[nameOff:afterName]) != name.Value {
		return splice{}, refuse(path, "the step name is not written as plain text")
	}
	tail := strings.TrimRight(string(src[afterName:lineEnd]), "\r\n")
	if strings.TrimRight(tail, " ") != ":" {
		return splice{}, refuse(path, "the step line holds more than `%s:`", name.Value)
	}
	// Drop the colon and trailing spaces, keep the line ending.
	return splice{start: afterName, end: afterName + len(tail)}, nil
}

func isSequenceItem(root, target *yaml.Node) bool {
	found := false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil || found {
			return
		}
		if n.Kind == yaml.SequenceNode && slices.Contains(n.Content, target) {
			found = true
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(root)
	return found
}

// lineBounds returns the start of the line containing off and the end of
// that line including its line ending (or the end of src).
func lineBounds(src []byte, off int) (int, int) {
	start := bytes.LastIndexByte(src[:off], '\n') + 1
	if start == 0 && bytes.HasPrefix(src, pipelineconfig.UTF8BOM) {
		start = len(pipelineconfig.UTF8BOM) // keep a leading byte order mark
	}
	end := len(src)
	if nl := bytes.IndexByte(src[off:], '\n'); nl >= 0 {
		end = off + nl + 1
	}
	return start, end
}

// apply performs non-overlapping splices, last first, so earlier offsets stay
// valid.
func apply(src []byte, splices []splice) ([]byte, error) {
	sorted := slices.Clone(splices)
	slices.SortFunc(sorted, func(x, y splice) int { return y.start - x.start })
	for i := 1; i < len(sorted); i++ {
		// Two insertions at one offset would land in an arbitrary order.
		if sorted[i].end > sorted[i-1].start || sorted[i].start == sorted[i-1].start {
			return nil, refuse(nil, "two edits overlap")
		}
	}
	out := slices.Clone(src)
	for _, s := range sorted {
		out = slices.Concat(out[:s.start], []byte(s.text), out[s.end:])
	}
	return out, nil
}
