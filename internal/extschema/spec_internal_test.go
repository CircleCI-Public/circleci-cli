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
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestSpecsAreWellFormed(t *testing.T) {
	shortNameRE := regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	shorts := map[string]bool{}
	props := map[string]bool{}
	for _, s := range specs {
		inExt := map[string]bool{}
		for _, a := range s.attrs {
			assert.Check(t, shortNameRE.MatchString(a.short), "%s: short name %q", s.name, a.short)
			assert.Check(t, strings.HasPrefix(a.property, "ext_"), "%s: property %q", s.name, a.property)
			assert.Check(t, !inExt[a.short], "%s: duplicate short name %q", s.name, a.short)
			assert.Check(t, !shorts[a.short], "short name %q is used by two extensions", a.short)
			assert.Check(t, !props[a.property], "property %q is used twice", a.property)
			inExt[a.short] = true
			shorts[a.short] = true
			props[a.property] = true
		}
	}
}

func TestCheckBool(t *testing.T) {
	spec := extSpec{
		name:    "test-ext",
		version: 1,
		attrs: []attrSpec{
			{short: "flag", property: "ext_testext_flag", kind: kindBool, present: "optional"},
		},
	}
	tests := []struct {
		name      string
		raw       string
		want      Status
		wantProps map[string]any
	}{
		{
			name:      "true",
			raw:       `true`,
			want:      Accepted,
			wantProps: map[string]any{"ext_testext_flag": true},
		},
		{
			name:      "false",
			raw:       `false`,
			want:      Accepted,
			wantProps: map[string]any{"ext_testext_flag": false},
		},
		{
			name:      "true with whitespace",
			raw:       ` true `,
			want:      Accepted,
			wantProps: map[string]any{"ext_testext_flag": true},
		},
		{
			name:      "number 1",
			raw:       `1`,
			want:      Rejected,
			wantProps: map[string]any{},
		},
		{
			name:      "number 0",
			raw:       `0`,
			want:      Rejected,
			wantProps: map[string]any{},
		},
		{
			name:      "quoted string",
			raw:       `"true"`,
			want:      Rejected,
			wantProps: map[string]any{},
		},
		{
			name:      "null",
			raw:       `null`,
			want:      Rejected,
			wantProps: map[string]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkSpec(spec, 1, map[string]json.RawMessage{"flag": json.RawMessage(tt.raw)})
			assert.Check(t, cmp.Equal(res.Status, tt.want))
			got := map[string]any{}
			for _, a := range res.Accepted {
				got[a.Property] = a.Value
			}
			assert.Check(t, cmp.DeepEqual(got, tt.wantProps))
		})
	}
}

func TestExportBool(t *testing.T) {
	spec := extSpec{
		name:    "test-ext",
		version: 1,
		attrs: []attrSpec{
			{short: "flag", property: "ext_testext_flag", kind: kindBool, present: "optional"},
		},
	}
	out, err := exportSpecs([]extSpec{spec})
	assert.NilError(t, err)
	var doc map[string][]map[string]any
	err = json.Unmarshal(out, &doc)
	assert.NilError(t, err)
	attrs, ok := doc["extensions"][0]["attributes"].([]any)
	assert.Assert(t, ok)
	assert.Assert(t, cmp.Len(attrs, 1))
	assert.Check(t, cmp.DeepEqual(attrs[0], map[string]any{
		"short_name": "flag",
		"property":   "ext_testext_flag",
		"type":       "bool",
		"present":    "optional",
	}))
}
