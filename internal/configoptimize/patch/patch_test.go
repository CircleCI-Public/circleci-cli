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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/patch"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
)

var (
	machineDLC = []string{"jobs", "build", "machine", "docker_layer_caching"}
	stepDLC    = func(i string) []string {
		return []string{"jobs", "build", "steps", i, "setup_remote_docker", "docker_layer_caching"}
	}
)

// TestMain turns off gotest's CRLF-to-LF normalization: the kernel's line
// endings are part of what the goldens prove.
func TestMain(m *testing.M) {
	golden.NormalizeCRLFToLF = false
	os.Exit(m.Run())
}

func del(path []string, expect any) plan.Edit {
	return plan.Edit{Op: plan.Delete, Path: path, Expect: expect}
}

// TestCandidate is the byte-level contract of the kernel: each accepted case
// is a generated golden of the exact output bytes (`task test --
// ./internal/configoptimize/patch/... -update`); each refused case names why.
func TestCandidate(t *testing.T) {
	tests := []struct {
		fixture string
		edits   []plan.Edit
		refused string // substring of the refusal; empty means accepted
	}{
		{fixture: "last-key", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "first-key", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "comment-below", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "quoted-key", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "no-final-newline", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "crlf", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "mixed-crlf", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "bom", edits: []plan.Edit{del(machineDLC, true)}},
		{fixture: "two-edits", edits: []plan.Edit{del(machineDLC, true), del(stepDLC("0"), true)}},
		{fixture: "sole-child-step", edits: []plan.Edit{del(stepDLC("0"), true)}},
		{fixture: "sole-child-step-crlf", edits: []plan.Edit{del(stepDLC("0"), true)}},

		{fixture: "sole-child-machine", edits: []plan.Edit{del(machineDLC, true)}, refused: "empty mapping"},
		{fixture: "comment-above", edits: []plan.Edit{del(machineDLC, true)}, refused: "comment above"},
		{fixture: "inline-comment", edits: []plan.Edit{del(machineDLC, true)}, refused: "inline comment"},
		{fixture: "flow", edits: []plan.Edit{del(machineDLC, true)}, refused: "flow style"},
		{fixture: "anchor", edits: []plan.Edit{del(machineDLC, true)}, refused: "anchor"},
		{fixture: "tag", edits: []plan.Edit{del(machineDLC, true)}, refused: "explicit tag"},
		{fixture: "split-lines", edits: []plan.Edit{del(machineDLC, true)}, refused: "different lines"},
		{fixture: "dash-prefix", edits: []plan.Edit{del([]string{"jobs", "build", "steps", "0", "docker_layer_caching"}, true)}, refused: "precedes the key"},
		{fixture: "multi-doc", edits: []plan.Edit{del(machineDLC, true)}, refused: "YAML documents"},
		{fixture: "expect-mismatch", edits: []plan.Edit{del(machineDLC, true)}, refused: "not the true that was analysed"},
		{fixture: "last-key", edits: []plan.Edit{del(machineDLC, true), del(machineDLC, true)}, refused: "overlap"},
		{fixture: "last-key", edits: []plan.Edit{{Op: plan.Insert, Path: machineDLC, Value: false}}, refused: "plain identifier value"},
		{fixture: "last-key", edits: []plan.Edit{{Op: plan.Replace, Path: machineDLC, Expect: true, Value: false}}, refused: "only a string value"},
		{fixture: "last-key", edits: []plan.Edit{del([]string{"jobs", "build", "nope"}, true)}, refused: "not in the config"},
	}
	// Regression cases found in code review, given inline: each produced
	// wrong bytes without refusing.
	inline := []struct {
		name, src string
		edits     []plan.Edit
		refused   string
	}{
		{"folded continuation", "keep: alpha\nremove: beta\n  continuation\n",
			[]plan.Edit{del([]string{"remove"}, "beta continuation")}, "changed more of the config"},
		{"lone carriage return", "remove: true\rkeep: safe\nlast: ok\n",
			[]plan.Edit{del([]string{"remove"}, true)}, "carriage return"},
		{"a non-step sole child", "items:\n  - widget:\n      doomed: true\n",
			[]plan.Edit{del([]string{"items", "0", "widget", "doomed"}, true)}, "empty mapping"},
		{"deleting every child in one group", "jobs:\n  j:\n    steps:\n      - s:\n          a: true\n          b: true\n",
			[]plan.Edit{del([]string{"jobs", "j", "steps", "0", "s", "a"}, true), del([]string{"jobs", "j", "steps", "0", "s", "b"}, true)},
			"changed more of the config"},
		{"a comment after the entry", "jobs:\n  j:\n    machine:\n      image: x\n      docker_layer_caching: true\n\n      # about the flag above\n\n    steps: []\n",
			[]plan.Edit{del([]string{"jobs", "j", "machine", "docker_layer_caching"}, true)}, "comment"},
	}
	for _, tc := range inline {
		t.Run(tc.name+"/refused", func(t *testing.T) {
			_, err := patch.Candidate([]byte(tc.src), plan.Group{Edits: tc.edits})
			_, isRefusal := errors.AsType[*patch.RefusedError](err)
			assert.Check(t, isRefusal, "want a refusal, got %v", err)
			assert.Check(t, cmp.ErrorContains(err, tc.refused))
		})
	}
	for _, tc := range tests {
		name := tc.fixture
		if tc.refused != "" {
			name += "/refused " + tc.refused
		}
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", tc.fixture+".yml")) //#nosec:G304 // test fixture
			assert.NilError(t, err)

			out, err := patch.Candidate(src, plan.Group{Edits: tc.edits})
			if tc.refused != "" {
				_, isRefusal := errors.AsType[*patch.RefusedError](err)
				assert.Check(t, isRefusal, "want a refusal, got %v", err)
				assert.Check(t, cmp.ErrorContains(err, tc.refused))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, golden.String(string(out), filepath.Join("golden", tc.fixture+".out.yml")))
			assert.Check(t, onlyDeletedLines(src, out, tc.fixture), "a deletion must not rewrite other lines")
		})
	}
}

