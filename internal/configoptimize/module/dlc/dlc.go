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

// Package dlc finds Docker Layer Caching enabled on jobs that build no
// images. DLC is a flat charge per job run and only caches
// images the job builds itself, so on a job that builds none it buys nothing.
//
// Config-only: it needs no telemetry and works on a pasted pipelineconfig.
package dlc

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/data"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
)

// Name is the module identifier.
const Name = "dlc"

// Verdicts. A detected build does not prove DLC helps (a remote builder or
// --no-cache can make it useless), so the verdict says only what was seen.
const (
	VerdictUnused        finding.Verdict = "DLC_UNUSED"
	VerdictBuildDetected finding.Verdict = "DLC_BUILD_DETECTED"
	VerdictUnverifiable  finding.Verdict = "DLC_UNVERIFIABLE"
)

// flatChargeKind is the pricing table key for the DLC charge.
const flatChargeKind = "dlc"

// Module is the dlc analyzer.
type Module struct {
	pricing pricing.Provider
}

// New returns the module. It does nothing else, so modules stay pure.
func New(p pricing.Provider) *Module {
	return &Module{pricing: p}
}

// Name implements module.Analyzer.
func (*Module) Name() string { return Name }

// Requires implements module.Analyzer: nothing.
func (*Module) Requires() []data.Requirement { return nil }

// Evaluate implements module.Analyzer. It emits one finding per job with DLC
// enabled. DLC is one setting per job run however many places turn it on,
// so every enabling location belongs to that one finding, and removing DLC
// means removing all of them together.
func (m *Module) Evaluate(_ context.Context, cfg *pipelineconfig.Effective, _ data.View) ([]finding.Finding, error) {
	var out []finding.Finding
	for _, job := range cfg.Jobs {
		locations := enabledAt(job)
		if len(locations) == 0 {
			continue
		}
		out = append(out, m.finding(job, locations, inspect(job)))
	}
	return out, nil
}

// location is one place DLC is turned on.
type location struct {
	path []string
	how  string
}

// enabledAt returns every place DLC is enabled on a job: job level, under
// machine, or on a setup_remote_docker step.
func enabledAt(job pipelineconfig.Job) []location {
	var out []location
	if pipelineconfig.IsTrue(job.Get("docker_layer_caching")) {
		out = append(out, location{path: []string{"jobs", job.Name, "docker_layer_caching"}, how: "at job level"})
	}
	if pipelineconfig.IsTrue(job.Get("machine", "docker_layer_caching")) {
		out = append(out, location{path: []string{"jobs", job.Name, "machine", "docker_layer_caching"}, how: "on the machine executor"})
	}
	for _, step := range job.Steps() {
		if step.Type == "setup_remote_docker" && pipelineconfig.IsTrue(pipelineconfig.MapGet(step.Body, "docker_layer_caching")) {
			path := append(job.StepPath(step.Index), "setup_remote_docker", "docker_layer_caching")
			out = append(out, location{path: path, how: "on setup_remote_docker (step " + strconv.Itoa(step.Index) + ")"})
		}
	}
	return out
}

// inspection is what the job's steps revealed.
type inspection struct {
	steps  int
	builds []string // "step 2: docker build"
	opaque []stepOpaque
}

type stepOpaque struct {
	step int // jobLevel for the job's own settings
	opaque
}

// jobLevel marks evidence about the job rather than one step.
const jobLevel = -1

// execEnvOf returns one entry per variable in an environment setting that
// can change what programs run (BASH_ENV, PATH, LD_PRELOAD, GOFLAGS=-toolexec,
// ...). The compiler emits job environments as a list of single-key maps, so
// mappings, lists of mappings, and KEY=VALUE strings are all read.
func execEnvOf(env *yaml.Node) []opaque {
	var out []opaque
	add := func(name, value string) {
		if isExecEnvValue(name, value) {
			out = append(out, opaque{CodeShellState, "environment sets " + name + ", which can change what the steps run"})
		}
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		switch {
		case n == nil:
		case n.Kind == yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				add(n.Content[i].Value, n.Content[i+1].Value)
			}
		case n.Kind == yaml.SequenceNode:
			for _, c := range n.Content {
				walk(c)
			}
		case n.Kind == yaml.ScalarNode:
			if name, value, ok := strings.Cut(n.Value, "="); ok {
				add(name, value)
			}
		}
	}
	walk(env)
	return out
}

