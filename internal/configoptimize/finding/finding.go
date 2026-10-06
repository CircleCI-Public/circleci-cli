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

// Package finding defines what a module reports. A finding is never an
// edit: it states what is true and, structurally, what change would follow.
// Reconcile decides what happens to it; the patch planner owns the YAML.
package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Finding is one module result.
type Finding struct {
	// ID is stable across runs for the same module and target, and does not
	// change when the targeted value changes. See NewID.
	ID      string
	Module  string
	Verdict Verdict
	Target  Target

	Impact     Impact
	Confidence Confidence
	// Evidence makes a finding auditable. A finding with no evidence is a bug.
	Evidence []Evidence

	Suggestion Suggestion

	// Rank orders a module's findings, lowest first (0 when unranked), so
	// the most severe come first; ties fall back to the target order.
	Rank int

	// Codes are module-defined reason codes, e.g. "TEST_RUNNER". They are
	// metadata for ranking and filtering, never a disposition.
	Codes []string
	// Metrics are a module's numbers behind the verdict, by name, so tools
	// read values rather than parse the evidence text (e.g. resourceclass's
	// "pipeline_saving"). Shares are fractions, not percentages.
	Metrics map[string]float64
	// Keys are identifiers a tool matches findings on across runs, by name
	// (e.g. resourceclass's "fingerprint" of the job apart from its class).
	Keys map[string]string
}

// Verdict is module-specific, e.g. "DLC_UNUSED".
type Verdict string

// Target is what a finding is about, located in the EFFECTIVE config.
type Target struct {
	// Path into the effective config, e.g.
	// ["jobs", "build", "machine", "docker_layer_caching"].
	Path []string
	// Job is the effective job name, "" if not job-scoped.
	Job string
	// Workflow is "" if not workflow-scoped.
	Workflow string
	// Site is the authored node the finding is about, when it is not where
	// the effective path points: for a parameterized job, the workflow call
	// site that supplies the value. It is not part of the ID.
	Site []string
}

// PathString renders Path with dots, for reports and sorting.
func (t Target) PathString() string {
	return strings.Join(t.Path, ".")
}

// NewID derives a finding ID from the module and target. The targeted value
// is deliberately not an input: a finding about DLC on job "build" must keep
// its ID whether the value is true today or changes tomorrow. A renamed job
// or a different matrix expansion yields a new ID.
func NewID(module string, target Target) string {
	h := sha256.New()
	parts := append([]string{module, target.Job, target.Workflow}, target.Path...)
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return module + ":" + hex.EncodeToString(h.Sum(nil))[:12]
}

// Impact carries time and cost independently, because they diverge.
type Impact struct {
	Time Direction
	Cost Direction

	// Referent says what the directions and figures describe: a
	// proposed change, the current state, or a rejected option. Findings with
	// different referents must never be summed together.
	Referent Referent

	// CostCredits is the estimated credit delta over Period, negative for a
	// saving. It is a rate delta at constant duration. Nil when the data does
	// not support a figure — never fabricate one.
	CostCredits *float64
	// Period is the window CostCredits covers, e.g. "14d". Required whenever
	// CostCredits is set.
	Period string

	// UnitCredits is a per-unit charge the finding is about, when known, e.g.
	// a flat per-run fee. Unit names it, e.g. "job run". It lets a report say
	// "N findings x R credits per job run" without a run count.
	UnitCredits *float64
	Unit        string

	// Basis names what any figure was computed from, and its assumptions.
	Basis string
}

// FormatCredits writes a credit amount in its shortest exact form, e.g. 200
// or 0.5.
func FormatCredits(c float64) string {
	return strconv.FormatFloat(c, 'f', -1, 64)
}

// Direction is the expected movement on one axis.
type Direction int

const (
	// DirectionUnknown means not evaluated.
	DirectionUnknown Direction = iota
	// DirectionBetter is faster, or cheaper.
	DirectionBetter
	// DirectionNeutral means evaluated and found not to move.
	DirectionNeutral
	// DirectionWorse is slower, or pricier.
	DirectionWorse
)

// String returns the lowercase name used in reports and JSON.
func (d Direction) String() string {
	switch d {
	case DirectionUnknown:
		return "unknown"
	case DirectionBetter:
		return "better"
	case DirectionNeutral:
		return "neutral"
	case DirectionWorse:
		return "worse"
	}
	return "unknown"
}

// Referent is what an Impact describes.
type Referent int

const (
	// ReferentProposedChange describes the effect of the suggested change.
	ReferentProposedChange Referent = iota
	// ReferentCurrentState describes measured state as it is today.
	ReferentCurrentState
)

// String returns the lowercase name used in reports and JSON.
func (r Referent) String() string {
	switch r {
	case ReferentProposedChange:
		return "proposed_change"
	case ReferentCurrentState:
		return "current_state"
	}
	return "proposed_change"
}

// Confidence says how far a finding can be trusted.
type Confidence struct {
	Level ConfidenceLevel
	// Reason is required for anything below High.
	Reason string
}

