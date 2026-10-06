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

// Package resourceclass downsizes Docker jobs one class when the saving is
// worth it, using per-job CPU and memory from an exported usage
// file. It predicts the slowdown on the smaller class from each
// sample's CPU demand, take a model-error margin off the predicted saving,
// weigh it by the job's share of the pipeline's credits, and propose the
// downsize only when that is material. Real runs then decide.
package resourceclass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

// Name is the module identifier.
const Name = "resourceclass"

// Verdicts.
const (
	// VerdictDownsize: step down one class.
	VerdictDownsize finding.Verdict = "RC_DOWNSIZE"
	// VerdictNoChange: the job has usage data, but a condition of the rule
	// fails; the reason code says which.
	VerdictNoChange finding.Verdict = "RC_NO_CHANGE"
)

// Rule identifies the version of this rule in a finding's evidence.
const Rule = "RC-R1-v2"

// Thresholds. None is calibrated against customer data yet.
var (
	// MinRuns is the fewest runs with samples the rule accepts; fewer are
	// noise. Not yet calibrated.
	MinRuns = 10
	// MinMedianSeconds: shorter jobs have too few 15-second samples.
	// Not yet calibrated.
	MinMedianSeconds = 15.0
	// MinVCPUs: below 3 cores, duration is sensitive to core count, so the
	// job is left alone. Not yet calibrated.
	MinVCPUs = 3.0
	// SaturatedCPUPct: a sample above this share of the CPU ceiling is
	// saturated. Saturation is evidence, not a gate: demand at the ceiling
	// is censored, so the prediction is optimistic. Not yet calibrated.
	SaturatedCPUPct = 85.0
	// MemoryHeadroom: the peak memory must fit within this share of the
	// smaller class's RAM. Not yet calibrated.
	MemoryHeadroom = 0.8
	// ModelErrorMargin is taken off the predicted job saving: the largest
	// error seen in validation (−39% predicted, −26% measured).
	// Not yet calibrated.
	ModelErrorMargin = 0.15
	// MinPipelineSaving is the smallest conservative saving, as a share of
	// the pipeline's estimated credits, worth proposing and validating.
	// Not yet calibrated.
	MinPipelineSaving = 0.05
)

// Reason codes for RC_NO_CHANGE.
const (
	CodeNotDocker     = "NOT_DOCKER"
	CodeClassMismatch = "CLASS_MISMATCH"
	CodeUnsized       = "UNSIZED_CLASS"
	CodeSmallClass    = "UNDER_3_VCPUS"
	CodeNoSmaller     = "NO_SMALLER_CLASS"
	CodeFewRuns       = "FEW_RUNS"
	CodeShortJob      = "SHORT_JOB"
	CodeMemory        = "MEMORY_BOUND"
	// CodeRightSized: no saving is left once the model-error margin is
	// taken off. Not a claim that the class is optimal.
	CodeRightSized = "RIGHT_SIZED_FOR_CREDITS"
	// CodeImmaterial: a saving remains, but too small a share of the
	// pipeline's credits to be worth validating.
	CodeImmaterial = "BELOW_MATERIALITY_THRESHOLD"
	// CodeShareUnknown: the pipeline's credits cannot be estimated, because
	// a job has no usage, no rate or an unknown parallelism, so materiality
	// cannot be judged.
	CodeShareUnknown = "PIPELINE_SHARE_UNKNOWN"
	// CodeNoUsage: the usage file has no runs for the job. It is reported,
	// not skipped.
	CodeNoUsage = "NO_USAGE"
	// CodeNoMemory: the runs have no memory samples, so the memory
	// condition cannot be checked.
	CodeNoMemory = "NO_MEMORY_SAMPLES"
)

// percent turns a percentage into a share.
const percent = 100.0

// platform and generation of the classes this module sizes.
const (
	platform   = "docker"
	generation = "gen1"
)

// Module is the resourceclass analyzer.
type Module struct {
	pricing pricing.Provider
	usage   *usage.Data
}

// New returns the module. Without usage data it reports nothing: missing
// data means no change.
func New(p pricing.Provider, u *usage.Data) *Module { return &Module{pricing: p, usage: u} }

