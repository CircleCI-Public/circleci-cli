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

package configaudit

import (
	"fmt"
	"slices"
	"strings"
)

// Audit runs every rule over doc and returns the report. Findings are
// de-duplicated on rule id and position, split into reported and suppressed,
// filtered by opts.MinSeverity (a finding below it appears nowhere, in no
// count) and sorted.
func Audit(doc *Doc, opts Options) *Report {
	ignored, warnings := parseSuppressions(doc.lines)
	report := &Report{
		File:        doc.file,
		Findings:    []Finding{},
		Warnings:    warnings,
		MinSeverity: opts.MinSeverity,
	}

	// Gather every finding first: the online checks look at them (an unpinned
	// orb raises uncertified-orbs-allowed; a restricted context lowers
	// context-on-any-branch) before the minimum severity drops any.
	var all []Finding
	secrets := map[int][]redaction{}
	seenSecret := map[redaction]bool{}
	for _, rule := range catalog {
		check := checkers[rule.ID]
		if check == nil {
			continue
		}
		check(doc, func(f Finding) {
			f = complete(f, rule, doc)
			if r := f.secret; r != nil && !seenSecret[*r] {
				seenSecret[*r] = true
				secrets[r.line] = append(secrets[r.line], *r)
			}
			all = append(all, f)
		})
	}
	if s := opts.Online; s != nil {
		all = append(all, settingsFindings(s, all)...)
		for _, name := range doc.parameterisedContexts() {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("context %s is parameterised; its restrictions were not checked", name))
		}
		report.Online = &Online{Checked: s.checked(), Project: s.Project, Requests: append([]string{}, s.Requests...)}
	}

	seen := map[[3]any]bool{}
	for _, f := range all {
		if f.Severity < opts.MinSeverity {
			continue
		}
		if f.Position != nil {
			key := [3]any{f.RuleID, f.Line, f.Column}
			if seen[key] {
				continue
			}
			seen[key] = true
			if slices.Contains(ignored[f.Line], f.RuleID) {
				report.Suppressed = append(report.Suppressed, f)
				continue
			}
		}
		report.Findings = append(report.Findings, f)
	}

	// A secret is redacted in every excerpt of its line, whichever rule's
	// finding shows it.
	for _, list := range [][]Finding{report.Findings, report.Suppressed} {
		for i, f := range list {
			if f.Position != nil && len(secrets[f.Line]) > 0 {
				list[i].Snippet = redactLine(doc.snippet(f.Line), secrets[f.Line])
			}
		}
	}

	slices.SortStableFunc(report.Findings, compareFindings)
	slices.SortStableFunc(report.Suppressed, compareFindings)
	for _, f := range report.Findings {
		switch f.Severity {
		case SeverityHigh:
			report.Summary.High++
		case SeverityMedium:
			report.Summary.Medium++
		case SeverityLow:
			report.Summary.Low++
		}
	}
	report.Summary.Total = len(report.Findings)
	report.Summary.Suppressed = len(report.Suppressed)
	return report
}

// complete fills what a checker left out from the catalog and the source.
func complete(f Finding, rule Rule, doc *Doc) Finding {
	if f.Severity == 0 {
		f.Severity = rule.Severity
	}
	if f.Confidence == 0 {
		f.Confidence = rule.Confidence
	}
	if f.Fix == "" {
		f.Fix = rule.Fix
	}
	if f.Position != nil {
		f.File = doc.file
		f.Snippet = doc.snippet(f.Line)
	}
	return f
}

// snippet returns source line number line (1-based).
func (d *Doc) snippet(line int) string {
	if line < 1 || line > len(d.lines) {
		return ""
	}
	return strings.TrimRight(d.lines[line-1], "\r")
}

// compareFindings orders positioned findings by line, column and rule id, then
// unpositioned ones by severity (high first) and rule id.
func compareFindings(a, b Finding) int {
	switch {
	case a.Position != nil && b.Position == nil:
		return -1
	case a.Position == nil && b.Position != nil:
		return 1
	case a.Position == nil:
		if a.Severity != b.Severity {
			return int(b.Severity) - int(a.Severity)
		}
		return strings.Compare(a.RuleID, b.RuleID)
	}
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	if a.Column != b.Column {
		return a.Column - b.Column
	}
	return strings.Compare(a.RuleID, b.RuleID)
}