// ConfidenceLevel orders confidence from Low to High.
type ConfidenceLevel int

const (
	// ConfidenceLow is thin data, an uninspectable input, or a value near a
	// threshold not yet calibrated against measured runs.
	ConfidenceLow ConfidenceLevel = iota
	// ConfidenceMedium is some evidence, not conclusive.
	ConfidenceMedium
	// ConfidenceHigh is a static config fact, or ample telemetry.
	ConfidenceHigh
)

// String returns the lowercase name used in reports and JSON.
func (c ConfidenceLevel) String() string {
	switch c {
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	}
	return "low"
}

// Evidence is one auditable fact behind a finding.
type Evidence struct {
	Kind   EvidenceKind
	Claim  string // "no step invokes docker build"
	Value  string // "5 steps inspected"
	Source string // "config:jobs.build.steps"
}

// EvidenceKind says where a piece of evidence came from.
type EvidenceKind int

const (
	// EvidenceConfigFact is read directly from the effective config.
	EvidenceConfigFact EvidenceKind = iota
	// EvidenceTelemetry comes from the data view.
	EvidenceTelemetry
	// EvidenceDerived is computed from other evidence.
	EvidenceDerived
)

// String returns the lowercase name used in reports and JSON.
func (k EvidenceKind) String() string {
	switch k {
	case EvidenceConfigFact:
		return "config_fact"
	case EvidenceTelemetry:
		return "telemetry"
	case EvidenceDerived:
		return "derived"
	}
	return "config_fact"
}

// Suggestion is the module's intent, expressed structurally.
type Suggestion struct {
	Op ChangeOp
	// Paths are the effective nodes the change touches, when it touches more
	// than Target.Path. The change is all or nothing: it is actionable
	// only if every path is writable. Empty means just Target.Path.
	Paths [][]string
	// Value for Set and Add, as a plain Go value.
	Value any
	// Note is carried into the report verbatim.
	Note string
	// FalseMeansAbsent says the removed keys are booleans whose default is
	// false, so an explicit false left by the compiler counts as removed.
	FalseMeansAbsent bool
	// Authored names the authored nodes an OpSet changes, when the module
	// traced them itself (a value set by a parameter at a workflow call
	// site, which the mutability map does not resolve). Empty means reconcile
	// resolves Paths the usual way.
	Authored []AuthoredSet
	// Compiled are the effective paths and values the change must produce,
	// when they differ from Paths set to Value.
	Compiled []CompiledSet
	// Override lets an OpSet land on the job itself when the value is
	// inherited from a shared executor or a merge key: the job gets a key of
	// its own and the shared source is left alone. Only for a key
	// directly under the job.
	Override bool
	// FromUsage says the change rests on measured usage of real runs, not on
	// the compiled steps. Such a finding is not held back only because the
	// job calls a registry orb command whose body is unseen, since the runs
	// executed it; visible pipeline-value dependence still holds it back.
	FromUsage bool
}

// AuthoredSet is one authored scalar and the value it gets.
type AuthoredSet struct {
	Path   []string
	Expect any
	Value  any
}

// CompiledSet is one effective path and the value it must hold afterwards.
type CompiledSet struct {
	Path  []string
	Value any
}

// ChangePaths returns the effective paths a finding's change touches.
func (f Finding) ChangePaths() [][]string {
	if len(f.Suggestion.Paths) > 0 {
		return f.Suggestion.Paths
	}
	return [][]string{f.Target.Path}
}

// ChangeOp is the kind of change a finding suggests.
type ChangeOp int

const (
	// OpNone proposes nothing: the finding is informational.
	OpNone ChangeOp = iota
	// OpRemove deletes a node.
	OpRemove
	// OpSet replaces a scalar.
	OpSet
	// OpAdd inserts nodes.
	OpAdd
)

// String returns the lowercase name used in reports and JSON.
func (o ChangeOp) String() string {
	switch o {
	case OpNone:
		return "none"
	case OpRemove:
		return "remove"
	case OpSet:
		return "set"
	case OpAdd:
		return "add"
	}
	return "none"
}

// Disposition is the one exclusive outcome of a finding. Modules
// never set it; reconcile does. The lifecycle is
// detected -> {informational | actionable} -> planned ->
// {applied | rejected | report-only}.
type Disposition int

const (
	// DispositionActionable is a change that can be made (analyze).
	DispositionActionable Disposition = iota
	// DispositionReportOnly is reported but not changed.
	DispositionReportOnly
	// DispositionApplied was applied and compiled (apply).
	DispositionApplied
	// DispositionRejected was applied and failed to compile (apply).
	DispositionRejected
)

// String returns the lowercase name used in reports and JSON.
func (d Disposition) String() string {
	switch d {
	case DispositionActionable:
		return "actionable"
	case DispositionReportOnly:
		return "report_only"
	case DispositionApplied:
		return "applied"
	case DispositionRejected:
		return "rejected"
	}
	return "report_only"
}
