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
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

// CheckDelta proves a candidate changed the compiled config exactly as the
// group said it would: after must equal before with the removed paths
// deleted and the set paths set. Mapping order is ignored; sequence order is kept. Two scoped
// equivalences apply, and only at the removed paths: a step emptied by the
// removal equals its bare name (the compiler emits `- setup_remote_docker`),
// and, when the group says so, an explicit false equals absent (the compiler
// keeps an explicit `docker_layer_caching: false`).
func CheckDelta(before, after []byte, want plan.Delta) error {
	b, err := decode(before)
	if err != nil {
		return fmt.Errorf("read the compiled config before the change: %w", err)
	}
	a, err := decode(after)
	if err != nil {
		return fmt.Errorf("read the compiled config after the change: %w", err)
	}
	for _, p := range want.Removed {
		old, ok := lookup(b, p)
		if !ok || !remove(&b, p) {
			return fmt.Errorf("%s was not in the compiled config before the change", strings.Join(p, "."))
		}
		if v, present := lookup(a, p); present {
			if !want.FalseMeansAbsent || old != true || v != false {
				return fmt.Errorf("%s is still %v after the change", strings.Join(p, "."), v)
			}
			remove(&a, p)
		}
		bareIfEmptied(b, p)
		bareIfEmptied(a, p)
	}
	for _, s := range want.Set {
		if _, ok := lookup(b, s.Path); !ok {
			return fmt.Errorf("%s was not in the compiled config before the change", strings.Join(s.Path, "."))
		}
		set(b, s.Path, s.Value)
	}
	if !reflect.DeepEqual(b, a) {
		changed := slices.Clone(want.Removed)
		for _, s := range want.Set {
			changed = append(changed, s.Path)
		}
		return fmt.Errorf("the change altered the compiled config beyond %s", describe(changed))
	}
	return nil
}

// SameCompile reports whether two compilations of the same bytes agree; a
// difference is compiler or environment drift, not the change's fault. It
// compares decoded values, so mapping order, comments and YAML spelling
// (anchors versus copies, 01 versus 1) are not differences: the compiled
// config is consumed as values, and a spelling change there changes nothing
// a pipeline sees.
func SameCompile(x, y []byte) (bool, error) {
	a, err := decode(x)
	if err != nil {
		return false, err
	}
	b, err := decode(y)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(a, b), nil
}

// decode reads exactly one YAML document; compiled output with more is an
// error, never a partial comparison.
func decode(src []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("expected exactly one YAML document")
	}
	return v, nil
}

// bareIfEmptied turns a step at path[:len-2] into its bare name when the
// removal at path left its mapping empty: (…, steps, i, name, key).
func bareIfEmptied(root any, path []string) {
	if len(path) < len([]string{"steps", "i", "name", "key"}) || path[len(path)-4] != "steps" {
		return
	}
	stepPath := path[:len(path)-1]
	params, ok := lookup(root, stepPath)
	if m, isMap := params.(map[string]any); !ok || (params != nil && (!isMap || len(m) > 0)) {
		return
	}
	seq, ok := lookup(root, stepPath[:len(stepPath)-2])
	items, isSeq := seq.([]any)
	i, err := strconv.Atoi(stepPath[len(stepPath)-2])
	if !ok || !isSeq || err != nil || i < 0 || i >= len(items) {
		return
	}
	if item, isMap := items[i].(map[string]any); isMap && len(item) == 1 {
		items[i] = stepPath[len(stepPath)-1]
	}
}

// remove deletes the value at path, returning whether it existed.
func remove(root *any, path []string) bool {
	if len(path) == 0 {
		return false
	}
	parent, ok := lookup(*root, path[:len(path)-1])
	if !ok {
		return false
	}
	m, ok := parent.(map[string]any)
	if !ok {
		return false
	}
	if _, exists := m[path[len(path)-1]]; !exists {
		return false
	}
	delete(m, path[len(path)-1])
	return true
}

func lookup(v any, path []string) (any, bool) {
	for _, part := range path {
		switch n := v.(type) {
		case map[string]any:
			next, ok := n[part]
			if !ok {
				return nil, false
			}
			v = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(n) {
				return nil, false
			}
			v = n[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func describe(paths [][]string) string {
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		parts = append(parts, strings.Join(p, "."))
	}
	return strings.Join(parts, ", ")
}