// stepsWithoutCommands are builtin steps that run no user command.
var stepsWithoutCommands = []string{
	"add_ssh_keys", "attach_workspace", "checkout", "persist_to_workspace",
	"restore_cache", "save_cache", "setup_remote_docker", "store_artifacts",
	"store_test_results",
}

func inspect(job pipelineconfig.Job) inspection {
	var res inspection
	jobShell := ""
	if s := job.Get("shell"); s != nil {
		jobShell = s.Value
	}
	// Environment set for the whole job, including on docker images.
	envs := []*yaml.Node{job.Get("environment")}
	if images := job.Get("docker"); images != nil && images.Kind == yaml.SequenceNode {
		for _, img := range images.Content {
			envs = append(envs, pipelineconfig.MapGet(img, "environment"))
		}
	}
	for _, env := range envs {
		for _, o := range execEnvOf(env) {
			res.opaque = append(res.opaque, stepOpaque{jobLevel, o})
		}
	}
	for _, step := range job.Steps() {
		res.steps++
		switch {
		case step.Type == "run":
			for _, o := range execEnvOf(pipelineconfig.MapGet(step.Body, "environment")) {
				res.opaque = append(res.opaque, stepOpaque{step.Index, o})
			}
			command, shell := runCommand(step.Body)
			if shell == "" {
				shell = jobShell
			}
			if !shellSupported(shell) {
				res.opaque = append(res.opaque, stepOpaque{step.Index, opaque{CodeUnsupportedShell, "runs under the shell " + shell}})
				continue
			}
			s := scanScript(command)
			for _, b := range s.builds {
				res.builds = append(res.builds, "step "+strconv.Itoa(step.Index)+": "+b)
			}
			for _, o := range s.opaque {
				res.opaque = append(res.opaque, stepOpaque{step.Index, o})
			}
		case slices.Contains(stepsWithoutCommands, step.Type):
		default:
			res.opaque = append(res.opaque, stepOpaque{step.Index, opaque{CodeUnsupportedStep, "step type " + step.Type + " cannot be inspected"}})
		}
	}
	return res
}

// runCommand returns a run step's command and shell. The compiler normalizes
// `- run: cmd` to a mapping, but both forms are accepted.
func runCommand(body *yaml.Node) (command, shell string) {
	if body == nil {
		return "", ""
	}
	if body.Kind == yaml.ScalarNode {
		return body.Value, ""
	}
	if c := pipelineconfig.MapGet(body, "command"); c != nil {
		command = c.Value
	}
	if s := pipelineconfig.MapGet(body, "shell"); s != nil {
		shell = s.Value
	}
	return command, shell
}

