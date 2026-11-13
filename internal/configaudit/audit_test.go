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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func parseAudit(t *testing.T, src string, opts Options) *Report {
	t.Helper()
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	return Audit(doc, opts)
}

func TestAuditAnchorsAndMerges(t *testing.T) {
	t.Run("a finding in an anchor is reported once at the anchor", func(t *testing.T) {
		src := "x-docker: &d\n  - image: cimg/base:latest\n" +
			"jobs:\n  a:\n    docker: *d\n  b:\n    docker: *d\n"
		report := parseAudit(t, src, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].RuleID, "unpinned-image"))
		assert.Check(t, cmp.Equal(report.Findings[0].Line, 2))
		assert.Check(t, cmp.Equal(report.Findings[0].Column, 12))
	})

	t.Run("an executor merged from a base is audited", func(t *testing.T) {
		src := "x-base: &base\n  docker:\n    - image: cimg/base:latest\n" +
			"executors:\n  e:\n    <<: *base\n"
		report := parseAudit(t, src, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].Line, 3))
	})

	t.Run("an explicit key overrides a merged one", func(t *testing.T) {
		src := "x-base: &base\n  docker:\n    - image: cimg/base:latest\n" +
			"executors:\n  e:\n    <<: *base\n    docker:\n      - image: cimg/base@sha256:abc\n"
		report := parseAudit(t, src, Options{})
		assert.Check(t, cmp.Len(report.Findings, 0))
	})

	t.Run("a merge that refers to itself terminates", func(t *testing.T) {
		src := "executors:\n  e: &a\n    <<: *a\n    k: v\n"
		report := parseAudit(t, src, Options{})
		assert.Check(t, cmp.Len(report.Findings, 0))
	})
}

func TestAuditNotAPipeline(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "empty",
			yaml: "",
		},
		{
			name: "sequence",
			yaml: "- 1\n- 2\n",
		},
		{
			name: "scalar",
			yaml: "hello\n",
		},
		{
			name: "no sections",
			yaml: "version: 2.1\n",
		},
		{
			name: "sections of the wrong kind",
			yaml: "orbs: x\njobs: [a]\nexecutors: 1\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := parseAudit(t, tc.yaml, Options{})
			assert.Check(t, cmp.Len(report.Findings, 0))
			assert.Check(t, cmp.Len(report.Warnings, 0))
		})
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse([]byte("jobs: [\n"), "c.yml")
	assert.Check(t, cmp.ErrorContains(err, "parse c.yml: yaml: line"))
}

func TestAuditSortsFindings(t *testing.T) {
	src := "orbs:\n  b: x/b@1\n  a: x/a@1\n" +
		"jobs:\n  j:\n    docker:\n      - image: cimg/base:latest\n"
	report := parseAudit(t, src, Options{})
	got := make([]row, 0, len(report.Findings))
	for _, f := range report.Findings {
		got = append(got, row{ID: f.RuleID, Line: f.Line, Col: f.Column, Sev: f.Severity})
	}
	assert.Check(t, cmp.DeepEqual(got, []row{
		{ID: "unpinned-orb", Line: 2, Col: 6, Sev: SeverityMedium},
		{ID: "unpinned-orb", Line: 3, Col: 6, Sev: SeverityMedium},
		{ID: "unpinned-image", Line: 7, Col: 16, Sev: SeverityHigh},
	}))
}

func TestCompareFindingsUnpositioned(t *testing.T) {
	positioned := Finding{RuleID: "z", Severity: SeverityLow, Position: &Position{Line: 9, Column: 1}}
	tests := []struct {
		name string
		a, b Finding
		want int
	}{
		{
			name: "positioned before unpositioned",
			a:    positioned,
			b:    Finding{RuleID: "a", Severity: SeverityHigh},
			want: -1,
		},
		{
			name: "higher severity first",
			a:    Finding{RuleID: "z", Severity: SeverityHigh},
			b:    Finding{RuleID: "a", Severity: SeverityLow},
			want: -1,
		},
		{
			name: "same severity by rule id",
			a:    Finding{RuleID: "b", Severity: SeverityLow},
			b:    Finding{RuleID: "a", Severity: SeverityLow},
			want: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareFindings(tc.a, tc.b)
			assert.Check(t, cmp.Equal(got < 0, tc.want < 0))
			assert.Check(t, cmp.Equal(got > 0, tc.want > 0))
		})
	}
}

func TestAuditMinSeverity(t *testing.T) {
	src := "orbs:\n  a: x/a@1\njobs:\n  j:\n    docker:\n      - image: cimg/base:latest\n"
	tests := []struct {
		name       string
		min        Severity
		wantRules  []string
		wantHigh   int
		wantMedium int
		wantTotal  int
	}{
		{
			name:       "low reports both",
			min:        SeverityLow,
			wantRules:  []string{"unpinned-orb", "unpinned-image"},
			wantHigh:   1,
			wantMedium: 1,
			wantTotal:  2,
		},
		{
			name:       "medium reports both",
			min:        SeverityMedium,
			wantRules:  []string{"unpinned-orb", "unpinned-image"},
			wantHigh:   1,
			wantMedium: 1,
			wantTotal:  2,
		},
		{
			name:       "high keeps only the high one",
			min:        SeverityHigh,
			wantRules:  []string{"unpinned-image"},
			wantHigh:   1,
			wantMedium: 0,
			wantTotal:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := parseAudit(t, src, Options{MinSeverity: tc.min})
			got := make([]string, 0, len(report.Findings))
			for _, f := range report.Findings {
				got = append(got, f.RuleID)
			}
			assert.Check(t, cmp.DeepEqual(got, tc.wantRules))
			assert.Check(t, cmp.Equal(report.Summary.High, tc.wantHigh))
			assert.Check(t, cmp.Equal(report.Summary.Medium, tc.wantMedium))
			assert.Check(t, cmp.Equal(report.Summary.Total, tc.wantTotal))
		})
	}
}

