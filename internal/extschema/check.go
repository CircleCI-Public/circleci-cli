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

package extschema

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
)

// Attribute is an attribute that passed the check.
type Attribute struct {
	Property string // literal, such as ext_configoptimizer_suggestions
	Value    any    // int, string or bool
}

// Result is the outcome of Check.
type Result struct {
	Status   Status
	Accepted []Attribute // ordered by short name
}

var intRE = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

func lookup(extension string) (extSpec, bool) {
	for _, s := range specs {
		if s.name == extension {
			return s, true
		}
	}
	return extSpec{}, false
}

// Has reports whether a schema exists for the extension.
func Has(extension string) bool {
	_, ok := lookup(extension)
	return ok
}

// ShortNames returns the extension's short attribute names, sorted. It
// returns nil for an unknown extension.
func ShortNames(extension string) []string {
	s, ok := lookup(extension)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(s.attrs))
	for _, a := range s.attrs {
		names = append(names, a.short)
	}
	return names
}

// Check returns the attributes that passed the extension's schema, with their
// property names, and a status. It returns NotSupported for an extension with
// no schema, and otherwise only Accepted, PartiallyAccepted and Rejected.
func Check(extension string, version int, attributes map[string]json.RawMessage) Result {
	spec, ok := lookup(extension)
	if !ok {
		return Result{Status: NotSupported}
	}
	return checkSpec(spec, version, attributes)
}

// checkSpec checks a payload against one extension's schema.
func checkSpec(spec extSpec, version int, attributes map[string]json.RawMessage) Result {
	if version != spec.version || len(attributes) > maxAttributes {
		return Result{Status: Rejected}
	}
	if len(attributes) == 0 {
		return Result{Status: Accepted}
	}

	survivors := map[string]any{}
	for _, a := range spec.attrs {
		raw, present := attributes[a.short]
		if !present {
			continue
		}
		if v, ok := a.parse(raw); ok {
			survivors[a.short] = v
		}
	}

	if spec.cross != nil {
		for _, short := range spec.cross(survivors) {
			delete(survivors, short)
		}
	}

	var accepted []Attribute
	for _, a := range spec.attrs {
		if v, ok := survivors[a.short]; ok {
			accepted = append(accepted, Attribute{Property: a.property, Value: v})
		}
	}
	switch {
	case len(accepted) == 0:
		return Result{Status: Rejected}
	case len(accepted) == len(attributes):
		return Result{Status: Accepted, Accepted: accepted}
	default:
		return Result{Status: PartiallyAccepted, Accepted: accepted}
	}
}

// parse reads one attribute value and reports whether it is valid.
func (a attrSpec) parse(raw json.RawMessage) (any, bool) {
	switch a.kind {
	case kindInt:
		text := bytes.TrimSpace(raw)
		if !intRE.Match(text) {
			return nil, false
		}
		n, err := strconv.Atoi(string(text))
		if err != nil || n < a.min || n > a.max {
			return nil, false
		}
		return n, true
	case kindBool:
		switch string(bytes.TrimSpace(raw)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
		return nil, false
	case kindEnum:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || !slices.Contains(a.values, s) {
			return nil, false
		}
		return s, true
	}
	return nil, false
}

// crossCheckConfigOptimize returns the short names that fail a check across
// attributes. The first two checks each read the survivors of the per-attribute
// step, never the other's result, so their order does not matter. The
// no_suggestion_reason check runs last, on what those two leave, so a reason
// never outlives its suggestions.
func crossCheckConfigOptimize(survivors map[string]any) []string {
	var drop []string
	suggestions, hasSuggestions := survivors["suggestions"].(int)
	analyzed, hasAnalyzed := survivors["jobs_analyzed"].(int)
	withSuggestions, hasWith := survivors["jobs_with_suggestions"].(int)

	if hasAnalyzed && hasWith && withSuggestions > analyzed {
		drop = append(drop, "jobs_analyzed", "jobs_with_suggestions")
	}
	if hasSuggestions && hasWith && (suggestions == 0) != (withSuggestions == 0) {
		drop = append(drop, "suggestions", "jobs_with_suggestions")
	}

	_, hasReason := survivors["no_suggestion_reason"]
	suggestionsLeft := hasSuggestions && !slices.Contains(drop, "suggestions")
	if hasReason && (!suggestionsLeft || suggestions != 0) {
		drop = append(drop, "no_suggestion_reason")
	}
	return drop
}
