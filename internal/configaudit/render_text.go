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
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Painter styles a severity label; the identity function gives plain text.
type Painter func(sev Severity, s string) string

// RenderText writes the report as one block per finding: a header, the source
// line with carets under the flagged span, and the fix; then a summary line.
// Excerpts print Finding.Snippet as given.
func RenderText(w io.Writer, r *Report, paint Painter) error {
	var b strings.Builder
	width := 1
	for _, f := range r.Findings {
		if f.Position != nil {
			width = max(width, len(strconv.Itoa(f.Line)))
		}
	}
	for _, f := range r.Findings {
		renderFinding(&b, f, width, paint)
		b.WriteString("\n")
	}
	switch {
	case len(r.Findings) > 0:
		fmt.Fprintf(&b, "%s (%d high, %d medium, %d low), %d suppressed\n",
			CountFindings(r.Summary.Total), r.Summary.High, r.Summary.Medium, r.Summary.Low, r.Summary.Suppressed)
	default:
		fmt.Fprintf(&b, "No findings at or above %s.\n", r.MinSeverity)
		if r.Summary.Suppressed > 0 {
			fmt.Fprintf(&b, "0 findings (0 high, 0 medium, 0 low), %d suppressed\n", r.Summary.Suppressed)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// CountFindings returns "1 finding" or "N findings".
func CountFindings(n int) string {
	if n == 1 {
		return "1 finding"
	}
	return fmt.Sprintf("%d findings", n)
}

func renderFinding(b *strings.Builder, f Finding, width int, paint Painter) {
	where := f.Location
	if f.Position != nil {
		where = fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)
	}
	fmt.Fprintf(b, "%s %s %s  %s\n", paint(f.Severity, f.Severity.String()), f.RuleID, where, f.Message)
	if f.Position != nil {
		pad := strings.Repeat(" ", width)
		fmt.Fprintf(b, "%s |\n", pad)
		fmt.Fprintf(b, "%*d | %s\n", width, f.Line, f.Snippet)
		fmt.Fprintf(b, "%s | %s\n", pad, caretLine(f))
	}
	fmt.Fprintf(b, "%s = fix: %s\n", strings.Repeat(" ", width), f.Fix)
}

// caretLine underlines [Column, EndColumn) of the snippet. A span that runs
// past the end of the line (or onto another line) stops at the line's end, and
// at least one caret is always drawn.
func caretLine(f Finding) string {
	lineLen := utf8.RuneCountInString(f.Snippet)
	start := max(f.Column, 1)
	end := f.EndColumn
	if f.EndLine != f.Line || end > lineLen+1 {
		end = lineLen + 1
	}
	return strings.Repeat(" ", start-1) + strings.Repeat("^", max(end-start, 1))
}
