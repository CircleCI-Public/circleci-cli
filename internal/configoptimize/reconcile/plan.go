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
	"slices"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

// Plan builds the reconcile-to-apply contract from feasibility's entries:
// one group per actionable entry, in entry order (catalog, then job, then
// path).
func Plan(entries []Entry) plan.Plan {
	var p plan.Plan
	for _, e := range entries {
		if e.Disposition != finding.DispositionActionable {
			continue
		}
		f := e.Finding
		g := plan.Group{
			FindingID: f.ID,
			Fingerprint: plan.Fingerprint{
				Module:   f.Module,
				Job:      f.Target.Job,
				Workflow: f.Target.Workflow,
				Path:     slices.Clone(f.Target.Path),
			},
		}
		switch f.Suggestion.Op {
		case finding.OpRemove:
			for _, a := range e.Authored {
				g.Edits = append(g.Edits, plan.Edit{Op: plan.Delete, Path: slices.Clone(a.Path), Expect: a.Value})
			}
			for _, path := range f.ChangePaths() {
				g.Expected.Removed = append(g.Expected.Removed, slices.Clone(path))
			}
			g.Expected.FalseMeansAbsent = f.Suggestion.FalseMeansAbsent
		case finding.OpSet:
			if len(f.Suggestion.Authored) > 0 {
				for _, a := range f.Suggestion.Authored {
					g.Edits = append(g.Edits, plan.Edit{Op: plan.Replace, Path: slices.Clone(a.Path), Expect: a.Expect, Value: a.Value})
				}
			} else {
				for _, a := range e.Authored {
					edit := plan.Edit{Op: plan.Replace, Path: slices.Clone(a.Path), Expect: a.Value, Value: f.Suggestion.Value}
					if a.Insert {
						edit.Op = plan.Insert
					}
					g.Edits = append(g.Edits, edit)
				}
			}
			if len(f.Suggestion.Compiled) > 0 {
				for _, c := range f.Suggestion.Compiled {
					g.Expected.Set = append(g.Expected.Set, plan.SetValue{Path: slices.Clone(c.Path), Value: c.Value})
				}
			} else {
				for _, path := range f.ChangePaths() {
					g.Expected.Set = append(g.Expected.Set, plan.SetValue{Path: slices.Clone(path), Value: f.Suggestion.Value})
				}
			}
		case finding.OpAdd:
			for _, a := range e.Authored {
				g.Edits = append(g.Edits, plan.Edit{Op: plan.Insert, Path: slices.Clone(a.Path), Value: f.Suggestion.Value})
			}
		case finding.OpNone:
			continue
		}
		p.Groups = append(p.Groups, g)
	}
	return p
}
