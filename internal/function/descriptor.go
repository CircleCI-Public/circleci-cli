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

// Flags reads the arguments a function accepts. A descriptor is served exactly
// as published and its shape can differ between versions, so a missing or
// unexpectedly typed field degrades to a thinner result rather than an error.
func Flags(content map[string]any) []Flag {
	raw, ok := content["flags"].([]any)
	if !ok {
		return nil
	}

	flags := make([]Flag, 0, len(raw))
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
	if len(flags) == 0 {
		return nil
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
