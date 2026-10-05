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

package resourceclass

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

// rates is a two-class table: 2xlarge 80, xlarge 40, medium 10, small 5,
// and DLC 200 per job run.
type rates struct{ pricing.None }

func (rates) FlatCharge(kind string) (float64, bool) {
	c, ok := map[string]float64{"dlc": 200}[kind]
	return c, ok
}

func (rates) CreditsPerMinute(_, class, _ string) (float64, bool) {
	c, ok := map[string]float64{"2xlarge": 80, "xlarge": 40, "medium": 10, "small": 5}[class]
	return c, ok
}

func effective(t *testing.T, yaml string) *pipelineconfig.Effective {
	t.Helper()
	eff, err := pipelineconfig.NewEffective([]byte(yaml))
	assert.NilError(t, err)
	return eff
}

func job(t *testing.T, eff *pipelineconfig.Effective, name string) pipelineconfig.Job {
	t.Helper()
	i := slices.IndexFunc(eff.Jobs, func(j pipelineconfig.Job) bool { return j.Name == name })
	assert.Assert(t, i >= 0, "no job %s", name)
	return eff.Jobs[i]
}

// withinTolerance compares a computed ratio, whose last bits depend on the
// order of the arithmetic.
func withinTolerance(got, want, delta float64) cmp.Comparison {
	return func() cmp.Result {
		if math.Abs(got-want) <= delta {
			return cmp.ResultSuccess
		}
		return cmp.ResultFailure(fmt.Sprintf("%v not within %v of %v", got, delta, want))
	}
}

func samples(n int, pct float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = pct
	}
	return out
}

// Each container counts for its own share of the run.
func TestPerContainerPricing(t *testing.T) {
	eff := effective(t, `jobs:
  test: {docker: [{image: x}], resource_class: 2xlarge, parallelism: 2, steps: [checkout]}
`)
	run := usage.Run{DurationSeconds: 60, Executions: []usage.Execution{
		{Index: 0, CPUPct: samples(20, 100), MemoryPct: samples(20, 5)}, // busy: 16 cores on 8 → 2×
		{Index: 1, CPUPct: samples(10, 10), MemoryPct: samples(10, 5)},  // half as long, idle
	}}
	perContainer := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{"test": {ResourceClass: "2xlarge", Runs: []usage.Run{run}}}}
	busiest := &usage.Data{Jobs: map[string]usage.Job{"test": {ResourceClass: "2xlarge",
		Runs: []usage.Run{{DurationSeconds: 60, CPUPct: samples(20, 100), MemoryPct: samples(20, 5)}}}}}

	t.Run("credits: 1.5 container-minutes, not 2", func(t *testing.T) {
		c, ok := (&Module{pricing: rates{}, usage: perContainer}).jobCredits(job(t, eff, "test"))
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(c.measured, 120.0))
		_, ok = (&Module{pricing: rates{}, usage: busiest}).jobCredits(job(t, eff, "test"))
		assert.Check(t, !ok, "the first format cannot price a parallel job")
	})
	t.Run("credits follow container time, the wall clock the longest container", func(t *testing.T) {
		s := measure(perContainer.Jobs["test"], 16, 8, false)
		// (1 × 2 + 0.5 × 1) / 1.5 container-units; the longest is 1 × 2.
		assert.Check(t, withinTolerance(s.slowdown, 2.5/1.5, 1e-9))
		assert.Check(t, cmp.Equal(s.wall, 2.0))
		assert.Check(t, s.perContainer)
	})
	t.Run("a run missing a container is left out", func(t *testing.T) {
		partial := usage.Run{DurationSeconds: 600, Executions: run.Executions[:1]}
		d := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{"test": {ResourceClass: "2xlarge",
			Runs: []usage.Run{run, partial}}}}
		c, ok := (&Module{pricing: rates{}, usage: d}).jobCredits(job(t, eff, "test"))
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(c.measured, 120.0), "the partial run would have cost 800")
	})
}

// The credit slowdown is a ratio of sums over runs, not a mean of
// per-run ratios.
func TestRatioOfSums(t *testing.T) {
	two := usage.Run{DurationSeconds: 100, Executions: []usage.Execution{
		{Index: 0, CPUPct: samples(10, 100), MemoryPct: samples(10, 5)}, // 2× on 8 of 16 cores
		{Index: 1, CPUPct: samples(10, 100), MemoryPct: samples(10, 5)},
	}}
	one := usage.Run{DurationSeconds: 100, Executions: []usage.Execution{{Index: 0, CPUPct: samples(10, 10), MemoryPct: samples(10, 5)}}}
	s := measure(usage.Job{Runs: []usage.Run{two, one}}, 16, 8, false)
	// Σ duration × after / Σ duration × before = (100×4 + 100×1) / (100×2 + 100×1).
	assert.Check(t, withinTolerance(s.slowdown, 500.0/300.0, 1e-9))
}

