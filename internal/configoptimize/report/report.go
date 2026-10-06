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

// Package report renders the result of a run: the --json document, and the
// same content as text. No credit rate appears here: figures come from
// the findings, which got them from pricing.Provider.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
)

// SchemaVersion is the --json contract version. Bump it on any change a
// consumer could notice.
const SchemaVersion = 1

// The two modes of config optimize, as the report's command field: report
// (no output flag) and write (-o or --in-place).
const (
	CommandReport = "report"
	CommandWrite  = "write"
)

// Document is the --json output.
type Document struct {
	SchemaVersion int       `json:"schema_version"`
	Command       string    `json:"command"`
	Input         Input     `json:"input"`
	Expansion     Expansion `json:"expansion"`
	Compiler      string    `json:"compiler"`
	Modules       []string  `json:"modules"`
	Summary       Summary   `json:"summary"`
	Findings      []Finding `json:"findings"`
}

// Input identifies the config that was analyzed.
type Input struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Expansion records which compilation produced the findings, since findings
// hold for one set of pipeline inputs.
type Expansion struct {
	ID                 string         `json:"id"`
	PipelineParameters map[string]any `json:"pipeline_parameters"`
	// Branch says where pipeline.* values came from.
	Branch string `json:"branch"`
}

// Summary counts findings per report group. Every finding is in exactly one.
type Summary struct {
	Actionable int `json:"actionable"`
	ReportOnly int `json:"report_only"`
	// Applied and Rejected are set by apply only.
	Applied  int `json:"applied,omitempty"`
	Rejected int `json:"rejected,omitempty"`
	// Result is the run's outcome: one of the Result* codes.
	Result string `json:"result"`
	// MissingInputs are inputs a selected module needed and did not get.
	MissingInputs []string `json:"missing_inputs,omitempty"`
}

// Result codes. A candidate is a config for real runs to judge,
// not a proven saving.
const (
	// ResultCandidateProduced: at least one change was applied (apply), or
	// can be (analyze).
	ResultCandidateProduced = "candidate_produced"
	// ResultNoCandidate: the enabled modules found no applicable change with
	// the supplied inputs.
	ResultNoCandidate = "no_candidate"
	// ResultAllRejected: apply planned changes, and the compile check
	// rejected every one.
	ResultAllRejected = "all_candidates_rejected"
	// ResultMissingInput: nothing applicable, and a selected module lacked an
	// input it needs.
	ResultMissingInput = "missing_required_input"
)

// Finding is one finding with its disposition.
type Finding struct {
	ID            string   `json:"id"`
	Module        string   `json:"module"`
	Verdict       string   `json:"verdict"`
	Disposition   string   `json:"disposition"`
	Reason        string   `json:"reason,omitempty"`
	Mutability    string   `json:"mutability,omitempty"`
	Expansion     string   `json:"expansion"`
	Target        Target   `json:"target"`
	Locations     []string `json:"locations,omitempty"`
	AuthoredPaths []string `json:"authored_paths,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
	// Metrics are the module's numbers behind the verdict.
	Metrics map[string]float64 `json:"metrics,omitempty"`
	// Keys identify what the finding is about across runs.
	Keys       map[string]string `json:"keys,omitempty"`
	Impact     Impact            `json:"impact"`
	Confidence Confidence        `json:"confidence"`
	Evidence   []Evidence        `json:"evidence"`
	Suggestion Suggestion        `json:"suggestion"`
}

// Target is where the finding is, in the effective config.
type Target struct {
	Job  string `json:"job,omitempty"`
	Path string `json:"path"`
	// Site is the authored node the finding is about, e.g. the workflow call
	// site that passes a volatile cache key.
	Site string `json:"site,omitempty"`
}

// Impact mirrors finding.Impact with names for the enums.
type Impact struct {
	Time        string   `json:"time"`
	Cost        string   `json:"cost"`
	Referent    string   `json:"referent"`
	CostCredits *float64 `json:"cost_credits,omitempty"`
	Period      string   `json:"period,omitempty"`
	UnitCredits *float64 `json:"unit_credits,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Basis       string   `json:"basis"`
}

// Confidence mirrors finding.Confidence.
type Confidence struct {
	Level  string `json:"level"`
	Reason string `json:"reason,omitempty"`
}

// Evidence mirrors finding.Evidence.
type Evidence struct {
	Kind   string `json:"kind"`
	Claim  string `json:"claim"`
	Value  string `json:"value,omitempty"`
	Source string `json:"source"`
}

