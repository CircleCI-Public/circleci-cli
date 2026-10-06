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

// The exported helpers below are shared with the external tests in
// resourceclass_test.go.

// Ladder is an in-test pricing.Provider for Docker gen1: the classes it
// knows, smallest first, and a DLC charge of 200 per job run. Modules and
// their tests may not import the pricing table itself, so every rate a test
// relies on is stated here.
type Ladder []Class

// Class is one rung of a Ladder.
type Class struct {
	Name                  string
	Credits, VCPUs, RAMGB float64
}

var _ pricing.Provider = Ladder{}

// PublicLadder has the public Docker gen1 rates and sizes.
var PublicLadder = Ladder{
	{Name: "small", Credits: 5, VCPUs: 1, RAMGB: 2}, {Name: "medium", Credits: 10, VCPUs: 2, RAMGB: 4},
	{Name: "medium+", Credits: 15, VCPUs: 3, RAMGB: 6}, {Name: "large", Credits: 20, VCPUs: 4, RAMGB: 8},
	{Name: "xlarge", Credits: 40, VCPUs: 8, RAMGB: 16}, {Name: "2xlarge", Credits: 80, VCPUs: 16, RAMGB: 32},
	{Name: "2xlarge+", Credits: 100, VCPUs: 20, RAMGB: 40},
}

func (l Ladder) find(platform, class, generation string) int {
	if platform != "docker" || generation != "gen1" {
		return -1
	}
	return slices.IndexFunc(l, func(c Class) bool { return c.Name == class })
}

func (l Ladder) CreditsPerMinute(platform, class, generation string) (float64, bool) {
	if i := l.find(platform, class, generation); i >= 0 {
		return l[i].Credits, true
	}
	return 0, false
}

func (l Ladder) LadderBelow(platform, class, generation string) (string, bool) {
	if i := l.find(platform, class, generation); i > 0 {
		return l[i-1].Name, true
	}
	return "", false
}

func (l Ladder) Size(platform, class, generation string) (float64, float64, bool) {
	if i := l.find(platform, class, generation); i >= 0 {
		return l[i].VCPUs, l[i].RAMGB, true
	}
	return 0, 0, false
}

func (Ladder) FlatCharge(kind string) (float64, bool) { return 200, kind == "dlc" }

// Samples is n 15-second samples, each at pct.
func Samples(n int, pct float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = pct
	}
	return out
}

// Runs is n first-format runs of the given duration, each with the same CPU
// and memory samples (nil memory for none).
func Runs(n int, seconds float64, cpu, mem []float64) []usage.Run {
	out := make([]usage.Run, n)
	for i := range out {
		out[i] = usage.Run{DurationSeconds: seconds, CPUPct: cpu, MemoryPct: mem}
	}
	return out
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

// perContainer is per-container usage for one job.
func perContainer(name, class string, runs ...usage.Run) *usage.Data {
	return &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{name: {ResourceClass: class, Runs: runs}}}
}

// container is one execution of n samples at cpu%.
func container(index, n int, cpu float64) usage.Execution {
	return usage.Execution{Index: index, CPUPct: Samples(n, cpu), MemoryPct: Samples(n, 5)}
}

