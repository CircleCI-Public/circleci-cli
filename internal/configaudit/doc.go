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

// Package configaudit audits a CircleCI pipeline config for supply-chain risks.
package configaudit

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxAliasDepth bounds how many aliases resolve follows, so an alias chain that
// loops back on itself ends.
const maxAliasDepth = 32

// Doc is the parsed config with node handles the rules share. It is built once
// and read by every rule.
type Doc struct {
	file  string
	lines []string
	root  *yaml.Node

	// The top-level sections as resolved mapping nodes; nil when the section is
	// absent or not a mapping.
	orbs, executors, commands, jobs, workflows *yaml.Node
}

// Parse parses a pipeline config. A document that parses but is not a pipeline
// mapping yields a Doc with no sections, not an error: rejecting it is
// "config validate"'s job. The error names the YAML line.
func Parse(src []byte, file string) (*Doc, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(src, &node); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	d := &Doc{
		file:  file,
		lines: strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n"),
	}
	root := &node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	d.root = resolve(root)
	if d.root == nil || d.root.Kind != yaml.MappingNode {
		return d, nil
	}
	section := func(name string) *yaml.Node {
		n := mapGet(d.root, name)
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		return n
	}
	d.orbs = section("orbs")
	d.executors = section("executors")
	d.commands = section("commands")
	d.jobs = section("jobs")
	d.workflows = section("workflows")
	return d, nil
}

// resolve follows an alias to its anchored node.
func resolve(n *yaml.Node) *yaml.Node {
	for depth := 0; n != nil && n.Kind == yaml.AliasNode && depth < maxAliasDepth; depth++ {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.AliasNode {
		return nil
	}
	return n
}

func isMergeKey(k *yaml.Node) bool {
	return k.Kind == yaml.ScalarNode && k.Value == "<<" && (k.Tag == "!!merge" || k.Tag == "")
}

// mapPairs returns a mapping's key/value pairs with "<<" merges spliced in
// (merged pairs first, so explicit keys override), each value resolved.
func mapPairs(n *yaml.Node) [][2]*yaml.Node {
	return mapPairsVisiting(n, map[*yaml.Node]bool{})
}

// mapPairsVisiting is mapPairs with the mappings being merged into, so a merge
// that leads back to one of them is ignored rather than followed forever.
func mapPairsVisiting(n *yaml.Node, visiting map[*yaml.Node]bool) [][2]*yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode || visiting[n] {
		return nil
	}
	visiting[n] = true
	defer delete(visiting, n)
	var explicit, merged [][2]*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if isMergeKey(k) {
			continue
		}
		explicit = append(explicit, [2]*yaml.Node{k, resolve(v)})
		seen[k.Value] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], resolve(n.Content[i+1])
		if !isMergeKey(k) || v == nil {
			continue
		}
		sources := []*yaml.Node{v}
		if v.Kind == yaml.SequenceNode {
			sources = nil
			for _, item := range v.Content {
				sources = append(sources, resolve(item))
			}
		}
		// Earlier sources take precedence over later ones.
		for _, src := range sources {
			for _, kv := range mapPairsVisiting(src, visiting) {
				if !seen[kv[0].Value] {
					seen[kv[0].Value] = true
					merged = append(merged, kv)
				}
			}
		}
	}
	return append(merged, explicit...)
}

// mapGet returns the value for key in mapping n, merges and aliases followed.
func mapGet(n *yaml.Node, key string) *yaml.Node {
	for _, kv := range mapPairs(n) {
		if kv[0].Value == key {
			return kv[1]
		}
	}
	return nil
}

// seqItems returns the items of a sequence, each resolved; nil when n is not a
// sequence.
func seqItems(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	items := make([]*yaml.Node, 0, len(n.Content))
	for _, item := range n.Content {
		if r := resolve(item); r != nil {
			items = append(items, r)
		}
	}
	return items
}

// step is one entry of a steps list.
type step struct {
	name     string     // "run", "checkout", "orb/command", ...
	body     *yaml.Node // the step's value; nil for a bare-string step
	nameNode *yaml.Node // the scalar holding name
}

// steps returns each step of a steps sequence, in both the bare-string form
// (`- checkout`) and the single-key-map form (`- run: ...`).
func steps(n *yaml.Node) []step {
	var out []step
	for _, item := range seqItems(n) {
		if item.Kind == yaml.ScalarNode {
			out = append(out, step{name: item.Value, nameNode: item})
			continue
		}
		if pairs := mapPairs(item); len(pairs) == 1 {
			out = append(out, step{name: pairs[0][0].Value, body: pairs[0][1], nameNode: pairs[0][0]})
		}
	}
	return out
}