func (m *Module) finding(job pipelineconfig.Job, locations []location, res inspection) finding.Finding {
	target := finding.Target{Path: []string{"jobs", job.Name}, Job: job.Name}
	paths := make([][]string, 0, len(locations))
	f := finding.Finding{
		ID:     finding.NewID(Name, target),
		Module: Name,
		Target: target,
		Impact: m.impact(job),
	}
	for _, loc := range locations {
		paths = append(paths, loc.path)
		f.Evidence = append(f.Evidence, finding.Evidence{
			Kind:   finding.EvidenceConfigFact,
			Claim:  "DLC is enabled " + loc.how,
			Value:  "docker_layer_caching: true",
			Source: "config:" + strings.Join(loc.path, "."),
		})
	}
	stepsSource := "config:jobs." + job.Name + ".steps"

	switch {
	case len(res.builds) > 0:
		f.Verdict = VerdictBuildDetected
		f.Impact.Cost = finding.DirectionNeutral
		f.Impact.Time = finding.DirectionNeutral
		f.Confidence = finding.Confidence{Level: finding.ConfidenceHigh}
		for _, b := range res.builds {
			f.Evidence = append(f.Evidence, finding.Evidence{
				Kind: finding.EvidenceConfigFact, Claim: "the job builds an image", Value: b, Source: stepsSource,
			})
		}
		f.Suggestion = finding.Suggestion{
			Op:   finding.OpNone,
			Note: "The job builds images, so DLC may pay for itself. A build does not prove it helps: a remote builder or --no-cache makes DLC useless.",
		}
	case len(res.opaque) > 0:
		f.Verdict = VerdictUnverifiable
		f.Impact.Cost = finding.DirectionUnknown
		f.Impact.Time = finding.DirectionUnknown
		f.Confidence = finding.Confidence{
			Level:  finding.ConfidenceLow,
			Reason: "some steps cannot be inspected, so a build may be hidden",
		}
		for _, o := range res.opaque {
			f.Evidence = append(f.Evidence, finding.Evidence{
				Kind:   finding.EvidenceConfigFact,
				Claim:  "a step cannot be inspected (" + string(o.code) + ")",
				Value:  stepLabel(o.step) + ": " + o.why,
				Source: stepsSource,
			})
			if !slices.Contains(f.Codes, string(o.code)) {
				f.Codes = append(f.Codes, string(o.code))
			}
		}
		slices.Sort(f.Codes)
		f.Suggestion = finding.Suggestion{
			Op:   finding.OpNone,
			Note: "No visible step builds an image, but some steps cannot be inspected. Check them before removing DLC: removing it from a job that builds images makes every run slower.",
		}
	default:
		f.Verdict = VerdictUnused
		f.Impact.Cost = finding.DirectionBetter
		f.Impact.Time = finding.DirectionNeutral
		f.Confidence = finding.Confidence{Level: finding.ConfidenceHigh}
		f.Evidence = append(f.Evidence, finding.Evidence{
			Kind:   finding.EvidenceConfigFact,
			Claim:  "every step was inspected and none builds an image",
			Value:  fmt.Sprintf("%d steps inspected", res.steps),
			Source: stepsSource,
		})
		f.Suggestion = finding.Suggestion{
			Op:    finding.OpRemove,
			Paths: paths,
			// docker_layer_caching defaults to false.
			FalseMeansAbsent: true,
			Note:             "No step in this job invokes a docker image build. DLC is charged per job run regardless.",
		}
	}
	return f
}

// impact carries the per-run charge from pricing. There is no run count in
// v1 (no telemetry), so there is no credit total: a direction with no
// magnitude is honest and still actionable.
func (m *Module) impact(job pipelineconfig.Job) finding.Impact {
	imp := finding.Impact{Referent: finding.ReferentProposedChange, Unit: "job run"}
	rate, ok := m.pricing.FlatCharge(flatChargeKind)
	if !ok {
		imp.Basis = "DLC is a flat charge per job run; no rate was given (pass --credit-rates), and no run count is available"
		return imp
	}
	imp.UnitCredits = &rate
	// TODO: is DLC charged per job run or per parallel run? Until that is
	// known it is charged once per job run, so with parallelism the figure
	// may be low.
	imp.Basis = fmt.Sprintf("DLC is a flat %s credits per job run (charged once per job run); no run count is available", formatCredits(rate))
	if p := job.Get("parallelism"); p != nil {
		if n, err := strconv.Atoi(p.Value); err == nil && n > 1 {
			imp.Basis += fmt.Sprintf("; with parallelism %d the charge may be higher", n)
		}
	}
	return imp
}

func stepLabel(step int) string {
	if step == jobLevel {
		return "job"
	}
	return "step " + strconv.Itoa(step)
}

func formatCredits(c float64) string {
	return strconv.FormatFloat(c, 'f', -1, 64)
}
