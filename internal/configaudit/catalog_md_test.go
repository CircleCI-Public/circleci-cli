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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestCatalogMarkdown(t *testing.T) {
	md := CatalogMarkdown()
	lines := strings.Split(md, "\n")

	t.Run("every rule has exactly one heading", func(t *testing.T) {
		for _, r := range Catalog() {
			n := 0
			for _, line := range lines {
				if line == "### "+r.ID {
					n++
				}
			}
			assert.Check(t, cmp.Equal(n, 1), "rule %s", r.ID)
		}
	})

	t.Run("every rule is in the table", func(t *testing.T) {
		for _, r := range Catalog() {
			assert.Check(t, cmp.Contains(md, "| `"+r.ID+"` | "+r.Severity.String()+" | "+r.Confidence.String()+" |"), "rule %s", r.ID)
		}
	})

	t.Run("the table says what each rule needs", func(t *testing.T) {
		for _, r := range Catalog() {
			needs := "| file |"
			if r.Online {
				needs = "| `--online` |"
			}
			row := ""
			for _, line := range lines {
				if strings.HasPrefix(line, "| `"+r.ID+"` |") {
					row = line
				}
			}
			assert.Check(t, strings.HasSuffix(row, needs), "rule %s: %s", r.ID, row)
		}
	})

	t.Run("the reference sections are present", func(t *testing.T) {
		for _, want := range []string{
			"## Rules", "## Suppressing a finding", "## Exit codes", "## Code scanning",
			"circleci-audit: ignore[", "--format sarif > audit.sarif || true", "not expanded",
		} {
			assert.Check(t, cmp.Contains(md, want))
		}
	})
}
