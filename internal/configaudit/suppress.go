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
	"regexp"
	"strings"
)

var ignoreRE = regexp.MustCompile(`#\s*circleci-audit:\s*ignore\[([^\]]+)\]`)

// parseSuppressions scans the raw source lines for
// "# circleci-audit: ignore[a, b]" comments. It returns the rule ids to ignore
// by 1-based line number, and a warning for each id that names no rule.
//
// A comment alone on its line applies to the next line; a comment after other
// text applies to its own line. The scan is over raw lines, so a shell comment
// inside a block scalar counts too.
func parseSuppressions(lines []string) (map[int][]string, []string) {
	ignored := map[int][]string{}
	warnings := []string{}
	for i, line := range lines {
		m := ignoreRE.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		target := i + 1
		if strings.TrimSpace(line[:m[0]]) == "" {
			target++
		}
		for _, id := range strings.Split(line[m[2]:m[3]], ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := ruleByID(id); !ok {
				warnings = append(warnings, fmt.Sprintf("unknown rule %q in ignore comment on line %d", id, i+1))
				continue
			}
			ignored[target] = append(ignored[target], id)
		}
	}
	return ignored, warnings
}
