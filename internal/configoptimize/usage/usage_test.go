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

package usage_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

// The per-container format, and the first one unchanged.
func TestParse(t *testing.T) {
	t.Run("per container", func(t *testing.T) {
		d, err := usage.Parse([]byte(`{"schema_version": 2, "source": "s", "jobs": {"test": {"resource_class": "xlarge", "runs": [
			{"duration_seconds": 300, "executions": [{"index": 0, "cpu_pct": [50, 60], "memory_pct": [10, 10]},
			                                         {"index": 1, "cpu_pct": [40], "memory_pct": [10]}]},
			{"duration_seconds": 290, "executions": []}]}}}`))
		assert.NilError(t, err)
		assert.Check(t, d.PerContainer())
		job, ok := d.Jobs["test"]
		assert.Assert(t, ok, "no job %s", "test")
		runs := job.Runs
		assert.Assert(t, cmp.Len(runs, 2))
		assert.Check(t, cmp.Len(runs[0].Containers(), 2))
		assert.Check(t, cmp.Len(runs[1].Containers(), 0), "a run without executions is duration only")
	})
	t.Run("the first format reads as one container", func(t *testing.T) {
		d, err := usage.Parse([]byte(`{"source": "s", "jobs": {"j": {"resource_class": "xlarge", "runs": [
			{"duration_seconds": 60, "cpu_pct": [5], "memory_pct": [5]}]}}}`))
		assert.NilError(t, err)
		assert.Check(t, !d.PerContainer())
		job, ok := d.Jobs["j"]
		assert.Assert(t, ok, "no job %s", "j")
		assert.Assert(t, cmp.Len(job.Runs, 1))
		assert.Check(t, cmp.Len(job.Runs[0].Containers(), 1))
	})
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "mixing both formats in one run",
			body:    `{"schema_version": 2, "source": "s", "jobs": {"j": {"runs": [{"duration_seconds": 1, "cpu_pct": [1], "executions": []}]}}}`,
			wantErr: "job j run 0 mixes cpu_pct with executions",
		},
		{
			name:    "executions without the version",
			body:    `{"source": "s", "jobs": {"j": {"runs": [{"duration_seconds": 1, "executions": []}]}}}`,
			wantErr: "job j run 0 has executions, which need schema_version 2",
		},
		{
			name:    "an unknown version",
			body:    `{"schema_version": 3, "source": "s", "jobs": {}}`,
			wantErr: "schema_version 3 is not supported",
		},
		{
			name:    "an execution sample outside 0 to 100",
			body:    `{"schema_version": 2, "source": "s", "jobs": {"j": {"runs": [{"duration_seconds": 1, "executions": [{"index": 0, "cpu_pct": [101], "memory_pct": [1]}]}]}}}`,
			wantErr: "job j run 0 has a sample outside 0-100",
		},
	}
	for _, tc := range tests {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			_, err := usage.Parse([]byte(tc.body))
			assert.Check(t, cmp.ErrorContains(err, tc.wantErr))
		})
	}
}
