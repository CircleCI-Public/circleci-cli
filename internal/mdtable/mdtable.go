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

// Package mdtable builds GitHub-Flavored Markdown tables.
//
// Example:
//
//	t := mdtable.New("Name", "Value")
//	t.Row("FOO", "bar")
//	t.Row("BAZ", "qux")
//	fmt.Print(t.Render())
package mdtable

import (
	"fmt"
	"strings"
)

// Table accumulates headers and rows, then renders a GFM-aligned table.
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// New creates a Table with the given column headers.
func New(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{headers: headers, widths: widths}
}

// Row appends a data row. Values beyond the header count are ignored;
// missing values are treated as empty strings.
func (t *Table) Row(values ...string) {
	row := make([]string, len(t.headers))
	for i := range t.headers {
		if i < len(values) {
			row[i] = values[i]
		}
		if len(row[i]) > t.widths[i] {
			t.widths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

// Render returns the GFM markdown table string.
func (t *Table) Render() string {
	var sb strings.Builder

	// Header row
	for i, h := range t.headers {
		_, _ = fmt.Fprintf(&sb, "| %-*s ", t.widths[i], h)
	}
	sb.WriteString("|\n")

	// Separator row
	for _, w := range t.widths {
		_, _ = fmt.Fprintf(&sb, "| %s ", strings.Repeat("-", w))
	}
	sb.WriteString("|\n")

	// Data rows
	for _, row := range t.rows {
		for i, v := range row {
			_, _ = fmt.Fprintf(&sb, "| %-*s ", t.widths[i], v)
		}
		sb.WriteString("|\n")
	}

	return sb.String()
}
