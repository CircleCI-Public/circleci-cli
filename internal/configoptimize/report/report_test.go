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

package report_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// Every run ends with one result code, in the JSON and as the last line
// of the text report.
func TestResult(t *testing.T) {
	entry := func(d finding.Disposition) reconcile.Entry {
		return reconcile.Entry{Finding: finding.Finding{ID: "m:" + d.String(), Module: "m"}, Disposition: d}
	}
	usage := []string{"resourceclass: per-job usage (--usage)"}
	tests := []struct {
		name     string
		command  string
		entries  []reconcile.Entry
		missing  []string
		want     string
		lastLine string
	}{
		{"apply applied a change", report.CommandWrite,
			[]reconcile.Entry{entry(finding.DispositionApplied), entry(finding.DispositionRejected)}, usage,
			report.ResultCandidateProduced, "Result: candidate produced (1 change). Validate it with real runs before keeping it."},
		{"analyze found a change", report.CommandReport, []reconcile.Entry{entry(finding.DispositionActionable)}, nil,
			report.ResultCandidateProduced, "Result: a candidate can be produced (1 change); -o FILE writes it."},
		{"every change was rejected", report.CommandWrite, []reconcile.Entry{entry(finding.DispositionRejected)}, nil,
			report.ResultAllRejected, "Result: no candidate: the compile check rejected all 1 change."},
		{"a missing input outranks rejected changes", report.CommandWrite, []reconcile.Entry{entry(finding.DispositionRejected)}, usage,
			report.ResultMissingInput, "Result: no candidate: missing resourceclass: per-job usage (--usage)."},
		{"nothing applicable and an input missing", report.CommandReport, []reconcile.Entry{entry(finding.DispositionReportOnly)}, usage,
			report.ResultMissingInput, "Result: no candidate: missing resourceclass: per-job usage (--usage)."},
		{"nothing applicable", report.CommandReport, []reconcile.Entry{entry(finding.DispositionReportOnly)}, nil,
			report.ResultNoCandidate, "Result: no candidate: the enabled modules found no applicable change with the supplied inputs."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := report.Build(report.Meta{Command: tc.command, MissingInputs: tc.missing}, tc.entries)
			assert.Check(t, cmp.Equal(doc.Summary.Result, tc.want))
			var b strings.Builder
			assert.NilError(t, report.WriteText(&b, doc, report.TextOptions{}))
			lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
			assert.Check(t, cmp.Equal(lines[len(lines)-1], tc.lastLine))
		})
	}
}
