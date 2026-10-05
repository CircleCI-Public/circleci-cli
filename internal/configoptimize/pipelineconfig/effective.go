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

package pipelineconfig

import (
	"fmt"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Effective is the compiled config: orbs inlined, executors and commands
// resolved, parameters substituted, matrix jobs expanded. Modules read it and
// nothing else.
type Effective struct {
	// Jobs in name order.
	Jobs []Job
	// Setup is true for a setup (dynamic) config, whose real config is
	// generated at runtime. The compiler drops `setup: true`, so it is read
	// from the authored copy when the Mutability is built.
	Setup bool

	// tracer follows a compiled value back to the authored config. It is
	// set when the Mutability for this copy is built; modules reach it
	// only through Trace, never the authored bytes.
	tracer *Mutability
}

// Trace follows the compiled scalar at path (e.g. ["jobs", "build", "steps",
// "1", "restore_cache", "keys", "0"]) back to the authored config and
// resolves every substitution in it. Without an authored copy
// the trace is broken, which makes the value uninspectable.
func (e *Effective) Trace(job string, path []string) Trace {
	if e.tracer == nil {
		return Trace{Problem: "no authored config"}
	}
	var n *yaml.Node
	for _, j := range e.Jobs {
		if j.Name == job {
			n = j.Node
		}
	}
	for _, part := range path[min(2, len(path)):] {
		n = deref(n)
		switch {
		case n == nil:
		case n.Kind == yaml.MappingNode:
			n = MapGet(n, part)
		case n.Kind == yaml.SequenceNode:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(n.Content) {
				n = nil
				break
			}
			n = n.Content[i]
		default:
			n = nil
		}
	}
	return e.tracer.Trace(job, path, deref(n))
}

// Job is one effective job.
type Job struct {
	Name string
	// Node is the job's mapping node in the compiled YAML.
	Node *yaml.Node
}

// Step is one effective step.
type Step struct {
	// Index is the step's position in the job's steps sequence.
	Index int
	// Type is the step's key, e.g. "run", "checkout", "setup_remote_docker".
	Type string
	// Body is the step's value node: a mapping, or a scalar for shorthand
	// forms such as `- run: make test`. Nil for a bare `- checkout`.
	Body *yaml.Node
	// Node is the whole step node.
	Node *yaml.Node
}

// NewEffective reads compiled YAML.
func NewEffective(compiled []byte) (*Effective, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(compiled, &doc); err != nil {
		return nil, fmt.Errorf("read compiled config: %w", err)
	}
	eff := &Effective{}
	if len(doc.Content) == 0 {
		return eff, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("read compiled config: top level is not a mapping")
	}
	jobs := MapGet(root, "jobs")
	if jobs == nil {
		return eff, nil
	}
	if jobs.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("read compiled config: jobs is not a mapping")
	}
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		eff.Jobs = append(eff.Jobs, Job{Name: jobs.Content[i].Value, Node: jobs.Content[i+1]})
	}
	slices.SortFunc(eff.Jobs, func(a, b Job) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		}
		return 0
	})
	return eff, nil
}

// Get walks nested mapping keys from the job node. It returns nil when any
// key is absent.
func (j Job) Get(keys ...string) *yaml.Node {
	n := j.Node
	for _, k := range keys {
		n = MapGet(n, k)
		if n == nil {
			return nil
		}
	}
	return n
}

// Steps returns the job's steps in order.
func (j Job) Steps() []Step {
	seq := MapGet(j.Node, "steps")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	steps := make([]Step, 0, len(seq.Content))
	for i, n := range seq.Content {
		s := Step{Index: i, Node: n}
		switch n.Kind {
		case yaml.ScalarNode:
			s.Type = n.Value
		case yaml.MappingNode:
			if len(n.Content) >= 2 {
				s.Type = n.Content[0].Value
				s.Body = n.Content[1]
			}
		case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		}
		steps = append(steps, s)
	}
	return steps
}

// StepPath returns the effective path of a step, e.g.
// ["jobs", "build", "steps", "2"].
func (j Job) StepPath(index int) []string {
	return []string{"jobs", j.Name, "steps", strconv.Itoa(index)}
}

// MapGet returns the value for key in a mapping node, following an alias to
// its target. It returns nil when n is not a mapping or has no such key.
// Merge keys are not followed: callers that care must check for them.
func MapGet(n *yaml.Node, key string) *yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return deref(n.Content[i+1])
		}
	}
	return nil
}

// DockerLayerCaching reports whether the job enables DLC: on the job, on its
// machine executor, or in a setup_remote_docker step.
func (j Job) DockerLayerCaching() bool {
	if IsTrue(j.Get("docker_layer_caching")) || IsTrue(j.Get("machine", "docker_layer_caching")) {
		return true
	}
	for _, s := range j.Steps() {
		if s.Type == "setup_remote_docker" && IsTrue(MapGet(s.Body, "docker_layer_caching")) {
			return true
		}
	}
	return false
}

// IsTrue reports whether a node is the boolean true.
func IsTrue(n *yaml.Node) bool {
	n = deref(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	var b bool
	return n.Decode(&b) == nil && b
}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}
