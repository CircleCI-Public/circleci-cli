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

package function

import (
	"bytes"
	"maps"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Flag is one argument a function accepts, read out of a published descriptor.
type Flag struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// Flags reads the arguments a function accepts: its root flags, then those of
// the command a bare invocation runs, which a function built with a default
// subcommand publishes under commands.<default>. A descriptor is served exactly
// as published and its shape can differ between versions, so a missing or
// unexpectedly typed field degrades to a thinner result rather than an error.
func Flags(content map[string]any) []Flag {
	var flags []Flag
	for node := content; node != nil; node = defaultCommand(node) {
		flags = append(flags, nodeFlags(node)...)
	}
	return flags
}

// Command is one of a function's commands, which a step names after the alias
// as <alias>/<name>.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Flags       []Flag `json:"flags"`
}

// DefaultCommand returns the name of the command a bare step runs, or "" if
// the function names none.
func DefaultCommand(content map[string]any) string {
	name, _ := content["default"].(string)
	return name
}

// Commands lists a function's top-level commands, sorted by name, each with
// the arguments a step naming it accepts. Only the top level is listed because
// a step can name only one command; that command's own default chain is
// followed for its arguments, as it is for a bare step.
func Commands(content map[string]any) []Command {
	raw, _ := content["commands"].(map[string]any)
	commands := make([]Command, 0, len(raw))
	for _, name := range slices.Sorted(maps.Keys(raw)) {
		node, ok := raw[name].(map[string]any)
		if !ok || name == "" {
			continue
		}
		flags := Flags(node)
		if flags == nil {
			flags = []Flag{}
		}
		commands = append(commands, Command{
			Name:        name,
			Description: str(node, "description"),
			Flags:       flags,
		})
	}
	return commands
}

// defaultCommand returns the command a node runs when invoked without one, or
// nil if it names none.
func defaultCommand(node map[string]any) map[string]any {
	commands, _ := node["commands"].(map[string]any)
	cmd, _ := commands[DefaultCommand(node)].(map[string]any)
	return cmd
}

func nodeFlags(node map[string]any) []Flag {
	raw, ok := node["flags"].([]any)
	if !ok {
		return nil
	}

	var flags []Flag
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		// Coercing the name the way a default is coerced would invent a flag
		// called "42".
		name, ok := m["name"].(string)
		if !ok || name == "" {
			continue
		}
		flags = append(flags, Flag{
			Name:        name,
			Type:        str(m, "type"),
			Default:     str(m, "default"),
			Description: str(m, "description"),
		})
	}
	return flags
}

// str reads a key as a display string. A bool default arrives as "true" from
// one publisher and true from another, so non-string scalars are formatted
// rather than dropped.
func str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		// JSON numbers decode as float64, which %v would print as 1e+06.
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// ExampleStep returns the step from a descriptor's example config, keyed by
// alias instead of the function's own name. An example is rendered when the
// function is released, so one that is missing or shaped unexpectedly yields
// "" rather than an error.
func ExampleStep(content map[string]any, alias string) string {
	example, _ := content["example"].(string)
	if example == "" {
		return ""
	}
	var doc yaml.Node
	if yaml.Unmarshal([]byte(example), &doc) != nil || len(doc.Content) == 0 {
		return ""
	}
	jobs := findMappingValue(doc.Content[0], "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode || len(jobs.Content) != 2 {
		return ""
	}
	steps := findMappingValue(jobs.Content[1], "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode || len(steps.Content) != 1 {
		return ""
	}

	step := steps.Content[0]
	switch {
	case step.Kind == yaml.ScalarNode:
		step.Value = alias
	case step.Kind == yaml.MappingNode && len(step.Content) == 2:
		step.Content[0].Value = alias
	default:
		return ""
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if enc.Encode(&yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{step}}) != nil || enc.Close() != nil {
		return ""
	}
	return buf.String()
}
