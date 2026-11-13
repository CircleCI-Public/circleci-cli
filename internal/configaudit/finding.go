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

import "fmt"

// Severity ranks how much a finding matters. The zero value is invalid.
type Severity int

// The severities, lowest first so they compare with < and >.
const (
	SeverityLow Severity = iota + 1
	SeverityMedium
	SeverityHigh
)

// String returns the lowercase word for s.
func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	}
	return "unknown"
}

// MarshalText makes JSON show the lowercase word.
func (s Severity) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// ParseSeverity parses "low", "medium" or "high".
func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "low":
		return SeverityLow, nil
	case "medium":
		return SeverityMedium, nil
	case "high":
		return SeverityHigh, nil
	}
	return 0, fmt.Errorf("unknown severity %q: must be one of low, medium, high", s)
}

// Confidence says how likely a finding is to be a real problem.
type Confidence int

// The confidence levels, lowest first.
const (
	ConfidenceLow Confidence = iota + 1
	ConfidenceMedium
	ConfidenceHigh
)

// String returns the lowercase word for c.
func (c Confidence) String() string {
	switch c {
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	}
	return "unknown"
}

// MarshalText makes JSON show the lowercase word.
func (c Confidence) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// Position is a span in the audited file. Line and columns are 1-based;
// EndColumn is exclusive, the column just past the span.
type Position struct {
	Line      int `json:"line"`
	Column    int `json:"column"`
	EndLine   int `json:"end_line"`
	EndColumn int `json:"end_column"`
}

// Finding is one problem a rule found.
type Finding struct {
	RuleID     string     `json:"rule_id"`
	Severity   Severity   `json:"severity"`
	Confidence Confidence `json:"confidence"`
	Message    string     `json:"message"`
	Fix        string     `json:"fix"`
	File       string     `json:"file,omitempty"`
	*Position             // nil for settings findings
	Snippet    string     `json:"snippet,omitempty"`  // source line, secrets redacted
	Location   string     `json:"location,omitempty"` // settings findings only

	// secret is the span of the source line that holds a secret value; Audit
	// replaces it in every snippet of that line.
	secret *redaction

	// context names the context a context-on-any-branch finding is about, so
	// the online checks can tell whether it is restricted.
	context string
}

// Summary counts the findings in a Report.
type Summary struct {
	High       int `json:"high"`
	Medium     int `json:"medium"`
	Low        int `json:"low"`
	Total      int `json:"total"`
	Suppressed int `json:"suppressed"`
}

// Report is the result of auditing one config.
type Report struct {
	File string `json:"file"`
	// Findings are those at or above MinSeverity and not suppressed, sorted.
	Findings   []Finding `json:"findings"`
	Suppressed []Finding `json:"-"`
	Summary    Summary   `json:"summary"`
	Online     *Online   `json:"online,omitempty"`
	Warnings   []string  `json:"warnings"`
	// MinSeverity is the threshold the report was filtered with.
	MinSeverity Severity `json:"-"`
}

// Options configure Audit.
type Options struct {
	MinSeverity Severity
	// Online carries what --online read; nil without --online.
	Online *Settings
}
