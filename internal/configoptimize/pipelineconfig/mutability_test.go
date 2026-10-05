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

package pipelineconfig_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// loadPair reads an authored fixture and its real compiler output, captured
// with `circleci config process <name>.yml > <name>.compiled.yml`.
func loadPair(t *testing.T, dir, name string) (*pipelineconfig.Authored, *pipelineconfig.Effective) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", dir, name+".yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	compiled, err := os.ReadFile(filepath.Join("testdata", dir, name+".compiled.yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	authored, err := pipelineconfig.Parse(src)
	assert.NilError(t, err)
	eff, err := pipelineconfig.NewEffective(compiled)
	assert.NilError(t, err)
	return authored, eff
}

// effectiveNode walks an effective path such as
// ["jobs", "build", "steps", "1", "setup_remote_docker", "docker_layer_caching"].
func effectiveNode(t *testing.T, eff *pipelineconfig.Effective, path []string) *yaml.Node {
	t.Helper()
	j := slices.IndexFunc(eff.Jobs, func(j pipelineconfig.Job) bool { return j.Name == path[1] })
	assert.Assert(t, j >= 0, "effective job %s not found", path[1])
	n := eff.Jobs[j].Node
	for _, part := range path[2:] {
		if i, err := strconv.Atoi(part); err == nil && n.Kind == yaml.SequenceNode {
			assert.Assert(t, i < len(n.Content), "index %d out of range", i)
			n = n.Content[i]
			continue
		}
		n = pipelineconfig.MapGet(n, part)
		assert.Assert(t, n != nil, "effective path %v not found at %q", path, part)
	}
	return n
}

func TestMutability(t *testing.T) {
	machineDLC := func(job string) []string { return []string{"jobs", job, "machine", "docker_layer_caching"} }
	stepDLC := func(job string, i int) []string {
		return []string{"jobs", job, "steps", strconv.Itoa(i), "setup_remote_docker", "docker_layer_caching"}
	}

	tests := []struct {
		fixture     string
		job         string
		path        []string
		wantLabel   pipelineconfig.Label
		wantAuthPth []string
	}{
		// Directly authored, owned by one job: the only writable shapes.
		{fixture: "writable", job: "build", path: machineDLC("build"),
			wantLabel: pipelineconfig.LabelWritable, wantAuthPth: machineDLC("build")},
		{fixture: "writable", job: "remote", path: stepDLC("remote", 1),
			wantLabel: pipelineconfig.LabelWritable, wantAuthPth: stepDLC("remote", 1)},
		// A step after an expanded command lines up counted from the end.
		{fixture: "commands", job: "after-command", path: stepDLC("after-command", 2),
			wantLabel: pipelineconfig.LabelWritable, wantAuthPth: stepDLC("after-command", 1)},
		// A step that came out of a reusable command is never writable.
		{fixture: "commands", job: "via-command-a", path: stepDLC("via-command-a", 1),
			wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "commands", job: "via-command-b", path: stepDLC("via-command-b", 0),
			wantLabel: pipelineconfig.LabelSharedSource},
		// An executor used by one job is written on the executor.
		{fixture: "executors", job: "alone", path: machineDLC("alone"),
			wantLabel: pipelineconfig.LabelWritable, wantAuthPth: []string{"executors", "solo", "machine", "docker_layer_caching"}},
		{fixture: "executors", job: "first", path: machineDLC("first"), wantLabel: pipelineconfig.LabelSharedExecutor},
		{fixture: "executors", job: "second", path: machineDLC("second"), wantLabel: pipelineconfig.LabelSharedExecutor},
		// Matrix instances all come from one definition.
		{fixture: "matrix", job: "test-18", path: machineDLC("test-18"), wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "matrix", job: "test-20", path: machineDLC("test-20"), wantLabel: pipelineconfig.LabelSharedSource},
		// Both ends of an anchor/alias pair are shared.
		{fixture: "anchors", job: "origin", path: machineDLC("origin"), wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "anchors", job: "copy", path: machineDLC("copy"), wantLabel: pipelineconfig.LabelSharedSource},
		// A value from a merge key is shared, and so is a direct key beside a
		// merge: removing it could expose the merged value instead.
		{fixture: "merge-keys", job: "merged", path: machineDLC("merged"), wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "merge-keys", job: "overridden", path: machineDLC("overridden"), wantLabel: pipelineconfig.LabelSharedSource},
		// Orb jobs and orb executors cannot be edited from the caller's file.
		{fixture: "orb", job: "builder/test", path: machineDLC("builder/test"), wantLabel: pipelineconfig.LabelOrbSourced},
		{fixture: "orb", job: "uses-orb-executor", path: machineDLC("uses-orb-executor"), wantLabel: pipelineconfig.LabelOrbSourced},
		// Parameters, repeated invocations and pre-steps.
		{fixture: "parameters", job: "param", path: machineDLC("param"), wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "parameters", job: "twice-a", path: machineDLC("twice-a"), wantLabel: pipelineconfig.LabelSharedSource},
		{fixture: "parameters", job: "wrapped", path: stepDLC("wrapped", 1), wantLabel: pipelineconfig.LabelSharedSource},
	}
	for _, tc := range tests {
		t.Run(tc.fixture+"/"+tc.job, func(t *testing.T) {
			authored, eff := loadPair(t, "mutability", tc.fixture)
			m := pipelineconfig.NewMutability(authored, eff)
			got := m.Resolve(tc.job, tc.path, effectiveNode(t, eff, tc.path))
			assert.Check(t, cmp.Equal(got.Label, tc.wantLabel), "reason: %s", got.Reason)
			assert.Check(t, cmp.DeepEqual(got.AuthoredPath, tc.wantAuthPth))
			if got.Label != pipelineconfig.LabelWritable {
				assert.Check(t, got.Reason != "", "a report-only label needs a reason")
			}
		})
	}
}

func TestMutabilityNeverMarksSharedShapesWritable(t *testing.T) {
	// Shared shapes are report-only, checked as a property over every DLC
	// location in the shared-shape fixtures: none may come out writable.
	for _, fixture := range []string{"matrix", "anchors"} {
		t.Run(fixture, func(t *testing.T) {
			authored, eff := loadPair(t, "mutability", fixture)
			m := pipelineconfig.NewMutability(authored, eff)
			for _, j := range eff.Jobs {
				path := []string{"jobs", j.Name, "machine", "docker_layer_caching"}
				got := m.Resolve(j.Name, path, effectiveNode(t, eff, path))
				assert.Check(t, got.Label != pipelineconfig.LabelWritable, "%s: %s", j.Name, got.Reason)
			}
		})
	}
}

func TestMutabilityMergeShadow(t *testing.T) {
	// A direct key beside a merge: removing it would expose the merged true.
	src := `
x-machine: &machine
  image: ubuntu-2204:current
  docker_layer_caching: true
jobs:
  j:
    machine:
      <<: *machine
      docker_layer_caching: true
    steps: [checkout]
workflows: {ci: {jobs: [j]}}
`
	authored, err := pipelineconfig.Parse([]byte(src))
	assert.NilError(t, err)
	eff, err := pipelineconfig.NewEffective([]byte("jobs:\n  j:\n    machine: {image: ubuntu-2204:current, docker_layer_caching: true}\n    steps: [checkout]\n"))
	assert.NilError(t, err)
	path := []string{"jobs", "j", "machine", "docker_layer_caching"}
	got := pipelineconfig.NewMutability(authored, eff).Resolve("j", path, effectiveNode(t, eff, path))
	assert.Check(t, cmp.Equal(got.Label, pipelineconfig.LabelSharedSource), got.Reason)
}

// Which inherited values a job may override with a key of its own.
func TestMutabilityOverride(t *testing.T) {
	src := `
version: 2.1
orbs:
  o: circleci/node@5.0.0
x-big: &big
  resource_class: 2xlarge
  machine:
    image: ubuntu-2204:current
    docker_layer_caching: true
executors:
  shared: {docker: [{image: cimg/base:current}], resource_class: 2xlarge}
  solo: {docker: [{image: cimg/base:current}], resource_class: 2xlarge}
jobs:
  merged:
    <<: *big
    steps: [checkout]
  merged-own:
    <<: *big
    resource_class: 2xlarge
    steps: [checkout]
  exec-a: {executor: shared, steps: [checkout]}
  exec-b: {executor: shared, steps: [checkout]}
  alone: {executor: solo, steps: [checkout]}
  orb-exec: {executor: o/default, steps: [checkout]}
  matrixed:
    <<: *big
    parameters: {v: {type: string}}
    steps: [checkout]
workflows:
  ci:
    jobs:
      - merged
      - merged-own
      - exec-a
      - exec-b
      - alone
      - orb-exec
      - matrixed: {matrix: {parameters: {v: [a, b]}}}
`
	job := func(name string) string {
		return "  " + name + ":\n    docker: [{image: cimg/base:current}]\n    resource_class: 2xlarge\n" +
			"    machine: {image: ubuntu-2204:current, docker_layer_caching: true}\n    steps: [checkout]\n"
	}
	compiled := "jobs:\n"
	for _, j := range []string{"merged", "merged-own", "exec-a", "exec-b", "alone", "orb-exec", "matrixed-a", "matrixed-b"} {
		compiled += job(j)
	}
	authored, err := pipelineconfig.Parse([]byte(src))
	assert.NilError(t, err)
	eff, err := pipelineconfig.NewEffective([]byte(compiled))
	assert.NilError(t, err)
	m := pipelineconfig.NewMutability(authored, eff)
	rc := func(j string) []string { return []string{"jobs", j, "resource_class"} }

	tests := []struct {
		name, job string
		path      []string
		want      *pipelineconfig.Override // nil: no override
		label     pipelineconfig.Label
	}{
		{name: "inherited from a merge key", job: "merged", path: rc("merged"),
			want: &pipelineconfig.Override{Path: rc("merged"), Insert: true}, label: pipelineconfig.LabelSharedSource},
		{name: "the job's own key beside a merge key", job: "merged-own", path: rc("merged-own"),
			want: &pipelineconfig.Override{Path: rc("merged-own"), Value: "2xlarge"}, label: pipelineconfig.LabelSharedSource},
		{name: "a shared executor", job: "exec-a", path: rc("exec-a"),
			want: &pipelineconfig.Override{Path: rc("exec-a"), Insert: true}, label: pipelineconfig.LabelSharedExecutor},
		{name: "an executor used by one job is edited in place", job: "alone", path: rc("alone"),
			label: pipelineconfig.LabelWritable},
		{name: "an orb executor", job: "orb-exec", path: rc("orb-exec"), label: pipelineconfig.LabelOrbSourced},
		{name: "a matrix instance", job: "matrixed-a", path: rc("matrixed-a"), label: pipelineconfig.LabelSharedSource},
		{name: "a nested key beside a merge key", job: "merged",
			path: []string{"jobs", "merged", "machine", "docker_layer_caching"}, label: pipelineconfig.LabelSharedSource},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Resolve(tc.job, tc.path, effectiveNode(t, eff, tc.path))
			assert.Check(t, cmp.Equal(got.Label, tc.label), got.Reason)
			assert.Check(t, cmp.DeepEqual(got.Override, tc.want))
		})
	}
}