func TestAuditSuppression(t *testing.T) {
	t.Run("a matching ignore moves the finding to suppressed", func(t *testing.T) {
		src := "orbs:\n  # circleci-audit: ignore[unpinned-orb]\n  a: x/a@1\n"
		report := parseAudit(t, src, Options{})
		assert.Check(t, cmp.Len(report.Findings, 0))
		assert.Check(t, cmp.Len(report.Suppressed, 1))
		assert.Check(t, cmp.Equal(report.Summary.Suppressed, 1))
		assert.Check(t, cmp.Equal(report.Summary.Total, 0))
	})

	t.Run("a known but non-matching id suppresses nothing", func(t *testing.T) {
		src := "orbs:\n  # circleci-audit: ignore[unpinned-image]\n  a: x/a@1\n"
		report := parseAudit(t, src, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].RuleID, "unpinned-orb"))
		assert.Check(t, cmp.Len(report.Suppressed, 0))
		assert.Check(t, cmp.Len(report.Warnings, 0))
	})

	t.Run("an unknown id is a warning", func(t *testing.T) {
		src := "orbs:\n  a: x/a@1 # circleci-audit: ignore[nope]\n"
		report := parseAudit(t, src, Options{})
		assert.Check(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.DeepEqual(report.Warnings, []string{
			`unknown rule "nope" in ignore comment on line 2`,
		}))
	})

	t.Run("a suppressed finding below the threshold is not counted", func(t *testing.T) {
		src := "orbs:\n  # circleci-audit: ignore[unpinned-orb]\n  a: x/a@1\n"
		report := parseAudit(t, src, Options{MinSeverity: SeverityHigh})
		assert.Check(t, cmp.Equal(report.Summary.Suppressed, 0))
		assert.Check(t, cmp.Len(report.Suppressed, 0))
	})
}

func TestAuditWarningsNeverNil(t *testing.T) {
	report := parseAudit(t, "jobs: {}\n", Options{})
	assert.Check(t, report.Warnings != nil)
	assert.Check(t, report.Findings != nil)
}

func TestAuditFillsFindingFromCatalogAndSource(t *testing.T) {
	report := parseAudit(t, "orbs:\n  a: x/a@1\n", Options{})
	assert.Assert(t, cmp.Len(report.Findings, 1))
	f := report.Findings[0]
	assert.Check(t, cmp.Equal(f.File, "c.yml"))
	assert.Check(t, cmp.Equal(f.Snippet, "  a: x/a@1"))
	assert.Check(t, cmp.Equal(f.Severity, SeverityMedium))
	assert.Check(t, cmp.Equal(f.Confidence, ConfidenceHigh))
	assert.Check(t, cmp.Equal(f.EndColumn, 11))
}

func TestParseSeverity(t *testing.T) {
	for _, want := range []Severity{SeverityLow, SeverityMedium, SeverityHigh} {
		t.Run(want.String(), func(t *testing.T) {
			got, err := ParseSeverity(want.String())
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(got, want))
		})
	}
	t.Run("invalid", func(t *testing.T) {
		_, err := ParseSeverity("extreme")
		assert.Check(t, cmp.ErrorContains(err, "low, medium, high"))
	})
}

func TestMarshalText(t *testing.T) {
	sev, err := SeverityMedium.MarshalText()
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(sev), "medium"))
	conf, err := ConfidenceLow.MarshalText()
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(conf), "low"))
}

func TestMapPairsMergePrecedence(t *testing.T) {
	src := "a: &a {k: from-a, only-a: 1}\nb: &b {k: from-b, only-b: 2}\nm:\n  <<: [*a, *b]\n  z: 3\n"
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	m := mapGet(doc.root, "m")
	got := map[string]string{}
	for _, kv := range mapPairs(m) {
		got[kv[0].Value] = kv[1].Value
	}
	assert.Check(t, cmp.DeepEqual(got, map[string]string{
		"k":      "from-a",
		"only-a": "1",
		"only-b": "2",
		"z":      "3",
	}))
}

func TestSteps(t *testing.T) {
	src := "steps:\n  - checkout\n  - run: echo hi\n  - save_cache:\n      key: k\n  - {a: 1, b: 2}\n"
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	all := steps(mapGet(doc.root, "steps"))
	got := make([]string, 0, len(all))
	for _, s := range all {
		got = append(got, s.name)
		assert.Check(t, s.nameNode != nil)
		assert.Check(t, cmp.Equal(s.body == nil, s.name == "checkout"), "step %q", s.name)
	}
	assert.Check(t, cmp.DeepEqual(got, []string{"checkout", "run", "save_cache"}))
}
