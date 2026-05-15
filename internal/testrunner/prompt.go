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

package testrunner

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// BuildAgentPrompt returns the hardcoded prompt printed when local tests fail.
func BuildAgentPrompt(scan *reposcan.Result, lastN int, stdout, stderr string) string {
	test := testCommand(scan)
	if test == "" {
		test = "(no test command detected)"
	}

	var b strings.Builder
	b.WriteString("Agent-ready prompt:\n\n")
	b.WriteString("Please help me fix my repository so its existing test suite passes in CircleCI's generated test environment.\n\n")
	b.WriteString("Detected project:\n")
	fmt.Fprintf(&b, "- Stack: %s\n", valueOrFallback(scanValue(scan, "stack"), "unknown"))
	fmt.Fprintf(&b, "- Image: %s\n", valueOrFallback(scanImage(scan), "unknown"))
	fmt.Fprintf(&b, "- Test command: %s\n\n", test)
	b.WriteString("The test run failed. Use the command and output below to identify the likely code or test failure, then propose the smallest fix.\n\n")
	b.WriteString("Recent stdout:\n")
	b.WriteString(fenced(lastLines(stdout, lastN)))
	b.WriteString("\nRecent stderr:\n")
	b.WriteString(fenced(lastLines(stderr, lastN)))
	return b.String()
}

func scanValue(scan *reposcan.Result, field string) string {
	if scan == nil {
		return ""
	}
	if field == "stack" {
		return scan.Stack
	}
	return ""
}

func scanImage(scan *reposcan.Result) string {
	if scan == nil || scan.Image == "" {
		return ""
	}
	if scan.ImageVersion == "" {
		return scan.Image
	}
	return scan.Image + ":" + scan.ImageVersion
}

func valueOrFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func fenced(s string) string {
	if strings.TrimSpace(s) == "" {
		s = "(no output)"
	}
	return "```\n" + strings.TrimRight(s, "\n") + "\n```\n"
}

func lastLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
