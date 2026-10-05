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

package patch_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/patch"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

func TestCheckDelta(t *testing.T) {
	before := `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:current, docker_layer_caching: true}
    steps:
    - setup_remote_docker:
        docker_layer_caching: true
    - run: {command: echo hi}
`
	removed := plan.Delta{Removed: [][]string{
		{"jobs", "build", "machine", "docker_layer_caching"},
		{"jobs", "build", "steps", "0", "setup_remote_docker", "docker_layer_caching"},
	}, FalseMeansAbsent: true}

	tests := []struct {
		name  string
		after string
		want  string // empty: accepted
	}{
		{name: "exactly the keys removed, bare step", after: `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:current}
    steps:
    - setup_remote_docker
    - run: {command: echo hi}
`},
		{name: "mapping order does not matter", after: `
jobs:
  build:
    steps:
    - setup_remote_docker: {}
    - run: {command: echo hi}
    machine: {image: ubuntu-2204:current}
version: 2
`},
		{name: "an explicit false counts as absent", after: `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:current, docker_layer_caching: false}
    steps:
    - setup_remote_docker
    - run: {command: echo hi}
`},
		{name: "a removed key that is still true", want: "still true", after: `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:current, docker_layer_caching: true}
    steps:
    - setup_remote_docker
    - run: {command: echo hi}
`},
		{name: "anything else changed", want: "beyond", after: `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:edge}
    steps:
    - setup_remote_docker
    - run: {command: echo hi}
`},
		{name: "sequence order matters", want: "beyond", after: `
version: 2
jobs:
  build:
    machine: {image: ubuntu-2204:current}
    steps:
    - run: {command: echo hi}
    - setup_remote_docker
`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := patch.CheckDelta([]byte(before), []byte(tc.after), removed)
			if tc.want == "" {
				assert.Check(t, err)
				return
			}
			assert.Check(t, cmp.ErrorContains(err, tc.want))
		})
	}

	// Regression cases found in code review.
	t.Run("false counts as absent only when the group says so", func(t *testing.T) {
		err := patch.CheckDelta([]byte("flag: true\n"), []byte("flag: false\n"), plan.Delta{Removed: [][]string{{"flag"}}})
		assert.Check(t, cmp.ErrorContains(err, "still false"))
	})
	t.Run("an unrelated bare-step rewrite is not hidden", func(t *testing.T) {
		err := patch.CheckDelta([]byte("target: true\nsteps:\n  - keep: {}\n"), []byte("steps:\n  - keep\n"),
			plan.Delta{Removed: [][]string{{"target"}}})
		assert.Check(t, cmp.ErrorContains(err, "beyond"))
	})
	t.Run("a second document is an error, not ignored", func(t *testing.T) {
		err := patch.CheckDelta([]byte("target: true\n---\nsecond: old\n"), []byte("{}\n---\nsecond: new\n"),
			plan.Delta{Removed: [][]string{{"target"}}})
		assert.Check(t, cmp.ErrorContains(err, "exactly one YAML document"))
	})

	t.Run("a removed path that was never there", func(t *testing.T) {
		err := patch.CheckDelta([]byte(before), []byte(before), plan.Delta{Removed: [][]string{{"jobs", "build", "nope"}}})
		assert.Check(t, cmp.ErrorContains(err, "was not in the compiled config"))
	})
}

func TestSameCompile(t *testing.T) {
	same, err := patch.SameCompile([]byte("# Orb header\na: 1\nb: [x, y]\n"), []byte("b: [x, y]\na: 1\n"))
	assert.NilError(t, err)
	assert.Check(t, same, "comments and mapping order are not drift")

	same, err = patch.SameCompile([]byte("steps:\n  - keep: {}\n"), []byte("steps:\n  - keep\n"))
	assert.NilError(t, err)
	assert.Check(t, !same, "an empty step and a bare step are different compiled output")

	same, err = patch.SameCompile([]byte("a: 1\n"), []byte("a: 2\n"))
	assert.NilError(t, err)
	assert.Check(t, !same)
}

func TestCheckDeltaSet(t *testing.T) {
	before := "jobs:\n  a:\n    resource_class: xlarge\n    docker: [{image: x}]\n"
	want := plan.Delta{Set: []plan.SetValue{{Path: []string{"jobs", "a", "resource_class"}, Value: "large"}}}
	assert.Check(t, patch.CheckDelta([]byte(before), []byte("jobs:\n  a:\n    resource_class: large\n    docker: [{image: x}]\n"), want))
	assert.Check(t, cmp.ErrorContains(patch.CheckDelta([]byte(before),
		[]byte("jobs:\n  a:\n    resource_class: large\n    docker: [{image: y}]\n"), want), "beyond jobs.a.resource_class"))
	assert.Check(t, cmp.ErrorContains(patch.CheckDelta([]byte(before),
		[]byte("jobs:\n  a:\n    resource_class: medium\n    docker: [{image: x}]\n"), want), "beyond"))
}
