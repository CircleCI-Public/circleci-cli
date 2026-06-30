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
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestStepRows_UnfinishedStep verifies that a step with no stop time renders "~"
// in the duration column (rather than a blank gap), while a finished step shows
// its elapsed time, both right-padded to the same width.
func TestStepRows_UnfinishedStep(t *testing.T) {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute + 4*time.Second)

	rows := stepRows(apiclient.JobV3Execution{
		Index: 0,
		Steps: []apiclient.JobV3Step{
			{Name: "run tests", Num: 101, Phase: "ended", Outcome: "succeeded", StartedAt: start, StoppedAt: &end},
			{Name: "deploy", Num: 102, Phase: "running", StartedAt: start}, // no StoppedAt
		},
	})

	assert.Assert(t, is.Len(rows, 2))
	assert.Check(t, strings.Contains(rows[0].Label, "1m4s"), "finished step should show duration: %q", rows[0].Label)
	assert.Check(t, strings.Contains(rows[1].Label, "~"), "unfinished step should show ~: %q", rows[1].Label)
	// The "~" occupies the duration column, padded to the finished step's width.
	assert.Check(t, strings.Contains(rows[1].Label, "~    "), "~ should be padded to the duration column width: %q", rows[1].Label)
}