// Suggestion mirrors finding.Suggestion.
type Suggestion struct {
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Meta is the run context for Build.
type Meta struct {
	Command            string
	InputPath          string
	Input              []byte
	PipelineParameters map[string]any
	Compiler           string
	Modules            []string
	// MissingInputs are inputs a selected module needed and did not get.
	MissingInputs []string
}

// branchSource says where the compile's branch and other pipeline values
// come from.
const branchSource = "from the git checkout of the working directory"

// Build assembles the document. Entries must already be in report order.
func Build(meta Meta, entries []reconcile.Entry) Document {
	sum := sha256.Sum256(meta.Input)
	params := meta.PipelineParameters
	if params == nil {
		params = map[string]any{}
	}
	doc := Document{
		SchemaVersion: SchemaVersion,
		Command:       meta.Command,
		Input:         Input{Path: meta.InputPath, SHA256: hex.EncodeToString(sum[:])},
		Expansion:     Expansion{ID: expansionID(params), PipelineParameters: params, Branch: branchSource},
		Compiler:      meta.Compiler,
		Modules:       meta.Modules,
		Findings:      make([]Finding, 0, len(entries)),
	}
	for _, e := range entries {
		switch e.Disposition {
		case finding.DispositionActionable:
			doc.Summary.Actionable++
		case finding.DispositionApplied:
			doc.Summary.Applied++
		case finding.DispositionRejected:
			doc.Summary.Rejected++
		case finding.DispositionReportOnly:
			doc.Summary.ReportOnly++
		}
		doc.Findings = append(doc.Findings, toFinding(e, doc.Expansion.ID))
	}
	doc.Summary.MissingInputs = meta.MissingInputs
	doc.Summary.Result = outcome(doc.Summary)
	return doc
}

// outcome picks the result code: a candidate if any change was applied or
// can be; then missing input, since supplying it could change the result;
// then all-rejected; then nothing found. A produced candidate still lists
// missing inputs in the summary.
func outcome(s Summary) string {
	switch {
	case s.Applied > 0 || s.Actionable > 0:
		return ResultCandidateProduced
	case len(s.MissingInputs) > 0:
		return ResultMissingInput
	case s.Rejected > 0:
		return ResultAllRejected
	}
	return ResultNoCandidate
}

// resultLine is the human form of the result code.
func resultLine(doc Document) string {
	s := doc.Summary
	switch s.Result {
	case ResultCandidateProduced:
		if doc.Command == CommandWrite {
			return fmt.Sprintf("Result: candidate produced (%d %s). Validate it with real runs before keeping it.", s.Applied, plural(s.Applied, "change"))
		}
		return fmt.Sprintf("Result: a candidate can be produced (%d %s); -o FILE writes it.", s.Actionable, plural(s.Actionable, "change"))
	case ResultAllRejected:
		return fmt.Sprintf("Result: no candidate: the compile check rejected all %d %s.", s.Rejected, plural(s.Rejected, "change"))
	case ResultMissingInput:
		return "Result: no candidate: missing " + strings.Join(s.MissingInputs, "; ") + "."
	}
	return "Result: no candidate: the enabled modules found no applicable change with the supplied inputs."
}

func them(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

func their(n int) string {
	if n == 1 {
		return "its"
	}
	return "their"
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// expansionID identifies the parameter set a compilation used.
func expansionID(params map[string]any) string {
	b, err := json.Marshal(params) // map keys marshal sorted
	if err != nil {
		b = []byte(fmt.Sprint(params))
	}
	sum := sha256.Sum256(append(b, []byte(branchSource)...))
	return hex.EncodeToString(sum[:])[:12]
}

func toFinding(e reconcile.Entry, expansion string) Finding {
	f := e.Finding
	out := Finding{
		ID:          f.ID,
		Module:      f.Module,
		Verdict:     string(f.Verdict),
		Disposition: e.Disposition.String(),
		Reason:      e.Reason,
		Mutability:  string(e.Mutability),
		Expansion:   expansion,
		Target:      Target{Job: f.Target.Job, Path: f.Target.PathString(), Site: strings.Join(f.Target.Site, ".")},
		Impact: Impact{
			Time:        f.Impact.Time.String(),
			Cost:        f.Impact.Cost.String(),
			Referent:    f.Impact.Referent.String(),
			CostCredits: f.Impact.CostCredits,
			Period:      f.Impact.Period,
			UnitCredits: f.Impact.UnitCredits,
			Unit:        f.Impact.Unit,
			Basis:       f.Impact.Basis,
		},
		Confidence: Confidence{Level: f.Confidence.Level.String(), Reason: f.Confidence.Reason},
		Evidence:   make([]Evidence, 0, len(f.Evidence)),
		Suggestion: Suggestion{Op: f.Suggestion.Op.String(), Value: f.Suggestion.Value, Note: f.Suggestion.Note},
	}
	if f.Suggestion.Op != finding.OpNone {
		for _, p := range f.ChangePaths() {
			out.Locations = append(out.Locations, strings.Join(p, "."))
		}
	}
	for _, a := range e.Authored {
		out.AuthoredPaths = append(out.AuthoredPaths, strings.Join(a.Path, "."))
	}
	out.ReasonCodes = f.Codes
	out.Metrics = f.Metrics
	out.Keys = f.Keys
	for _, ev := range f.Evidence {
		out.Evidence = append(out.Evidence, Evidence{Kind: ev.Kind.String(), Claim: ev.Claim, Value: ev.Value, Source: ev.Source})
	}
	return out
}

// TextOptions shape the human report.
type TextOptions struct {
	// Verbose lists report-only findings, and every finding's
	// impact, confidence and evidence. Quiet, the default, lists only the
	// changes and counts the rest. --json is never shortened.
	Verbose bool
}

// WriteText writes the human report: every finding is counted in one group,
// and listed when it is a change or the report is verbose.
func WriteText(w io.Writer, doc Document, opts TextOptions) error {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "config optimize: %s\n", doc.Input.Path)
	_, _ = fmt.Fprintf(&b, "compiler: %s\n", doc.Compiler)
	_, _ = fmt.Fprintf(&b, "expansion %s: pipeline parameters %s; pipeline values %s\n",
		doc.Expansion.ID, describeParams(doc.Expansion.PipelineParameters), doc.Expansion.Branch)
	_, _ = fmt.Fprintf(&b, "modules: %s\n", strings.Join(doc.Modules, ", "))

	type group struct {
		title       string
		disposition string
		change      bool // listed even when the report is quiet
	}
	groups := []group{
		{"Changes that can be made", finding.DispositionActionable.String(), true},
		{"Findings that can only be reported", finding.DispositionReportOnly.String(), false},
	}
	if doc.Command == CommandWrite {
		groups = append([]group{
			{"Changes applied", finding.DispositionApplied.String(), true},
			{"Changes rejected", finding.DispositionRejected.String(), true},
		}, groups[1:]...)
	}
	hidden := 0
	for _, g := range groups {
		var members []Finding
		for _, f := range doc.Findings {
			if f.Disposition == g.disposition {
				members = append(members, f)
			}
		}
		if !g.change && !opts.Verbose {
			hidden += len(members)
			continue
		}
		_, _ = fmt.Fprintf(&b, "\n%s (%d)\n", g.title, len(members))
		if len(members) == 0 {
			b.WriteString("  none\n")
		}
		for _, f := range members {
			writeFinding(&b, f, opts.Verbose)
		}
	}
	if hidden > 0 {
		_, _ = fmt.Fprintf(&b, "\n%d other %s not listed; --verbose lists %s with %s suggestions.\n",
			hidden, plural(hidden, "finding"), them(hidden), their(hidden))
	}

	if doc.Command == CommandWrite {
		_, _ = fmt.Fprintf(&b, "\nSummary: %d applied, %d rejected, %d report-only\n",
			doc.Summary.Applied, doc.Summary.Rejected, doc.Summary.ReportOnly)
	} else {
		_, _ = fmt.Fprintf(&b, "\nSummary: %d actionable, %d report-only\n",
			doc.Summary.Actionable, doc.Summary.ReportOnly)
	}
	for _, line := range unitTotals(doc.Findings) {
		_, _ = fmt.Fprintf(&b, "  %s\n", line)
	}
	if line := codeRanking(doc.Findings); line != "" && opts.Verbose {
		_, _ = fmt.Fprintf(&b, "  %s\n", line)
	}
	_, _ = fmt.Fprintf(&b, "\n%s\n", resultLine(doc))
	_, err := io.WriteString(w, b.String())
	return err
}

// writeFinding writes one finding: what it is and what changes, and when
// verbose also its impact, confidence and evidence.
func writeFinding(b *strings.Builder, f Finding, verbose bool) {
	_, _ = fmt.Fprintf(b, "  %s  %s  [%s]\n", f.Verdict, f.Target.Path, f.ID)
	if f.Target.Site != "" {
		_, _ = fmt.Fprintf(b, "    authored at: %s\n", f.Target.Site)
	}
	if f.Suggestion.Op != finding.OpNone.String() {
		label := "change"
		paths := f.AuthoredPaths
		if f.Disposition == finding.DispositionApplied.String() {
			label = "changed"
		} else if f.Disposition != finding.DispositionActionable.String() {
			label = "would change (not made)"
			paths = f.Locations
		}
		_, _ = fmt.Fprintf(b, "    %s: %s %s\n", label, f.Suggestion.Op, strings.Join(paths, ", "))
	}
	if len(f.ReasonCodes) > 0 {
		_, _ = fmt.Fprintf(b, "    reason codes: %s\n", strings.Join(f.ReasonCodes, ", "))
	}
	if f.Reason != "" {
		prefix := "reported only"
		if f.Disposition == finding.DispositionRejected.String() {
			prefix = "rejected"
		}
		_, _ = fmt.Fprintf(b, "    %s: %s\n", prefix, f.Reason)
	}
	if f.Suggestion.Note != "" {
		_, _ = fmt.Fprintf(b, "    %s\n", f.Suggestion.Note)
	}
	if !verbose {
		return
	}
	_, _ = fmt.Fprintf(b, "    cost: %s, time: %s, confidence: %s", f.Impact.Cost, f.Impact.Time, f.Confidence.Level)
	if f.Confidence.Reason != "" {
		_, _ = fmt.Fprintf(b, " (%s)", f.Confidence.Reason)
	}
	b.WriteString("\n")
	_, _ = fmt.Fprintf(b, "    basis: %s\n", f.Impact.Basis)
	for _, ev := range f.Evidence {
		if ev.Value != "" {
			_, _ = fmt.Fprintf(b, "    - %s: %s\n", ev.Claim, ev.Value)
		} else {
			_, _ = fmt.Fprintf(b, "    - %s\n", ev.Claim)
		}
	}
}

// unitTotals says how per-unit charges add up across actionable findings,
// e.g. "DLC_UNUSED on 3 jobs: 200 credits per job run each". It never claims
// a saving over time: there is no run count.
func unitTotals(findings []Finding) []string {
	type key struct {
		verdict string
		unit    string
		credits float64
	}
	counts := map[key]int{}
	for _, f := range findings {
		if f.Disposition != finding.DispositionActionable.String() || f.Impact.UnitCredits == nil {
			continue
		}
		counts[key{f.Verdict, f.Impact.Unit, *f.Impact.UnitCredits}]++
	}
	keys := slices.SortedFunc(maps.Keys(counts), func(a, b key) int { return strings.Compare(a.verdict, b.verdict) })
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		n := counts[k]
		line := fmt.Sprintf("%s on %d %s: %s credits per %s each", k.verdict, n, plural(n, "job"), finding.FormatCredits(k.credits), k.unit)
		if n > 1 {
			total := finding.FormatCredits(k.credits * float64(n))
			line += fmt.Sprintf(", %s credits per pipeline run in which all of them run", total)
		}
		lines = append(lines, line)
	}
	return lines
}

// codeRanking counts findings by reason code, most frequent first, so the
// commonest blocker leads.
func codeRanking(findings []Finding) string {
	counts := map[string]int{}
	for _, f := range findings {
		for _, c := range f.ReasonCodes {
			counts[c]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	codes := slices.SortedFunc(maps.Keys(counts), func(a, b string) int {
		if counts[a] != counts[b] {
			return counts[b] - counts[a]
		}
		return strings.Compare(a, b)
	})
	parts := make([]string, 0, len(codes))
	for _, c := range codes {
		parts = append(parts, fmt.Sprintf("%s %d", c, counts[c]))
	}
	return "reason codes (findings): " + strings.Join(parts, ", ")
}

func describeParams(params map[string]any) string {
	if len(params) == 0 {
		return "none"
	}
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(params)) {
		parts = append(parts, fmt.Sprintf("%s=%v", k, params[k]))
	}
	return strings.Join(parts, ", ")
}
