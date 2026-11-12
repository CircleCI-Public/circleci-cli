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
	"slices"
	"strings"
	"unicode/utf8"
)

// redact returns what may be shown of a secret value: its first four
// characters and an ellipsis, or the ellipsis alone when the value is short
// enough that four characters would reveal much of it.
func redact(v string) string {
	if utf8.RuneCountInString(v) >= 12 {
		return string([]rune(v)[:4]) + "…"
	}
	return "…"
}

// redaction is a span of one source line to replace in an excerpt.
type redaction struct {
	line        int
	col, endCol int // 1-based, endCol exclusive, in characters
	with        string
}

// redactLine returns line with each redaction in rs applied. Redactions are
// non-overlapping spans of the same source line.
func redactLine(line string, rs []redaction) string {
	runes := []rune(line)
	sorted := slices.Clone(rs)
	slices.SortFunc(sorted, func(a, b redaction) int { return b.col - a.col })
	for _, r := range sorted {
		start := min(max(r.col-1, 0), len(runes))
		end := min(max(r.endCol-1, start), len(runes))
		runes = append(runes[:start:start], append([]rune(r.with), runes[end:]...)...)
	}
	return strings.TrimRight(string(runes), "\r")
}
