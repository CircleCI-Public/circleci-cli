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
)

const agentPromptTemplate = `My tests failed when running ` + "`circleci test run`" + `.

Project stack: %s
Image: %s
Command run: %s
Exit code: %d

Failing test output:
%s

Please diagnose the failure from the output above and propose a fix.
Once fixed, I will re-run ` + "`circleci test run`" + `.
`

// RenderPrompt returns the hardcoded POC prompt shown after local tests fail.
func RenderPrompt(stack, image, command string, exitCode int, outputTail string) string {
	if strings.TrimSpace(outputTail) == "" {
		outputTail = "(no output captured)"
	}
	return fmt.Sprintf(agentPromptTemplate, stack, image, command, exitCode, strings.TrimRight(outputTail, "\n"))
}
