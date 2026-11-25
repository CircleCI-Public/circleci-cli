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

package extschema_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/extschema"
)

const prefix = "ext_configoptimizer_"

// attrs builds an attribute map from name, raw JSON pairs.
func attrs(pairs ...string) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = json.RawMessage(pairs[i+1])
	}
	return m
}

// with returns base plus the extra pairs.
func with(base map[string]json.RawMessage, pairs ...string) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	for k, v := range base {
		m[k] = v
	}
	for k, v := range attrs(pairs...) {
		m[k] = v
	}
	return m
}

func manyUnknown(n int) []string {
	var pairs []string
	for i := 1; i <= n; i++ {
		pairs = append(pairs, "x"+strconv.Itoa(i), "1")
	}
	return pairs
}

func TestCheck(t *testing.T) {
	valid := attrs("suggestions", "3", "jobs_analyzed", "10", "jobs_with_suggestions", "2")
	validProps := map[string]any{
		prefix + "suggestions":           3,
		prefix + "jobs_analyzed":         10,
		prefix + "jobs_with_suggestions": 2,
	}
	none := map[string]any{}

	tests := []struct {
		name      string
		extension string
		version   int
		attrs     map[string]json.RawMessage
		want      extschema.Status
		wantProps map[string]any
	}{
		{
			name:      "worked example",
			extension: "configoptimizer",
			version:   1,
			attrs:     valid,
			want:      extschema.Accepted,
			wantProps: validProps,
		},
		{
			name:      "nil attributes",
			extension: "configoptimizer",
			version:   1,
			want:      extschema.Accepted,
			wantProps: none,
		},
		{
			name:      "empty attributes",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs(),
			want:      extschema.Accepted,
			wantProps: none,
		},
		{
			name:      "version 0",
			extension: "configoptimizer",
			attrs:     valid,
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "version 2",
			extension: "configoptimizer",
			version:   2,
			attrs:     valid,
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "unknown extension",
			extension: "other-ext",
			version:   1,
			attrs:     valid,
			want:      extschema.NotSupported,
			wantProps: none,
		},
		{
			name:      "exactly 20 attributes",
			extension: "configoptimizer",
			version:   1,
			attrs:     with(valid, manyUnknown(17)...),
			want:      extschema.PartiallyAccepted,
			wantProps: validProps,
		},
		{
			name:      "21 attributes",
			extension: "configoptimizer",
			version:   1,
			attrs:     with(valid, manyUnknown(18)...),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "uppercase name alone",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("Bad-Name", "1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "33 character name alone",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs(strings.Repeat("a", 33), "1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "empty name alone",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("", "1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "unknown well formed name alone",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("made_up", "1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "bad names beside valid",
			extension: "configoptimizer",
			version:   1,
			attrs:     with(valid, "Bad-Name", "1", strings.Repeat("a", 33), "1", "", "1", "made_up", "1"),
			want:      extschema.PartiallyAccepted,
			wantProps: validProps,
		},
		{
			name:      "float 3.5",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "3.5"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "float 3.0",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "3.0"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "exponent 1e2",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "1e2"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "negative zero",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "-0"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "negative one",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "-1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "leading zero",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "01"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "leading plus",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "+1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "string",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", `"3"`),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "null",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "null"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "bool",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "true"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "overflow",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "99999999999999999999"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "lower bound 0",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "0"),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "jobs_analyzed": 0},
		},
		{
			name:      "upper bound 10000",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "10000"),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "jobs_analyzed": 10000},
		},
		{
			name:      "above upper bound 10001",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "10001"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "validation_pairs 100",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_pairs", "100"),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "validation_pairs": 100},
		},
		{
			name:      "validation_pairs 101",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_pairs", "101"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "no_suggestion_reason valid",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`, "suggestions", "0", "jobs_with_suggestions", "0"),
			want:      extschema.Accepted,
			wantProps: map[string]any{
				prefix + "no_suggestion_reason":  "memory_bound",
				prefix + "suggestions":           0,
				prefix + "jobs_with_suggestions": 0,
			},
		},
		{
			name:      "no_suggestion_reason unknown value",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"nope"`, "suggestions", "0"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "suggestions": 0},
		},
		{
			name:      "validation_outcome valid",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_outcome", `"inconclusive"`),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "validation_outcome": "inconclusive"},
		},
		{
			name:      "validation_outcome wrong case",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_outcome", `"Inconclusive"`),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "validation_outcome null",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_outcome", "null"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "validation_outcome number",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("validation_outcome", "1"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "more with suggestions than analyzed",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "10", "jobs_with_suggestions", "20"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "more with suggestions than analyzed beside a third",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("jobs_analyzed", "10", "jobs_with_suggestions", "20", "validation_pairs", "5"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "validation_pairs": 5},
		},
		{
			name:      "suggestions without jobs",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("suggestions", "3", "jobs_with_suggestions", "0"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "jobs without suggestions",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("suggestions", "0", "jobs_with_suggestions", "2"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "suggestions 0 alone",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("suggestions", "0"),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "suggestions": 0},
		},
		{
			name:      "reason with suggestions 3",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`, "suggestions", "3"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "suggestions": 3},
		},
		{
			name:      "reason with suggestions absent",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "reason with suggestions absent beside a third",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`, "validation_pairs", "5"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "validation_pairs": 5},
		},
		{
			name:      "reason with suggestions 0",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`, "suggestions", "0"),
			want:      extschema.Accepted,
			wantProps: map[string]any{prefix + "no_suggestion_reason": "memory_bound", prefix + "suggestions": 0},
		},
		{
			name:      "reason with suggestions out of range",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("no_suggestion_reason", `"memory_bound"`, "suggestions", "99999"),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "reason with suggestions dropped by the jobs check",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("suggestions", "5", "jobs_with_suggestions", "0", "no_suggestion_reason", `"saving_too_small"`),
			want:      extschema.Rejected,
			wantProps: none,
		},
		{
			name:      "reason with suggestions dropped by the jobs check beside a third",
			extension: "configoptimizer",
			version:   1,
			attrs: attrs(
				"suggestions", "5", "jobs_with_suggestions", "0", "no_suggestion_reason", `"saving_too_small"`,
				"validation_pairs", "5"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "validation_pairs": 5},
		},
		{
			name:      "reason with suggestions 0 dropped by the jobs check",
			extension: "configoptimizer",
			version:   1,
			attrs: attrs(
				"suggestions", "0", "jobs_with_suggestions", "2", "no_suggestion_reason", `"saving_too_small"`,
				"validation_pairs", "5"),
			want:      extschema.PartiallyAccepted,
			wantProps: map[string]any{prefix + "validation_pairs": 5},
		},
		{
			name:      "order independence",
			extension: "configoptimizer",
			version:   1,
			attrs:     attrs("suggestions", "0", "jobs_with_suggestions", "5", "jobs_analyzed", "2"),
			want:      extschema.Rejected,
			wantProps: none,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := extschema.Check(tt.extension, tt.version, tt.attrs)
			assert.Check(t, cmp.Equal(res.Status, tt.want))
			got := map[string]any{}
			for _, a := range res.Accepted {
				got[a.Property] = a.Value
			}
			assert.Check(t, cmp.DeepEqual(got, tt.wantProps))
		})
	}
}

func TestCheckAcceptedOrder(t *testing.T) {
	res := extschema.Check("configoptimizer", 1, map[string]json.RawMessage{
		"suggestions":           json.RawMessage(`3`),
		"jobs_analyzed":         json.RawMessage(`10`),
		"jobs_with_suggestions": json.RawMessage(`2`),
	})
	got := make([]string, 0, len(res.Accepted))
	for _, a := range res.Accepted {
		got = append(got, a.Property)
	}
	assert.Check(t, cmp.DeepEqual(got, []string{
		"ext_configoptimizer_jobs_analyzed",
		"ext_configoptimizer_jobs_with_suggestions",
		"ext_configoptimizer_suggestions",
	}))
}

func TestHas(t *testing.T) {
	tests := []struct {
		extension string
		want      bool
	}{
		{
			extension: "configoptimizer",
			want:      true,
		},
		{
			extension: "other-ext",
		},
	}
	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			assert.Check(t, cmp.Equal(extschema.Has(tt.extension), tt.want))
		})
	}
	assert.Check(t, cmp.Nil(extschema.ShortNames("other-ext")))
}