// Only successful runs on the config's class are evidence for a prediction.
func TestEvidence(t *testing.T) {
	eff := effective(t, `jobs:
  j: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
`)
	ok := usage.Run{DurationSeconds: 60, Outcome: "succeeded", Executions: []usage.Execution{{Index: 0, CPUPct: samples(4, 10), MemoryPct: samples(4, 5)}}}
	failed := ok
	failed.Outcome = "failed"
	other := ok
	other.ResourceClass = "2xlarge"
	d := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{"j": {ResourceClass: "xlarge",
		Runs: []usage.Run{ok, failed, other}}}}
	ev := (&Module{pricing: rates{}, usage: d}).evidence(job(t, eff, "j"), d.Jobs["j"])
	assert.Check(t, cmp.Len(ev.Runs, 1))
}

// Duration-only runs price a job only when nothing about its containers or
// class is unknown.
func TestDurationOnly(t *testing.T) {
	eff := effective(t, `jobs:
  single: {docker: [{image: x}], resource_class: small, steps: [checkout]}
  parallel: {docker: [{image: x}], resource_class: small, parallelism: 2, steps: [checkout]}
  implicit: {docker: [{image: x}], steps: [checkout]}
  remote: {docker: [{image: x}], resource_class: small, steps: [checkout, setup_remote_docker]}
`)
	only := usage.Job{Runs: []usage.Run{{DurationSeconds: 60}}}
	d := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{
		"single": only, "parallel": only, "implicit": only, "remote": only}}
	m := &Module{pricing: rates{}, usage: d}
	tests := []struct {
		name string // the job
		want bool
	}{
		{name: "single", want: true},
		{name: "parallel", want: false},
		{name: "implicit", want: false},
		{name: "remote", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := m.jobCredits(job(t, eff, tc.name))
			assert.Check(t, cmp.Equal(ok, tc.want))
		})
	}
}

// An unexplained class mismatch is priced both ways; remote Docker's
// known upsize is priced as it ran, and any other remote-Docker mismatch is
// unexplained.
func TestMismatchPricing(t *testing.T) {
	eff := effective(t, `jobs:
  stale: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  image: {docker: [{image: x}], resource_class: small, steps: [checkout, setup_remote_docker]}
  odd: {docker: [{image: x}], resource_class: small, steps: [checkout, setup_remote_docker]}
`)
	run := usage.Run{DurationSeconds: 60, CPUPct: samples(4, 10), MemoryPct: samples(4, 5)}
	d := &usage.Data{Jobs: map[string]usage.Job{
		"stale": {ResourceClass: "2xlarge", Runs: []usage.Run{run}},
		"image": {ResourceClass: "medium", Runs: []usage.Run{run}},
		"odd":   {ResourceClass: "2xlarge", Runs: []usage.Run{run}},
	}}
	p := (&Module{pricing: rates{}, usage: d}).pipelineCredits(eff)
	slices.Sort(p.mismatched)
	assert.Check(t, cmp.DeepEqual(p.mismatched, []string{"odd", "stale"}))
	assert.Check(t, cmp.Equal(p.measured, 80.0+10.0+80.0))
	assert.Check(t, cmp.Equal(p.configured, 40.0+10.0+5.0))
}

// The usage API spells generations with a dash; configs with a dot.
func TestCanonicalClass(t *testing.T) {
	eff := effective(t, `jobs:
  j: {docker: [{image: x}], resource_class: xlarge.gen2, steps: [checkout]}
`)
	assert.Check(t, explained(job(t, eff, "j"), "xlarge-gen2"))
	assert.Check(t, !explained(job(t, eff, "j"), "xlarge"))
	assert.Check(t, cmp.Equal(canonical("medium-gen3"), "medium.gen3"))
}

// A flat charge per run (DLC) is part of the pipeline's credits, so the
// downsize's share of them is smaller.
func TestFlatCharges(t *testing.T) {
	eff := effective(t, `jobs:
  test: {docker: [{image: x}], resource_class: 2xlarge, steps: [checkout]}
  image:
    docker: [{image: x}]
    resource_class: small
    steps: [checkout, {setup_remote_docker: {docker_layer_caching: true}}]
`)
	run := func(sec float64) usage.Run {
		return usage.Run{DurationSeconds: sec, Executions: []usage.Execution{{Index: 0, CPUPct: samples(4, 10), MemoryPct: samples(4, 5)}}}
	}
	u := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{
		"test":  {ResourceClass: "2xlarge", Runs: []usage.Run{run(60)}},
		"image": {ResourceClass: "small", Runs: []usage.Run{run(60)}},
	}}
	assert.Check(t, job(t, eff, "image").DockerLayerCaching())
	assert.Check(t, !job(t, eff, "test").DockerLayerCaching())
	p := (&Module{pricing: rates{}, usage: u}).pipelineCredits(eff)
	assert.Check(t, cmp.Equal(p.measured, 85.0), "80 for test, 5 for image")
	assert.Check(t, cmp.Equal(p.flat, 200.0))
	assert.Check(t, cmp.Len(p.missing, 0))

	none := (&Module{pricing: ratesNoFlat{}, usage: u}).pipelineCredits(eff)
	assert.Check(t, cmp.Contains(none.missing, "image's DLC charge"), "an unknown charge leaves the pipeline unknown")
}

type ratesNoFlat struct{ rates }

func (ratesNoFlat) FlatCharge(string) (float64, bool) { return 0, false }
