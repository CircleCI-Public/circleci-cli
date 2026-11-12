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
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// locate maps a span of a scalar's text to a position in the source. off and
// length are a byte range of n.Value. Columns count characters, as yaml.v3's do.
//
// Plain and quoted single-line scalars map by arithmetic from the node's own
// column; literal blocks (|) by the block's indentation; folded blocks (>),
// multi-line flow scalars and escaped scalars by finding the text again in the
// source lines the node spans. When the text cannot be found the span starts
// at the node itself.
func locate(n *yaml.Node, lines []string, off, length int) Position {
	text := n.Value[off : off+length]
	if line, col, ok := locateDirect(n, lines, off, text); ok {
		return spanOf(n, line, col, text, lines)
	}
	if line, col, ok := locateBySearch(n, lines, off, text); ok {
		return spanOf(n, line, col, text, lines)
	}
	return Position{
		Line:      n.Line,
		Column:    n.Column,
		EndLine:   n.Line,
		EndColumn: n.Column + utf8.RuneCountInString(firstLine(text)),
	}
}

// spanOf builds the Position of text found at line:col.
func spanOf(n *yaml.Node, line, col int, text string, lines []string) Position {
	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		return Position{Line: line, Column: col, EndLine: line, EndColumn: col + utf8.RuneCountInString(text)}
	}
	endCol := utf8.RuneCountInString(parts[len(parts)-1]) + 1
	if n.Style&yaml.LiteralStyle != 0 {
		endCol += blockIndent(n, lines)
	}
	return Position{Line: line, Column: col, EndLine: line + len(parts) - 1, EndColumn: endCol}
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

// locateDirect computes the position without searching, and checks it against
// the source so an escape or an unusual block header cannot give a wrong answer.
func locateDirect(n *yaml.Node, lines []string, off int, text string) (line, col int, ok bool) {
	switch {
	case n.Style&yaml.FoldedStyle != 0:
		return 0, 0, false
	case n.Style&yaml.LiteralStyle != 0:
		before := n.Value[:off]
		idx := strings.Count(before, "\n")
		inLine := before[strings.LastIndex(before, "\n")+1:]
		line = n.Line + 1 + idx
		col = blockIndent(n, lines) + utf8.RuneCountInString(inLine) + 1
	default:
		if strings.Contains(n.Value, "\n") {
			return 0, 0, false
		}
		line = n.Line
		col = n.Column + utf8.RuneCountInString(n.Value[:off])
		if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
			col++
		}
	}
	return line, col, matchesAt(lines, line, col, firstLine(text))
}

// matchesAt reports whether the source has text at line:col.
func matchesAt(lines []string, line, col int, text string) bool {
	if line < 1 || line > len(lines) || col < 1 {
		return false
	}
	runes := []rune(lines[line-1])
	if col-1 > len(runes) {
		return false
	}
	return strings.HasPrefix(string(runes[col-1:]), text)
}

// locateBySearch finds the same occurrence of text in the source lines the node
// spans: the k-th, where k is how many times text occurs in n.Value before off.
func locateBySearch(n *yaml.Node, lines []string, off int, text string) (line, col int, ok bool) {
	needle := firstLine(text)
	if needle == "" {
		return 0, 0, false
	}
	skip := strings.Count(n.Value[:off], needle)
	first, last := spanLines(n, lines)
	for i := first; i < last; i++ {
		rest := lines[i]
		consumed := 0
		for {
			j := strings.Index(rest, needle)
			if j < 0 {
				break
			}
			if skip == 0 {
				return i + 1, utf8.RuneCountInString(lines[i][:consumed+j]) + 1, true
			}
			skip--
			rest = rest[j+len(needle):]
			consumed += j + len(needle)
		}
	}
	return 0, 0, false
}

// spanLines returns the 0-based half-open range of source lines n occupies.
func spanLines(n *yaml.Node, lines []string) (first, last int) {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		indent := blockIndent(n, lines)
		first, last = n.Line, n.Line
		for last < len(lines) {
			l := lines[last]
			if strings.TrimSpace(l) != "" && leadingSpaces(l) < indent {
				break
			}
			last++
		}
		return min(first, len(lines)), last
	}
	// Each source line of a flow scalar holds a word or is a blank line that
	// became a newline, so this bounds the lines it can occupy.
	count := len(strings.Fields(n.Value)) + strings.Count(n.Value, "\n")
	first = n.Line - 1
	return first, min(first+max(count, 1), len(lines))
}

// blockIndent is the indentation of a block scalar's content: the leading
// spaces of its first non-blank line.
func blockIndent(n *yaml.Node, lines []string) int {
	for i := n.Line; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return leadingSpaces(lines[i])
		}
	}
	return 0
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}