// onlyDeletedLines checks every output line appears in the input, in order
// (a removal changes only the lines it needs). A sole-child
// case may also shorten one step line from `- name:` to `- name`.
func onlyDeletedLines(src, out []byte, fixture string) bool {
	allowRewrite := strings.Contains(fixture, "sole-child")
	in := lines(src)
	i := 0
	for _, line := range lines(out) {
		for i < len(in) && !bytes.Equal(in[i], line) {
			if allowRewrite && bytes.HasPrefix(in[i], bytes.TrimRight(line, "\r\n")) {
				break
			}
			i++
		}
		if i == len(in) {
			return false
		}
		i++
	}
	return true
}

func lines(b []byte) [][]byte {
	parts := bytes.SplitAfter(b, []byte("\n"))
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func TestNoEditsIsIdentity(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "crlf.yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	out, err := patch.Candidate(src, plan.Group{})
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(out), string(src)), "an empty group must give identical bytes")
}

// Replace: only the scalar's own bytes change.
func TestReplace(t *testing.T) {
	tests := []struct {
		name, src string
		path      []string
		expect    any
		value     string
		want      string // the edited source, or "" when refused
		refused   string
	}{
		{
			name:   "a quoted value in a flow mapping at a workflow call site",
			src:    "workflows:\n  w:\n    jobs:\n      - build: {name: a, cache_key: \"{{ epoch }}\"}  # keep\n",
			path:   []string{"workflows", "w", "jobs", "0", "build", "cache_key"},
			expect: "{{ epoch }}", value: `{{ checksum "go.sum" }}`,
			want: "workflows:\n  w:\n    jobs:\n      - build: {name: a, cache_key: '{{ checksum \"go.sum\" }}'}  # keep\n",
		},
		{
			name:   "a plain block value keeps its comment",
			src:    "jobs:\n  build:\n    resource_class: xlarge # sized for tests\n    steps: [checkout]\n",
			path:   []string{"jobs", "build", "resource_class"},
			expect: "xlarge", value: "large",
			want: "jobs:\n  build:\n    resource_class: large # sized for tests\n    steps: [checkout]\n",
		},
		{
			name:   "a single-quoted sequence item",
			src:    "keys:\n  - 'npm-{{ epoch }}'\n",
			path:   []string{"keys", "0"},
			expect: "npm-{{ epoch }}", value: "npm-{{ .Branch }}",
			want: "keys:\n  - 'npm-{{ .Branch }}'\n",
		},
		{
			name: "an anchored value", src: "a: &x xlarge\nb: *x\n", path: []string{"a"},
			expect: "xlarge", value: "large", refused: "anchor",
		},
		{
			name: "a multi-line value", src: "a: |\n  xlarge\n", path: []string{"a"},
			expect: "xlarge\n", value: "large", refused: "more than one line",
		},
		{
			name: "a value that changed", src: "a: medium\n", path: []string{"a"},
			expect: "xlarge", value: "large", refused: "not the xlarge",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := patch.Candidate([]byte(tc.src), plan.Group{Edits: []plan.Edit{
				{Op: plan.Replace, Path: tc.path, Expect: tc.expect, Value: tc.value},
			}})
			if tc.refused != "" {
				assert.Check(t, cmp.ErrorContains(err, tc.refused))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(string(out), tc.want))
		})
	}
}

