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

package reconcile_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
)

func TestPlan(t *testing.T) {
	remove := finding.Finding{
		ID: "dlc:aaa", Module: "dlc",
		Target: finding.Target{Job: "build", Path: []string{"jobs", "build"}},
		Suggestion: finding.Suggestion{Op: finding.OpRemove, Paths: [][]string{
			{"jobs", "build", "machine", "docker_layer_caching"},
			{"jobs", "build", "steps", "0", "setup_remote_docker", "docker_layer_caching"},
		}},
	}
	entries := []reconcile.Entry{
		{Finding: finding.Finding{ID: "dlc:skip", Module: "dlc"}, Disposition: finding.DispositionReportOnly},
		{
			Finding: remove, Disposition: finding.DispositionActionable,
			Authored: []reconcile.AuthoredSite{
				{Path: []string{"executors", "vm", "machine", "docker_layer_caching"}, Value: true},
				{Path: []string{"jobs", "build", "steps", "0", "setup_remote_docker", "docker_layer_caching"}, Value: true},
			},
		},
	}

	got := reconcile.Plan(entries)

	assert.Assert(t, cmp.Len(got.Groups, 1), "only actionable entries become groups")
	g := got.Groups[0]
	t.Run("identity", func(t *testing.T) {
		assert.Check(t, cmp.Equal(g.FindingID, "dlc:aaa"))
		assert.Check(t, cmp.DeepEqual(g.Fingerprint, plan.Fingerprint{Module: "dlc", Job: "build", Path: []string{"jobs", "build"}}))
	})
	t.Run("one delete per authored path, expecting the analysed value", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(g.Edits, []plan.Edit{
			{Op: plan.Delete, Path: []string{"executors", "vm", "machine", "docker_layer_caching"}, Expect: true},
			{Op: plan.Delete, Path: []string{"jobs", "build", "steps", "0", "setup_remote_docker", "docker_layer_caching"}, Expect: true},
		}))
	})
	t.Run("the expected change is in effective paths", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(g.Expected.Removed, remove.Suggestion.Paths))
	})
}