// Name implements module.Analyzer.
func (*Module) Name() string { return Name }

// MissingInputs implements module.InputReporter: without a usage file the
// module reports nothing.
func (m *Module) MissingInputs() []string {
	if m.usage == nil {
		return []string{"per-job usage (--usage)"}
	}
	return nil
}

// Evaluate implements module.Analyzer. Every job gets a finding: RC_DOWNSIZE,
// or RC_NO_CHANGE with the reason, NO_USAGE when the usage file has no runs
// for it.
func (m *Module) Evaluate(_ context.Context, cfg *pipelineconfig.Effective) ([]finding.Finding, error) {
	if m.usage == nil {
		return nil, nil
	}
	pipe := m.pipelineCredits(cfg)
	var out []finding.Finding
	for _, job := range cfg.Jobs {
		u, ok := m.usage.Job(job.Name)
		if !ok {
			out = append(out, m.finding(job, []string{"jobs", job.Name, "resource_class"}, VerdictNoChange,
				"", "", stats{}, nil, CodeNoUsage, "the usage file has no runs for this job"))
			continue
		}
		out = append(out, m.judge(job, u, pipe))
	}
	return out, nil
}

// fingerprint identifies a compiled job apart from its resource class, so a
// finding can be matched across runs until anything else about the job changes.
// It hashes a canonical JSON encoding (mapping keys sorted, sequences in
// order), so it does not depend on how the compiler orders keys.
func fingerprint(job pipelineconfig.Job) string {
	var v map[string]any
	if job.Node == nil || job.Node.Decode(&v) != nil {
		return ""
	}
	delete(v, "resource_class")
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// pipeline is the estimated credits of one pipeline over every job in the
// config, priced two ways where a job's usage was measured on another class
// than the config names, and the jobs it could not price.
type pipeline struct {
	measured   float64  // each job at the class its runs used
	configured float64  // unexplained mismatches at the config's class instead
	flat       float64  // flat charges per run (DLC), the same whatever the classes
	mismatched []string // jobs priced differently in the two totals
	missing    []string // jobs without priceable runs, rate or a known parallelism
}

// pipelineCredits prices every job in the config. A job that cannot be
// priced makes the total unknown.
func (m *Module) pipelineCredits(cfg *pipelineconfig.Effective) pipeline {
	var p pipeline
	for _, job := range cfg.Jobs {
		if job.DockerLayerCaching() {
			// TODO: DLC may be charged per parallel run; this charges it once.
			if charge, ok := m.pricing.FlatCharge("dlc"); ok {
				p.flat += charge
			} else {
				p.missing = append(p.missing, job.Name+"'s DLC charge")
			}
		}
		c, ok := m.jobCredits(job)
		if !ok {
			p.missing = append(p.missing, job.Name)
			continue
		}
		p.measured += c.measured
		p.configured += c.configured
		if c.mismatched {
			p.mismatched = append(p.mismatched, job.Name)
		}
	}
	return p
}

// credits is one job's mean credits per pipeline, priced two ways.
type credits struct {
	measured, configured float64
	mismatched           bool // runs on another class than the config's, unexplained
}

// jobCredits prices a job's complete runs as container-seconds × rate: in the
// per-container format each container counts for its share of the run (its
// samples over the longest one's); in the first format the one container
// stands for the run, which is complete only for a single-container job
// Runs on the config's class, or on the class remote Docker is known
// to run it on, are what the pipeline pays. Without any, runs on another
// class may be stale, so they are priced at their class and at the config's.
// A job with only duration-only runs is priced at the config's class when it
// has one container, a class set in the config, and no remote Docker; any
// other job without complete runs is unpriced.
func (m *Module) jobCredits(job pipelineconfig.Job) (credits, bool) {
	u, ok := m.usage.Job(job.Name)
	n, known := parallelism(job)
	if !ok || !known || (!m.usage.PerContainer() && n > 1) {
		return credits{}, false
	}
	cfgClass := configClass(job)
	cfgRate, cfgOK := m.pricing.CreditsPerMinute(platform, cfgClass, generation)
	var current, other, durationOnly []usage.Run
	for _, r := range u.Runs {
		switch {
		case len(r.Containers()) == 0:
			durationOnly = append(durationOnly, r)
		case !m.complete(r, n):
		case explained(job, r.Class(u)):
			current = append(current, r)
		default:
			other = append(other, r)
		}
	}
	price := func(runs []usage.Run, atConfig bool) (float64, bool) {
		total := 0.0
		for _, r := range runs {
			rate, ok := cfgRate, cfgOK
			if !atConfig {
				rate, ok = m.pricing.CreditsPerMinute(platform, canonical(r.Class(u)), generation)
			}
			if !ok {
				return 0, false
			}
			total += containerSeconds(r) / secondsPerMinute * rate
		}
		return total / float64(len(runs)), true
	}
	switch {
	case len(current) > 0:
		c, ok := price(current, false)
		return credits{measured: c, configured: c}, ok
	case len(other) > 0:
		measured, ok1 := price(other, false)
		configured, ok2 := price(other, true)
		return credits{measured: measured, configured: configured, mismatched: true}, ok1 && ok2
	case len(durationOnly) > 0 && n == 1 && job.Get("resource_class") != nil && !remoteDocker(job) && cfgOK:
		total := 0.0
		for _, r := range durationOnly {
			total += r.DurationSeconds
		}
		c := total / float64(len(durationOnly)) / secondsPerMinute * cfgRate
		return credits{measured: c, configured: c}, true
	}
	return credits{}, false
}

// complete says whether every container of the run was measured.
func (m *Module) complete(r usage.Run, n float64) bool {
	got := float64(len(r.Containers()))
	if m.usage.PerContainer() {
		return got >= n
	}
	return got >= 1 && n == 1
}

// containerSeconds is the run's containers' time: each container's share of
// the run's duration, summed.
func containerSeconds(r usage.Run) float64 {
	total := 0.0
	for _, share := range shares(r.Containers()) {
		total += r.DurationSeconds * share
	}
	return total
}

// evidence is the job's successful, complete runs on the config's class (or
// the class remote Docker is known to run it on): what a prediction for the
// current config may use.
func (m *Module) evidence(job pipelineconfig.Job, u usage.Job) usage.Job {
	n, _ := parallelism(job)
	out := usage.Job{ResourceClass: u.ResourceClass}
	for _, r := range u.Runs {
		if len(r.Containers()) > 0 && r.Succeeded() && explained(job, r.Class(u)) &&
			(m.complete(r, n) || !m.usage.PerContainer()) {
			out.Runs = append(out.Runs, r)
		}
	}
	return out
}

// explained says whether a run's class is the config's, or the one remote
// Docker is known to run the job on.
func explained(job pipelineconfig.Job, runClass string) bool {
	class, run := canonical(configClass(job)), canonical(runClass)
	return run == class || (remoteDocker(job) && run == remoteDockerClass(class))
}

// canonical spells a class the way configs do: the usage API names
// generations with a dash (xlarge-gen2), configs with a dot (xlarge.gen2).
func canonical(class string) string {
	for _, gen := range []string{"-gen2", "-gen3"} {
		if base, ok := strings.CutSuffix(class, gen); ok {
			return base + "." + gen[1:]
		}
	}
	return class
}

// remoteDockerClass is the class gen1 remote Docker runs a job on: small as
// medium, medium and larger as large. Not yet calibrated: from CircleCI's
// resource-class guidance, not measured here.
func remoteDockerClass(class string) string {
	switch class {
	case "small":
		return "medium"
	case "medium", "medium+", "large":
		return "large"
	}
	return class
}

// shares is each container's share of the run's duration: its samples over
// the longest container's.
func shares(containers []usage.Execution) []float64 {
	longest := 0
	for _, c := range containers {
		longest = max(longest, len(c.CPUPct))
	}
	out := make([]float64, len(containers))
	for i, c := range containers {
		if longest > 0 {
			out[i] = float64(len(c.CPUPct)) / float64(longest)
		}
	}
	return out
}

// remoteDocker says whether the job starts remote Docker, which the platform
// may run on another class than the job's.
func remoteDocker(job pipelineconfig.Job) bool {
	for _, st := range job.Steps() {
		if st.Type == "setup_remote_docker" {
			return true
		}
	}
	return false
}

const secondsPerMinute = 60.0

// measuredOn names the classes the job's runs used.
func measuredOn(u usage.Job) string {
	seen := map[string]bool{}
	var out []string
	for _, r := range u.Runs {
		if c := r.Class(u); c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return strings.Join(out, ", ")
}

func configClass(job pipelineconfig.Job) string {
	if rc := job.Get("resource_class"); rc != nil {
		return rc.Value
	}
	return "medium"
}

// parallelism is the job's container count; false when it is set but not a
// positive integer.
func parallelism(job pipelineconfig.Job) (float64, bool) {
	p := job.Get("parallelism")
	if p == nil {
		return 1, true
	}
	n, err := strconv.Atoi(p.Value)
	if err != nil || n < 1 {
		return 0, false
	}
	return float64(n), true
}

// stats are what the samples show, and the slowdown they predict on a class
// with target vCPUs.
type stats struct {
	runs          int
	medianSeconds float64
	maxSaturation float64 // the highest share of saturated samples in a run
	peakMemoryPct float64
	meanCores     float64 // over every sample
	p90Cores      float64
	overTarget    float64 // share of samples whose demand exceeds the target's vCPUs
	slowdown      float64 // predicted container-seconds on the target, as a multiple: what credits follow
	wall          float64 // predicted wall-clock on the target, as a multiple: the longest container
	memSamples    int
	perContainer  bool // every parallel container was measured
}

// measure reads the runs. It predicts the slowdown on a class with target
// vCPUs: each 15-second sample takes max(1, cores / target) intervals, and a
// container's slowdown is the mean of that. Credits follow the containers'
// time, as a ratio of sums over runs, Σ duration × Σ share × slowdown over
// Σ duration × Σ share; the wall clock follows each run's longest container,
// weighted by duration. Both are expected costs, which a median would
// understate for occasional heavy runs. With dropFirst the first sample
// of each container is left out of the evidence (it is usually skewed); the
// shares always use every sample.
func measure(u usage.Job, vcpus, target float64, dropFirst bool) stats {
	var s stats
	var durations, cores []float64
	credit, base, wall, weight := 0.0, 0.0, 0.0, 0.0
	for _, r := range u.Runs {
		containers := r.Containers()
		if len(containers) == 0 {
			continue
		}
		s.runs++
		durations = append(durations, r.DurationSeconds)
		before, after, longest := 0.0, 0.0, 0.0
		part := shares(containers)
		for i, c := range containers {
			cpu, mem := c.CPUPct, c.MemoryPct
			if dropFirst && len(cpu) > 1 {
				cpu = cpu[1:]
			}
			if dropFirst && len(mem) > 1 {
				mem = mem[1:]
			}
			if len(cpu) == 0 {
				continue
			}
			saturated, stretch := 0, 0.0
			for _, v := range cpu {
				if v > SaturatedCPUPct {
					saturated++
				}
				demand := v / percent * vcpus
				cores = append(cores, demand)
				stretch += max(1, demand/target)
			}
			// TODO: measure saturation over the job's work phase only. The
			// samples do not mark it, so every sample counts.
			s.maxSaturation = max(s.maxSaturation, float64(saturated)/float64(len(cpu)))
			slow := stretch / float64(len(cpu))
			before += part[i]
			after += part[i] * slow
			longest = max(longest, part[i]*slow)
			for _, v := range mem {
				s.peakMemoryPct = max(s.peakMemoryPct, v)
			}
			s.memSamples += len(mem)
		}
		if before > 0 {
			credit += r.DurationSeconds * after
			base += r.DurationSeconds * before
			wall += r.DurationSeconds * longest
			weight += r.DurationSeconds
		}
		s.perContainer = s.perContainer || len(r.Executions) > 0
	}
	s.medianSeconds = median(durations)
	s.slowdown, s.wall = 1, 1
	if base > 0 && weight > 0 {
		s.slowdown, s.wall = credit/base, wall/weight
	}
	if len(cores) > 0 {
		total, over := 0.0, 0
		for _, c := range cores {
			total += c
			if c > target {
				over++
			}
		}
		s.meanCores = total / float64(len(cores))
		s.overTarget = float64(over) / float64(len(cores))
		slices.Sort(cores)
		s.p90Cores = cores[len(cores)*9/10]
	}
	return s
}

// median of the values, the mean of the middle two for an even count.
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	slices.Sort(v)
	mid := len(v) / halves
	if len(v)%halves != 0 {
		return v[mid]
	}
	return (v[mid-1] + v[mid]) / halves
}

// halves splits a sorted list at its middle.
const halves = 2

// metrics are the numbers behind a verdict, for tools such as the batch
// report (shares as fractions).
func metrics(s stats, p *prediction) map[string]float64 {
	if s.runs == 0 {
		return nil
	}
	m := map[string]float64{
		"runs": float64(s.runs), "median_seconds": s.medianSeconds, "mean_cores": s.meanCores, "p90_cores": s.p90Cores,
		"peak_memory_pct": s.peakMemoryPct, "max_saturation": s.maxSaturation,
	}
	if p != nil {
		m["over_target"] = s.overTarget
		m["slowdown"] = s.slowdown
		m["wall"] = s.wall
		m["rate_ratio"] = p.rateRatio
		m["job_saving"] = p.jobSaving
		m["conservative_saving"] = p.conservative
		m["share"] = p.share
		m["pipeline_saving"] = p.pipelineSaving
	}
	return m
}

// containerNote says how parallel containers were measured.
func containerNote(s stats) string {
	if s.perContainer {
		return "; every parallel container measured"
	}
	return "; from the busiest container only"
}

// prediction is what the rule expects of the downsize.
type prediction struct {
	rateRatio      float64 // the smaller class's credits per minute over the current one's
	jobSaving      float64 // 1 − rate ratio × slowdown
	conservative   float64 // jobSaving − ModelErrorMargin
	share          float64 // the job's share of the pipeline's credits
	pipelineSaving float64 // conservative × share
}

func (m *Module) judge(job pipelineconfig.Job, u usage.Job, pipe pipeline) finding.Finding {
	path := []string{"jobs", job.Name, "resource_class"}
	class := configClass(job)
	vcpus, ramGB, sized := m.pricing.Size(platform, class, generation)
	next, hasNext := m.pricing.LadderBelow(platform, class, generation)
	nextVCPUs, nextRAM, nextSized := m.pricing.Size(platform, next, generation)
	// The evidence is the successful, complete runs on the config's class;
	// until the classes are known it describes only itself, with no
	// prediction against a target.
	drop := m.usage.PerContainer()
	ev := m.evidence(job, u)
	s := measure(ev, vcpus, max(vcpus, 1), drop)
	noChange := func(code, why string) finding.Finding {
		return m.finding(job, path, VerdictNoChange, class, next, s, nil, code, why)
	}
	switch {
	case job.Get("docker") == nil:
		return noChange(CodeNotDocker, "only Docker classes are sized in this version")
	case len(ev.Runs) == 0 && measuredOn(u) != "":
		return noChange(CodeClassMismatch, "the usage was measured on "+measuredOn(u)+", but the config uses "+class)
	case !sized:
		return noChange(CodeUnsized, "the pricing table has no size for "+class)
	case vcpus < MinVCPUs:
		return noChange(CodeSmallClass, fmt.Sprintf("%s has %g vCPUs; below %g the rule does not downsize", class, vcpus, MinVCPUs))
	case !hasNext || !nextSized || nextVCPUs <= 0:
		return noChange(CodeNoSmaller, "the pricing table has no sized class below "+class)
	case s.runs < MinRuns:
		return noChange(CodeFewRuns, fmt.Sprintf("%d runs with samples; the rule needs at least %d", s.runs, MinRuns))
	case s.medianSeconds <= MinMedianSeconds:
		return noChange(CodeShortJob, fmt.Sprintf("the median run is %.0fs; runs under %gs have too few samples", s.medianSeconds, MinMedianSeconds))
	case s.memSamples == 0:
		return noChange(CodeNoMemory, "the runs have no memory samples, so the memory condition cannot be checked")
	case s.peakMemoryPct/percent*ramGB > nextRAM*MemoryHeadroom:
		return noChange(CodeMemory, fmt.Sprintf("the memory peak (%.0f%% of %g GB) does not fit %s's %g GB with headroom",
			s.peakMemoryPct, ramGB, next, nextRAM))
	}
	s = measure(ev, vcpus, nextVCPUs, drop)
	cur, curOK := m.pricing.CreditsPerMinute(platform, class, generation)
	nw, nwOK := m.pricing.CreditsPerMinute(platform, next, generation)
	own, ownOK := m.jobCredits(job)
	if !curOK || !nwOK {
		return noChange(CodeUnsized, "the pricing table has no rate for "+class+" or "+next)
	}
	if !ownOK && MinPipelineSaving > 0 {
		return noChange(CodeShareUnknown, "the job's own credits cannot be estimated from complete runs, so its share of the pipeline's is unknown")
	}
	// With no materiality threshold no share is needed; otherwise an
	// incomplete pipeline leaves the share unknown.
	if MinPipelineSaving > 0 && (len(pipe.missing) > 0 || pipe.measured <= 0) {
		return noChange(CodeShareUnknown, "the pipeline's credits cannot be estimated without "+strings.Join(pipe.missing, ", ")+
			", so the saving's share of them is unknown")
	}
	p := &prediction{rateRatio: nw / cur}
	// The job's share counts its compute credits, which the downsize
	// changes, against the whole pipeline's, flat charges included.
	if pipe.measured > 0 {
		p.share = own.measured / (pipe.measured + pipe.flat)
	}
	p.jobSaving = 1 - p.rateRatio*s.slowdown
	p.conservative = p.jobSaving - ModelErrorMargin
	p.pipelineSaving = max(p.conservative, 0) * p.share
	if pipe.configured > 0 && MinPipelineSaving > 0 {
		// The verdict must not depend on how unexplained mismatches are
		// priced.
		alt := max(p.conservative, 0) * own.configured / (pipe.configured + pipe.flat)
		if (p.pipelineSaving >= MinPipelineSaving) != (alt >= MinPipelineSaving) {
			return noChange(CodeShareUnknown, fmt.Sprintf("the verdict depends on how %s are priced: their usage was measured on "+
				"another class than the config names", strings.Join(pipe.mismatched, ", ")))
		}
	}
	switch {
	case p.conservative <= 0:
		return m.finding(job, path, VerdictNoChange, class, next, s, p, CodeRightSized, fmt.Sprintf(
			"no worthwhile one-class downsize: on %s it is predicted %.2f× slower, so it saves %.0f%% of its credits, "+
				"nothing once the %.0f-point model margin is taken off", next, s.slowdown, 100*p.jobSaving, 100*ModelErrorMargin))
	case p.pipelineSaving < MinPipelineSaving:
		return m.finding(job, path, VerdictNoChange, class, next, s, p, CodeImmaterial, fmt.Sprintf(
			"on %s it saves about %.0f%% of its credits (%.0f%% after the margin), but it is %.0f%% of the pipeline's credits, "+
				"so the pipeline saves about %.1f%%, under the %.0f%% worth validating",
			next, 100*p.jobSaving, 100*p.conservative, 100*p.share, 100*p.pipelineSaving, 100*MinPipelineSaving))
	}
	return m.finding(job, path, VerdictDownsize, class, next, s, p, "", "")
}

func (m *Module) finding(job pipelineconfig.Job, path []string, v finding.Verdict, class, next string, s stats, p *prediction, code, why string) finding.Finding {
	target := finding.Target{Job: job.Name, Path: path}
	ev := []finding.Evidence{
		{Kind: finding.EvidenceConfigFact, Claim: "rule", Source: "resourceclass rule",
			Value: Rule + " (resourceclass " + Version + ")" + fmt.Sprintf(": predict the slowdown one class down from each sample's CPU demand, take %.0f points off the "+
				"predicted saving, and propose the downsize only if it saves at least %.0f%% of the pipeline's estimated credits",
				100*ModelErrorMargin, 100*MinPipelineSaving)},
		{Kind: finding.EvidenceTelemetry, Claim: "observed", Source: "usage file",
			Value: fmt.Sprintf("%d runs with samples, median %.0fs; mean %.1f and p90 %.1f cores used; memory peak %.0f%%; "+
				"at most %.0f%% of a run's samples saturated", s.runs, s.medianSeconds, s.meanCores, s.p90Cores, s.peakMemoryPct, 100*s.maxSaturation)},
	}
	if p != nil {
		ev = append(ev, finding.Evidence{Kind: finding.EvidenceTelemetry, Claim: "predicted", Source: "usage file and pricing table",
			Value: fmt.Sprintf("on %s: %.0f%% of samples need more than its cores, %.2f× the container time and %.2f× the wall clock, "+
				"%.0f%% of the job's credits saved (%.0f%% after the margin); the job is %.0f%% of the pipeline's credits, so the pipeline "+
				"saves about %.1f%%%s", next, 100*s.overTarget, s.slowdown, s.wall, 100*p.jobSaving, 100*p.conservative, 100*p.share,
				100*p.pipelineSaving, containerNote(s))})
	}
	if s.maxSaturation >= SaturatedCPUPct/percent {
		ev = append(ev, finding.Evidence{Kind: finding.EvidenceTelemetry, Claim: "caveat", Source: "usage file",
			Value: "a run was often at the ceiling of " + class + ": its demand may be higher than measured, so the prediction is optimistic"})
	}
	f := finding.Finding{
		ID: finding.NewID(Name, target), Module: Name, Verdict: v, Target: target, Evidence: ev,
		Impact:  finding.Impact{Time: finding.DirectionUnknown, Cost: finding.DirectionUnknown, Referent: finding.ReferentCurrentState},
		Metrics: metrics(s, p),
		Keys: map[string]string{"fingerprint": fingerprint(job), "fingerprint_version": FingerprintVersion,
			"class": class, "rule": Rule, "version": Version},
	}
	if v == VerdictNoChange {
		f.Codes = []string{code}
		f.Confidence = finding.Confidence{Level: finding.ConfidenceHigh, Reason: "missing or failing evidence means no change"}
		f.Suggestion = finding.Suggestion{Op: finding.OpNone, Note: "No change: " + why + "."}
		if p != nil {
			f.Confidence = finding.Confidence{Level: finding.ConfidenceMedium, Reason: "the prediction and its thresholds are uncalibrated"}
		}
		return f
	}
	f.Impact.Referent = finding.ReferentProposedChange
	f.Impact.Cost = finding.DirectionBetter
	f.Impact.Time = finding.DirectionNeutral
	if s.slowdown > 1 {
		f.Impact.Time = finding.DirectionWorse
	}
	f.Impact.Basis = fmt.Sprintf("predicted %.2f× the duration on %s at %.2f× the rate: about %.0f%% of the job's credits, and %.1f%% of "+
		"the pipeline's after the model margin", s.slowdown, next, p.rateRatio, 100*p.jobSaving, 100*p.pipelineSaving)
	f.Confidence = finding.Confidence{Level: finding.ConfidenceMedium,
		Reason: "the prediction is uncalibrated; paired real runs decide"}
	// A class inherited from a shared executor or a merge key is set on the
	// job itself. The evidence is measured usage, not the steps.
	f.Suggestion = finding.Suggestion{Op: finding.OpSet, Value: next, Override: true, FromUsage: true,
		Note: strings.Join([]string{
			"Step down one class: " + class + " → " + next + ".",
			fmt.Sprintf("It is predicted %.2f× slower there, saving about %.0f%% of its credits and %.1f%% of the pipeline's after the model margin.",
				s.slowdown, 100*p.jobSaving, 100*p.pipelineSaving),
			"Confirm with paired runs before keeping it.",
		}, " ")}
	return f
}
