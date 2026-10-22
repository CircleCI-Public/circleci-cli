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

package run

import (
	"fmt"

	"github.com/google/uuid"
)

// Next steps for a failed run.
const (
	// maxListedFailures caps the per-job and per-workflow lines in the next
	// steps. Anything over it is counted by moreFailedLine rather than listed.
	maxListedFailures = 3

	pushAndWatchCommand = `git push && circleci run watch --sha "$(git rev-parse HEAD)"`
)

// cappedLines renders one line per item, up to maxListedFailures in the order
// given, and counts the rest with moreFailedLine.
func cappedLines[T any](items []T, one, many string, runID uuid.UUID, line func(T) string) []string {
	listed := items[:min(len(items), maxListedFailures)]
	lines := make([]string, 0, len(listed)+1)
	for _, item := range listed {
		lines = append(lines, line(item))
	}
	if n := len(items) - len(listed); n > 0 {
		lines = append(lines, moreFailedLine(n, one, many, runID))
	}
	return lines
}

// rerunFromFailedCommand reruns only the failed jobs of a workflow. A workflow
// that arrived without an ID falls back to the placeholder: a nil UUID would
// produce a command that looks copy-pasteable but cannot work.
func rerunFromFailedCommand(workflowID uuid.UUID) string {
	id := "<workflow-id>"
	if workflowID != uuid.Nil {
		id = workflowID.String()
	}
	return fmt.Sprintf("circleci workflow rerun %s --from-failed", id)
}

// moreFailedLine counts the items left out by maxListedFailures and points to
// the command that lists them all. one and many name what is counted, e.g.
// "failed job" and "failed jobs", so the count says exactly what was left out.
func moreFailedLine(n int, one, many string, runID uuid.UUID) string {
	noun := many
	if n == 1 {
		noun = one
	}
	return fmt.Sprintf("%d more %s; see all with: circleci run get %s", n, noun, runID)
}