// Each container counts for its own share of the run.
func TestPerContainerPricing(t *testing.T) {
	eff := effective(t, `jobs:
  test: {docker: [{image: x}], resource_class: 2xlarge, parallelism: 2, steps: [checkout]}
`)
	// On 2xlarge (80/min): a busy container (16 cores, 2× on 8) and one half
	// as long and idle.
	run := usage.Run{DurationSeconds: 60, Executions: []usage.Execution{container(0, 20, 100), container(1, 10, 10)}}
	credits := func(u *usage.Data) (credits, bool) {
		return (&Module{pricing: PublicLadder, usage: u}).jobCredits(job(t, eff, "test"))
	}

	t.Run("credits: 1.5 container-minutes, not 2", func(t *testing.T) {
		c, ok := credits(perContainer("test", "2xlarge", run))
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(c.measured, 120.0))
	})
	t.Run("the first format cannot price a parallel job", func(t *testing.T) {
		_, ok := credits(&usage.Data{Jobs: map[string]usage.Job{"test": {ResourceClass: "2xlarge",
			Runs: Runs(1, 60, Samples(20, 100), Samples(20, 5))}}})
		assert.Check(t, !ok)
	})
	t.Run("a run missing a container is left out", func(t *testing.T) {
		partial := usage.Run{DurationSeconds: 600, Executions: run.Executions[:1]}
		c, ok := credits(perContainer("test", "2xlarge", run, partial))
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(c.measured, 120.0), "the partial run would have cost 800")
	})
	t.Run("credits follow container time, the wall clock the longest container", func(t *testing.T) {
		s := measure(usage.Job{Runs: []usage.Run{run}}, 16, 8, false)
		// (1 × 2 + 0.5 × 1) / 1.5 container-units; the longest is 1 × 2.
		assert.Check(t, withinTolerance(s.slowdown, 2.5/1.5, 1e-9))
		assert.Check(t, cmp.Equal(s.wall, 2.0))
		assert.Check(t, s.perContainer)
	})
	t.Run("the slowdown is a ratio of sums over runs, not a mean of ratios", func(t *testing.T) {
		two := usage.Run{DurationSeconds: 100, Executions: []usage.Execution{container(0, 10, 100), container(1, 10, 100)}}
		one := usage.Run{DurationSeconds: 100, Executions: []usage.Execution{container(0, 10, 10)}}
		s := measure(usage.Job{Runs: []usage.Run{two, one}}, 16, 8, false)
		// (100×4 + 100×1) / (100×2 + 100×1).
		assert.Check(t, withinTolerance(s.slowdown, 500.0/300.0, 1e-9))
	})
}

// Only successful runs on the config's class are evidence for a prediction.
func TestEvidence(t *testing.T) {
	eff := effective(t, `jobs:
  j: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
`)
	ok := usage.Run{DurationSeconds: 60, Outcome: "succeeded", Executions: []usage.Execution{container(0, 4, 10)}}
	failed, other := ok, ok
	failed.Outcome = "failed"
	other.ResourceClass = "2xlarge"
	d := perContainer("j", "xlarge", ok, failed, other)
	ev := (&Module{pricing: PublicLadder, usage: d}).evidence(job(t, eff, "j"), d.Jobs["j"])
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
	m := &Module{pricing: PublicLadder, usage: &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{
		"single": only, "parallel": only, "implicit": only, "remote": only}}}
	for name, want := range map[string]bool{"single": true, "parallel": false, "implicit": false, "remote": false} {
		t.Run(name, func(t *testing.T) {
			_, ok := m.jobCredits(job(t, eff, name))
			assert.Check(t, cmp.Equal(ok, want))
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
	runs := Runs(1, 60, Samples(4, 10), Samples(4, 5))
	d := &usage.Data{Jobs: map[string]usage.Job{
		"stale": {ResourceClass: "2xlarge", Runs: runs},
		"image": {ResourceClass: "medium", Runs: runs},
		"odd":   {ResourceClass: "2xlarge", Runs: runs},
	}}
	p := (&Module{pricing: PublicLadder, usage: d}).pipelineCredits(eff)
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
	run := usage.Run{DurationSeconds: 60, Executions: []usage.Execution{container(0, 4, 10)}}
	u := &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: map[string]usage.Job{
		"test":  {ResourceClass: "2xlarge", Runs: []usage.Run{run}},
		"image": {ResourceClass: "small", Runs: []usage.Run{run}},
	}}
	p := (&Module{pricing: PublicLadder, usage: u}).pipelineCredits(eff)
	assert.Check(t, cmp.Equal(p.measured, 85.0), "80 for test, 5 for image")
	assert.Check(t, cmp.Equal(p.flat, 200.0))
	assert.Check(t, cmp.Len(p.missing, 0))

	none := (&Module{pricing: noDLC{PublicLadder}, usage: u}).pipelineCredits(eff)
	assert.Check(t, cmp.Contains(none.missing, "image's DLC charge"), "an unknown charge leaves the pipeline unknown")
}

type noDLC struct{ Ladder }

func (noDLC) FlatCharge(string) (float64, bool) { return 0, false }
