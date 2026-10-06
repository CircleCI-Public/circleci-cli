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

package reconcile

import (
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Entry is a finding with its disposition. Every input finding yields exactly
// one entry: nothing detected is dropped; a finding that cannot be applied
// becomes report-only.
type Entry struct {
	Finding     finding.Finding
	Disposition finding.Disposition
	// Mutability is the most restrictive label among the nodes the change
	// touches.
	Mutability pipelineconfig.Label
	// Authored are where the change would land: for actionable entries,
	// and for a report-only module's entries that would otherwise be
	// actionable, so the report can show the eligible change.
	Authored []AuthoredSite
	// Reason explains a report-only disposition. It is metadata, not a label.
	Reason string
}

// AuthoredSite is one authored node a change would edit.
type AuthoredSite struct {
	Path []string
	// Value is the authored value at Path.
	Value any
	// Insert is true when a job-level key is added rather than replaced,
	// leaving the shared source untouched.
	Insert bool
}

// ReasonNoChange marks an informational finding: its module proposes nothing.
const ReasonNoChange = "no change proposed"

// ReasonModuleReportOnly prefixes the reason for an eligible change that
// module policy keeps report-only.
const ReasonModuleReportOnly = "report-only module"

// ReasonInputDependent prefixes the reason for a change that is safe only
// under the pipeline inputs analysed.
const ReasonInputDependent = "input-dependent"

// Policy is module policy applied by feasibility. ReportOnly maps a module
// name to the reason its findings are never applied, even when they could be.
type Policy struct {
	ReportOnly map[string]string
}

// Feasibility splits findings into actionable and report-only using the
// mutability map. A change that touches several nodes is all or nothing:
// actionable only if every node is writable. A change is also
// report-only when the job's evidence could differ under other pipeline
// inputs, since the config is compiled for one set of inputs. Severity is not
// touched here.
func Feasibility(findings []finding.Finding, eff *pipelineconfig.Effective, mut *pipelineconfig.Mutability, policy Policy) []Entry {
	entries := make([]Entry, 0, len(findings))
	for _, f := range findings {
		if f.Suggestion.Op == finding.OpNone {
			// Informational: nothing would be edited, so no node is labelled.
			entries = append(entries, Entry{Finding: f, Disposition: finding.DispositionReportOnly, Reason: ReasonNoChange})
			continue
		}
		e := Entry{Finding: f, Mutability: pipelineconfig.LabelWritable}
		var blocked []string
		// A module that traced the authored values itself names them; the
		// patch still refuses anything it cannot edit narrowly, and the
		// compile check still verifies the result.
		for _, a := range f.Suggestion.Authored {
			e.Authored = append(e.Authored, AuthoredSite{Path: a.Path, Value: a.Expect})
		}
		paths := f.ChangePaths()
		if len(f.Suggestion.Authored) > 0 {
			paths = nil
		}
		for _, p := range paths {
			res := mut.Resolve(f.Target.Job, p, eff.Node(p))
			if res.Label != pipelineconfig.LabelWritable && f.Suggestion.Op == finding.OpSet && f.Suggestion.Override && res.Override != nil {
				// The job gets a value of its own; the shared source stays.
				e.Authored = append(e.Authored, AuthoredSite{Path: res.Override.Path, Value: res.Override.Value, Insert: res.Override.Insert})
				continue
			}
			if res.Label != pipelineconfig.LabelWritable {
				if e.Mutability == pipelineconfig.LabelWritable {
					e.Mutability = res.Label
				}
				blocked = append(blocked, string(res.Label)+": "+res.Reason)
				continue
			}
			e.Authored = append(e.Authored, AuthoredSite{Path: res.AuthoredPath, Value: res.Value})
		}
		switch {
		case len(blocked) > 0:
			e.Disposition = finding.DispositionReportOnly
			e.Reason = strings.Join(blocked, "; ")
			e.Authored = nil
		default:
			if dep, why := mut.InputDependence(f.Target.Job); blocksChange(dep, f) {
				e.Disposition = finding.DispositionReportOnly
				e.Reason = ReasonInputDependent + ": " + why
				e.Authored = nil
				break
			}
			e.Disposition = finding.DispositionActionable
			if why, reportOnly := policy.ReportOnly[f.Module]; reportOnly {
				// Keep the authored paths: the change is eligible, just not
				// applied by this module in this version.
				e.Disposition = finding.DispositionReportOnly
				e.Reason = ReasonModuleReportOnly + ": " + why + "; the change would otherwise be eligible"
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// blocksChange reports whether input dependence keeps a change report-only.
// Usage evidence already covers an opaque orb command, which the measured
// runs executed, but not a visible dependence on pipeline values.
func blocksChange(dep pipelineconfig.Dependence, f finding.Finding) bool {
	switch dep {
	case pipelineconfig.DependenceNone:
		return false
	case pipelineconfig.DependenceOpaqueOrb:
		return !f.Suggestion.FromUsage
	case pipelineconfig.DependenceKnown:
		return true
	}
	return true
}
