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

package cache

import (
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Kind is which cache step a Step is.
type Kind int

const (
	// Restore is a restore_cache step: a restore policy.
	Restore Kind = iota
	// Save is a save_cache step.
	Save
)

// Step is one save_cache or restore_cache step.
type Step struct {
	Kind  Kind
	Job   string
	Index int
	// Path is the step's effective path, e.g. ["jobs", "build", "steps", "3"].
	Path []string
	// Keys are the step's keys in order, as compiled. A restore step's keys
	// are one policy: its fallbacks decide whether it can match.
	Keys []Key
	// Paths are what a save step saves.
	Paths []string
	// Problem is set when the step's body could not be read (no key, a key
	// that is not a string, ...). Such a step is counted, never judged.
	Problem string
}

// Key is one compiled key template.
type Key struct {
	Raw string
	// Path is the key's effective path, for tracing.
	Path  []string
	atoms []atom
}

// changingToken reports whether the compiled key has a scoped runtime token
// ({{ epoch }}, {{ .BuildNum }}, {{ .Revision }}, …). Such a token is
// evaluated when the step runs, so a save with the same token at the same
// place changes with it.
func (k Key) changingToken() bool {
	for _, a := range k.atoms {
		if a.tok != nil && a.tok.Scope() != Unscoped {
			return true
		}
	}
	return false
}

// Edge says a restore policy's key may match a save step's key.
type Edge struct {
	Policy *Step
	Key    int // index into Policy.Keys
	Save   *Step
	Exact  bool
	// Ambiguous: a literal meets a token, or a token argument differs.
	Ambiguous bool
	// Broad: a prefix key that may match saves with different templates.
	// Flagged, never merged: such saves may be different caches.
	Broad bool
}

// Job is one job's view of the model.
type Job struct {
	Name  string
	Steps []*Step
	// UnknownSteps are indexes of steps whose type the model does not know,
	// which might cache.
	UnknownSteps []int
}

// Model is every cache step of a compiled config and the may-match graph.
type Model struct {
	Jobs  []Job
	Saves []*Step
	// Policies are the restore steps.
	Policies []*Step
	Edges    []Edge
}

// knownSteps are the built-in step types that never cache implicitly.
var knownSteps = []string{
	"run", "checkout", "setup_remote_docker", "store_artifacts", "store_test_results",
	"persist_to_workspace", "attach_workspace", "add_ssh_keys", "deploy",
}

// Build reads the model from the compiled config.
func Build(cfg *pipelineconfig.Effective) Model {
	var m Model
	for _, job := range cfg.Jobs {
		j := Job{Name: job.Name}
		for _, st := range job.Steps() {
			switch {
			case st.Type == "save_cache" || st.Type == "restore_cache":
				s := readStep(job, st)
				j.Steps = append(j.Steps, s)
				switch {
				case s.Problem != "":
				case s.Kind == Restore:
					m.Policies = append(m.Policies, s)
				default:
					m.Saves = append(m.Saves, s)
				}
			case !slices.Contains(knownSteps, st.Type):
				j.UnknownSteps = append(j.UnknownSteps, st.Index)
			}
		}
		m.Jobs = append(m.Jobs, j)
	}
	for _, p := range m.Policies {
		for ki, k := range p.Keys {
			var edges []Edge
			templates := map[string]bool{}
			for _, s := range m.Saves {
				switch compare(k.atoms, s.Keys[0].atoms) {
				case exactMatch:
					edges = append(edges, Edge{Policy: p, Key: ki, Save: s, Exact: true})
					templates[s.Keys[0].Raw] = true
				case prefixMatch:
					edges = append(edges, Edge{Policy: p, Key: ki, Save: s})
					templates[s.Keys[0].Raw] = true
				case ambiguousMatch:
					edges = append(edges, Edge{Policy: p, Key: ki, Save: s, Ambiguous: true})
					templates[s.Keys[0].Raw] = true
				case noMatch:
				}
			}
			for i := range edges {
				edges[i].Broad = len(templates) > 1
			}
			m.Edges = append(m.Edges, edges...)
		}
	}
	return m
}

// EdgesOf returns the edges from one restore policy.
func (m Model) EdgesOf(p *Step) []Edge {
	var out []Edge
	for _, e := range m.Edges {
		if e.Policy == p {
			out = append(out, e)
		}
	}
	return out
}

func readStep(job pipelineconfig.Job, st pipelineconfig.Step) *Step {
	s := &Step{Job: job.Name, Index: st.Index, Path: job.StepPath(st.Index)}
	if st.Type == "save_cache" {
		s.Kind = Save
	}
	body := st.Body
	if body == nil || body.Kind != yaml.MappingNode {
		s.Problem = st.Type + " has no parameters"
		return s
	}
	add := func(n *yaml.Node, path ...string) {
		s.Keys = append(s.Keys, Key{Raw: n.Value, Path: append(append(slices.Clone(s.Path), st.Type), path...), atoms: atoms(n.Value)})
	}
	if k := pipelineconfig.MapGet(body, "key"); k != nil {
		if k.Kind != yaml.ScalarNode {
			s.Problem = "key is not a string"
			return s
		}
		add(k, "key")
	}
	if ks := pipelineconfig.MapGet(body, "keys"); ks != nil {
		if s.Kind == Save || ks.Kind != yaml.SequenceNode {
			s.Problem = "keys is not a list of strings on restore_cache"
			return s
		}
		for i, k := range ks.Content {
			if k.Kind != yaml.ScalarNode {
				s.Problem = "a key in keys is not a string"
				return s
			}
			add(k, "keys", strconv.Itoa(i))
		}
	}
	if len(s.Keys) == 0 {
		s.Problem = st.Type + " has no key"
		return s
	}
	if p := pipelineconfig.MapGet(body, "paths"); p != nil && p.Kind == yaml.SequenceNode {
		for _, n := range p.Content {
			s.Paths = append(s.Paths, n.Value)
		}
	}
	return s
}
