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
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
)

func plainPainter(_ Severity, s string) string { return s }

func TestRenderText(t *testing.T) {
	report := &Report{
		File: "c.yml",
		Findings: []Finding{
			{
				RuleID:     "unpinned-orb",
				Severity:   SeverityMedium,
				Confidence: ConfidenceHigh,
				Message:    "orb circleci/node@5 is a major-version alias",
				Fix:        "pin the orb to an exact version (name@X.Y.Z)",
				File:       "c.yml",
				Position:   &Position{Line: 3, Column: 9, EndLine: 3, EndColumn: 24},
				Snippet:    "  node: circleci/node@5",
			},
			{
				RuleID:     "unpinned-image",
				Severity:   SeverityHigh,
				Confidence: ConfidenceHigh,
				Message:    "image cimg/base:latest uses the latest tag",
				Fix:        "pin the image to a digest (image:tag@sha256:<digest>)",
				File:       "c.yml",
				Position:   &Position{Line: 12, Column: 16, EndLine: 12, EndColumn: 32},
				Snippet:    "      - image: cimg/base:latest",
			},
			{
				RuleID:     "unpinned-image",
				Severity:   SeverityLow,
				Confidence: ConfidenceLow,
				Message:    "span runs past the end of its line",
				Fix:        "shorten the span",
				File:       "c.yml",
				Position:   &Position{Line: 20, Column: 8, EndLine: 20, EndColumn: 99},
				Snippet:    "  key: value",
			},
			{
				RuleID:     "unpinned-image",
				Severity:   SeverityLow,
				Confidence: ConfidenceLow,
				Message:    "span runs onto the next line",
				Fix:        "shorten the span",
				File:       "c.yml",
				Position:   &Position{Line: 21, Column: 3, EndLine: 22, EndColumn: 5},
				Snippet:    "  abc",
			},
			{
				RuleID:     "fork-pr-secrets",
				Severity:   SeverityHigh,
				Confidence: ConfidenceHigh,
				Message:    "a finding with no position",
				Fix:        "turn it off",
				Location:   "project settings",
			},
		},
		Summary:     Summary{High: 2, Medium: 1, Low: 2, Total: 5, Suppressed: 1},
		Warnings:    []string{},
		MinSeverity: SeverityLow,
	}

	t.Run("findings", func(t *testing.T) {
		var buf bytes.Buffer
		err := RenderText(&buf, report, plainPainter)
		assert.NilError(t, err)
		assert.Check(t, golden.String(buf.String(), "render_text.txt"))
	})

	t.Run("painter styles only the severity word", func(t *testing.T) {
		var buf bytes.Buffer
		paint := func(sev Severity, s string) string { return "[" + sev.String() + ":" + s + "]" }
		err := RenderText(&buf, report, paint)
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(buf.String(), "[medium:medium] unpinned-orb c.yml:3:9  "))
	})

	t.Run("one finding is singular", func(t *testing.T) {
		var buf bytes.Buffer
		one := &Report{
			File:        "c.yml",
			Findings:    report.Findings[:1],
			Summary:     Summary{Medium: 1, Total: 1},
			MinSeverity: SeverityLow,
		}
		err := RenderText(&buf, one, plainPainter)
		assert.NilError(t, err)
		assert.Check(t, golden.String(buf.String(), "render_text_single.txt"))
	})

	t.Run("no findings", func(t *testing.T) {
		var buf bytes.Buffer
		empty := &Report{File: "c.yml", Findings: []Finding{}, MinSeverity: SeverityHigh}
		err := RenderText(&buf, empty, plainPainter)
		assert.NilError(t, err)
		assert.Check(t, golden.String(buf.String(), "render_text_empty.txt"))
	})

	t.Run("no findings but some suppressed", func(t *testing.T) {
		var buf bytes.Buffer
		empty := &Report{
			File:        "c.yml",
			Findings:    []Finding{},
			Summary:     Summary{Suppressed: 2},
			MinSeverity: SeverityLow,
		}
		err := RenderText(&buf, empty, plainPainter)
		assert.NilError(t, err)
		assert.Check(t, golden.String(buf.String(), "render_text_suppressed.txt"))
	})
}

func TestCountFindings(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{
			n:    0,
			want: "0 findings",
		},
		{
			n:    1,
			want: "1 finding",
		},
		{
			n:    2,
			want: "2 findings",
		},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Check(t, cmp.Equal(CountFindings(tc.n), tc.want))
		})
	}
}
