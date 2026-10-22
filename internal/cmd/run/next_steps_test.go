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
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRerunFromFailedCommand(t *testing.T) {
	t.Run("interpolates the workflow id", func(t *testing.T) {
		wfID := uuid.MustParse("b0000000-0000-4000-8000-0000000fa011")
		assert.Check(t, is.Equal(rerunFromFailedCommand(wfID),
			"circleci workflow rerun b0000000-0000-4000-8000-0000000fa011 --from-failed"))
	})

	t.Run("falls back to the placeholder for a nil id", func(t *testing.T) {
		assert.Check(t, is.Equal(rerunFromFailedCommand(uuid.Nil),
			"circleci workflow rerun <workflow-id> --from-failed"))
	})
}

func TestMoreFailedLine(t *testing.T) {
	runID := uuid.MustParse("f0000000-0000-4000-8000-00000000fa11")

	t.Run("singular for one", func(t *testing.T) {
		assert.Check(t, is.Equal(moreFailedLine(1, "failed job", "failed jobs", runID),
			"1 more failed job; see all with: circleci run get f0000000-0000-4000-8000-00000000fa11"))
	})

	t.Run("plural for more than one", func(t *testing.T) {
		assert.Check(t, is.Equal(moreFailedLine(2, "failed workflow", "failed workflows", runID),
			"2 more failed workflows; see all with: circleci run get f0000000-0000-4000-8000-00000000fa11"))
	})
}

func TestCappedLines(t *testing.T) {
	runID := uuid.MustParse("f0000000-0000-4000-8000-00000000fa11")
	line := func(s string) string { return "line " + s }

	t.Run("lists every item up to the cap", func(t *testing.T) {
		assert.Check(t, is.DeepEqual(cappedLines([]string{"a", "b", "c"}, "failed job", "failed jobs", runID, line), []string{
			"line a", "line b", "line c",
		}))
	})

	t.Run("lists the first items in order and counts the rest", func(t *testing.T) {
		assert.Check(t, is.DeepEqual(cappedLines([]string{"a", "b", "c", "d", "e"}, "failed job", "failed jobs", runID, line), []string{
			"line a", "line b", "line c",
			"2 more failed jobs; see all with: circleci run get f0000000-0000-4000-8000-00000000fa11",
		}))
	})

	t.Run("no lines for no items", func(t *testing.T) {
		assert.Check(t, is.Len(cappedLines(nil, "failed job", "failed jobs", runID, line), 0))
	})
}