// Insert: one new line below the mapping's key line; every other line
// is kept byte for byte.
func TestInsert(t *testing.T) {
	const anchor = "x: &big\n  resource_class: 2xlarge\n"
	rc := func(job string) []string { return []string{"jobs", job, "resource_class"} }
	tests := []struct {
		name, src string
		edits     []plan.Edit
		want      string // the edited source, or "" when refused
		refused   string
	}{
		{
			name:  "a job that inherits from a merge key",
			src:   anchor + "jobs:\n  test:\n    <<: *big\n    steps: [checkout]\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("test"), Value: "xlarge"}},
			want:  anchor + "jobs:\n  test:\n    resource_class: xlarge\n    <<: *big\n    steps: [checkout]\n",
		},
		{
			name:  "a comment above the first entry stays with it",
			src:   "jobs:\n  lint:  # the linters\n    # shared with build\n    executor: default\n    steps: [checkout]\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("lint"), Value: "xlarge"}},
			want:  "jobs:\n  lint:  # the linters\n    resource_class: xlarge\n    # shared with build\n    executor: default\n    steps: [checkout]\n",
		},
		{
			name:  "CRLF line endings are kept",
			src:   "jobs:\r\n  lint:\r\n    executor: default\r\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("lint"), Value: "xlarge"}},
			want:  "jobs:\r\n  lint:\r\n    resource_class: xlarge\r\n    executor: default\r\n",
		},
		{
			name:  "a class name that starts with a digit is written plain",
			src:   "jobs:\n  build:\n      executor: big\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("build"), Value: "2xlarge"}},
			want:  "jobs:\n  build:\n      resource_class: 2xlarge\n      executor: big\n",
		},
		{
			name: "the key already exists", src: "jobs:\n  j:\n    resource_class: large\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "medium"}}, refused: "already has resource_class",
		},
		{
			name: "a flow mapping", src: "jobs:\n  j: {executor: default}\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "large"}}, refused: "flow style",
		},
		{
			name: "an anchored mapping", src: "jobs:\n  j: &j\n    executor: default\n  k: *j\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "large"}}, refused: "anchor",
		},
		{
			name: "an alias", src: "x: &j\n  executor: default\njobs:\n  j: *j\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "large"}}, refused: "alias",
		},
		{
			name: "a scalar job", src: "jobs:\n  j: nothing\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "large"}}, refused: "non-empty mapping",
		},
		{
			name: "a value YAML reads as another type", src: "jobs:\n  j:\n    executor: default\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "true"}}, refused: "plain identifier value",
		},
		{
			name: "a value that needs quotes", src: "jobs:\n  j:\n    executor: default\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("j"), Value: "x large"}}, refused: "plain identifier value",
		},
		{
			name: "two insertions at one place", src: "jobs:\n  j:\n    executor: default\n",
			edits: []plan.Edit{
				{Op: plan.Insert, Path: rc("j"), Value: "large"},
				{Op: plan.Insert, Path: []string{"jobs", "j", "shell"}, Value: "bash"},
			},
			refused: "overlap",
		},
		{
			name: "a mapping that is not there", src: "jobs:\n  j:\n    executor: default\n",
			edits: []plan.Edit{{Op: plan.Insert, Path: rc("k"), Value: "large"}}, refused: "not in the config",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := patch.Candidate([]byte(tc.src), plan.Group{Edits: tc.edits})
			if tc.refused != "" {
				_, isRefusal := errors.AsType[*patch.RefusedError](err)
				assert.Check(t, isRefusal, "want a refusal, got %v", err)
				assert.Check(t, cmp.ErrorContains(err, tc.refused))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(string(out), tc.want))
		})
	}
}
