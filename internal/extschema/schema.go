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

// Package extschema holds the schema for each extension's telemetry
// attributes, and checks a payload against it.
package extschema

// Status says what became of an extension's attribute payload.
type Status string

// The values a Status takes.
const (
	NotSupported      Status = "not_supported"
	SetupFailed       Status = "setup_failed"
	NoPayload         Status = "no_payload"
	Accepted          Status = "accepted"
	PartiallyAccepted Status = "partially_accepted"
	Rejected          Status = "rejected"
	ReadFailed        Status = "read_failed"
)

type attrKind int

const (
	kindInt attrKind = iota
	kindEnum
	kindBool
)

type attrSpec struct {
	short, property string
	kind            attrKind
	min, max        int      // kindInt, inclusive
	values          []string // kindEnum
	present         string   // text for the export
}

type extSpec struct {
	name    string
	version int
	attrs   []attrSpec // sorted by short
	// cross returns the short names to drop, given the attributes that
	// passed their own checks (short name to int, string or bool).
	cross func(survivors map[string]any) []string
}

// maxAttributes is the most attributes a payload may carry, counting every
// attribute received, including unknown names.
const maxAttributes = 20

// Property names are literals, never built at runtime, so the review of
// telemetry events can read them.
var specs = []extSpec{{
	name:    "configoptimizer",
	version: 1,
	cross:   crossCheckConfigOptimize,
	attrs: []attrSpec{
		{
			short: "jobs_analyzed", property: "ext_configoptimizer_jobs_analyzed",
			kind: kindInt, max: 10000, present: "optional",
		},
		{
			short: "jobs_with_suggestions", property: "ext_configoptimizer_jobs_with_suggestions",
			kind: kindInt, max: 10000, present: "optional",
		},
		{
			short: "no_suggestion_reason", property: "ext_configoptimizer_no_suggestion_reason", kind: kindEnum,
			values: []string{
				"not_enough_usage", "saving_too_small", "already_right_sized", "memory_bound",
				"pipeline_not_priced", "unsupported_jobs", "class_mismatch", "previously_rejected",
			},
			present: "optional; only when suggestions is 0",
		},
		{short: "suggestions", property: "ext_configoptimizer_suggestions", kind: kindInt, max: 10000, present: "optional"},
		{
			short: "validation_outcome", property: "ext_configoptimizer_validation_outcome", kind: kindEnum,
			values: []string{
				"savings_confirmed", "savings_not_realized", "failed_on_suggested_hardware", "error", "inconclusive",
			},
			present: "optional",
		},
		{
			short: "validation_pairs", property: "ext_configoptimizer_validation_pairs",
			kind: kindInt, max: 100, present: "optional",
		},
	},
}}
