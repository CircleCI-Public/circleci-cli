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
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
)

// An inherited value becomes a job-level key only for a module that
// asks for it; any other finding on the same node stays report-only.
func TestFeasibilityOverride(t *testing.T) {
	authored, err := pipelineconfig.Parse([]byte(`
version: 2.1
executors:
  shared: {docker: [{image: cimg/base:current}], resource_class: xlarge}
jobs:
  a:
    executor: shared
    steps: [checkout]
  b: {executor: shared, steps: [checkout]}
workflows: {ci: {jobs: [a, b]}}
`))
	assert.NilError(t, err)
	eff, err := pipelineconfig.NewEffective([]byte("jobs:\n" +
		"  a: {docker: [{image: cimg/base:current}], resource_class: xlarge, steps: [checkout]}\n" +
		"  b: {docker: [{image: cimg/base:current}], resource_class: xlarge, steps: [checkout]}\n"))
	assert.NilError(t, err)
	mut := pipelineconfig.NewMutability(authored, eff)
	path := []string{"jobs", "a", "resource_class"}
	set := func(override bool) finding.Finding {
		return finding.Finding{
			ID: "rc:a", Module: "resourceclass", Target: finding.Target{Job: "a", Path: path},
			Suggestion: finding.Suggestion{Op: finding.OpSet, Value: "large", Override: override},
		}
	}

	t.Run("without the opt-in the finding stays report-only", func(t *testing.T) {
		entries := reconcile.Feasibility([]finding.Finding{set(false)}, eff, mut, reconcile.Policy{})
		assert.Assert(t, cmp.Len(entries, 1))
		e := entries[0]
		assert.Check(t, cmp.Equal(e.Disposition, finding.DispositionReportOnly))
		assert.Check(t, cmp.Contains(e.Reason, "shared-executor"))
	})
	t.Run("with the opt-in the job gets a key of its own", func(t *testing.T) {
		entries := reconcile.Feasibility([]finding.Finding{set(true)}, eff, mut, reconcile.Policy{})
		assert.Assert(t, cmp.Len(entries, 1))
		assert.Assert(t, cmp.Equal(entries[0].Disposition, finding.DispositionActionable), entries[0].Reason)
		groups := reconcile.Plan(entries).Groups
		assert.Assert(t, cmp.Len(groups, 1))
		g := groups[0]
		assert.Check(t, cmp.DeepEqual(g.Edits, []plan.Edit{{Op: plan.Insert, Path: path, Value: "large"}}))
		assert.Check(t, cmp.DeepEqual(g.Expected.Set, []plan.SetValue{{Path: path, Value: "large"}}))
	})
}

// Usage evidence is not held back by an unseen orb command, which the
// measured runs executed, but still is by a visible pipeline value.
func TestFeasibilityInputDependence(t *testing.T) {
	authored, err := pipelineconfig.Parse([]byte(`
version: 2.1
orbs:
  node: circleci/node@7
jobs:
  orb:
    docker: [{image: cimg/base:current}]
    resource_class: xlarge
    steps: [node/install-packages]
  piped:
    docker: [{image: cimg/base:current}]
    resource_class: xlarge
    steps: [{run: echo << pipeline.git.branch >>}]
workflows: {ci: {jobs: [orb, piped]}}
`))
	assert.NilError(t, err)
	eff, err := pipelineconfig.NewEffective([]byte("jobs:\n" +
		"  orb: {docker: [{image: cimg/base:current}], resource_class: xlarge, steps: [checkout]}\n" +
		"  piped: {docker: [{image: cimg/base:current}], resource_class: xlarge, steps: [checkout]}\n"))
	assert.NilError(t, err)
	mut := pipelineconfig.NewMutability(authored, eff)
	set := func(job string, fromUsage bool) finding.Finding {
		return finding.Finding{
			ID: "rc:" + job, Module: "resourceclass",
			Target:     finding.Target{Job: job, Path: []string{"jobs", job, "resource_class"}},
			Suggestion: finding.Suggestion{Op: finding.OpSet, Value: "large", FromUsage: fromUsage},
		}
	}
	tests := []struct {
		name      string
		f         finding.Finding
		want      finding.Disposition
		reasonHas string
	}{
		{"usage evidence past an opaque orb", set("orb", true), finding.DispositionActionable, ""},
		{"step evidence stays behind an opaque orb", set("orb", false), finding.DispositionReportOnly, "registry orb command"},
		{"usage evidence stays behind a visible pipeline value", set("piped", true), finding.DispositionReportOnly, "pipeline values"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries := reconcile.Feasibility([]finding.Finding{tc.f}, eff, mut, reconcile.Policy{})
			assert.Assert(t, cmp.Len(entries, 1))
			e := entries[0]
			assert.Check(t, cmp.Equal(e.Disposition, tc.want), e.Reason)
			assert.Check(t, cmp.Contains(e.Reason, tc.reasonHas))
		})
	}
}