func TestMutabilityFailsClosed(t *testing.T) {
	authored, eff := loadPair(t, "mutability", "writable")
	m := pipelineconfig.NewMutability(authored, eff)
	got := m.Resolve("build", []string{"jobs", "build", "machine", "docker_layer_caching"}, nil)
	assert.Check(t, cmp.Equal(got.Label, pipelineconfig.LabelUnmatched), "a missing effective node must never be writable")
}

func TestInputDependence(t *testing.T) {
	src := func(t *testing.T, body string) *pipelineconfig.Authored {
		t.Helper()
		a, err := pipelineconfig.Parse([]byte(body))
		assert.NilError(t, err)
		return a
	}
	effective := &pipelineconfig.Effective{Jobs: []pipelineconfig.Job{{Name: "build"}}}
	tests := []struct {
		name string
		yaml string
		want pipelineconfig.Dependence
	}{
		{name: "no pipeline values", want: pipelineconfig.DependenceNone, yaml: `
jobs:
  build:
    parameters: {v: {type: string, default: "1"}}
    steps: [{run: echo << parameters.v >>}]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value in a step", want: pipelineconfig.DependenceKnown, yaml: `
jobs:
  build:
    steps:
      - when:
          condition: << pipeline.parameters.x >>
          steps: [{run: docker build .}]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value in a local command", want: pipelineconfig.DependenceKnown, yaml: `
commands:
  outer: {steps: [inner]}
  inner: {steps: [{run: echo << pipeline.git.branch >>}]}
jobs:
  build:
    steps: [outer]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value in the executor", want: pipelineconfig.DependenceKnown, yaml: `
executors:
  e: {machine: {image: "<< pipeline.parameters.image >>"}}
jobs:
  build:
    executor: e
    steps: [checkout]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value with extra spaces", want: pipelineconfig.DependenceKnown, yaml: `
jobs:
  build:
    steps: [{run: "echo <<   pipeline.git.branch >>"}]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value in a pre-step command", want: pipelineconfig.DependenceKnown, yaml: `
commands:
  gate: {steps: [{run: echo << pipeline.git.tag >>}]}
jobs:
  build:
    steps: [checkout]
workflows: {ci: {jobs: [{build: {pre-steps: [gate]}}]}}
`},
		{name: "pipeline value in an inline orb command", want: pipelineconfig.DependenceKnown, yaml: `
orbs:
  tools:
    commands:
      maybe: {steps: [{run: echo << pipeline.git.branch >>}]}
jobs:
  build:
    steps: [tools/maybe]
workflows: {ci: {jobs: [build]}}
`},
		{name: "a registry orb command is never proven input-free", want: pipelineconfig.DependenceOpaqueOrb, yaml: `
orbs:
  node: circleci/node@7
jobs:
  build:
    steps: [checkout, node/install-packages]
workflows: {ci: {jobs: [build]}}
`},
		{name: "a visible pipeline value outranks an opaque orb", want: pipelineconfig.DependenceKnown, yaml: `
orbs:
  node: circleci/node@7
jobs:
  build:
    steps: [node/install-packages, {run: echo << pipeline.git.branch >>}]
workflows: {ci: {jobs: [build]}}
`},
		{name: "pipeline value passed by the invocation", want: pipelineconfig.DependenceKnown, yaml: `
jobs:
  build:
    parameters: {v: {type: string}}
    steps: [{run: echo << parameters.v >>}]
workflows: {ci: {jobs: [{build: {v: << pipeline.git.tag >>}}]}}
`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := pipelineconfig.NewMutability(src(t, tc.yaml), effective)
			got, why := m.InputDependence("build")
			assert.Check(t, cmp.Equal(got, tc.want), why)
			if got != pipelineconfig.DependenceNone {
				assert.Check(t, why != "")
			}
		})
	}
}
