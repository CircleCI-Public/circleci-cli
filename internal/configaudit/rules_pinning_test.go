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

package configaudit

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// row is a finding reduced to what a rule test asserts on.
type row struct {
	ID   string
	Line int
	Col  int
	Sev  Severity
}

// auditRows audits src and returns the reported findings of rule id, in order.
func auditRows(t *testing.T, src, id string) []row {
	t.Helper()
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	report := Audit(doc, Options{})
	got := []row{}
	for _, f := range report.Findings {
		if f.RuleID == id {
			got = append(got, row{ID: f.RuleID, Line: f.Line, Col: f.Column, Sev: f.Severity})
		}
	}
	return got
}

func TestUnpinnedOrb(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "major alias",
			yaml: "orbs:\n  node: circleci/node@5\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 9, Sev: SeverityMedium}},
		},
		{
			name: "minor alias",
			yaml: "orbs:\n  node: circleci/node@5.1\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 9, Sev: SeverityMedium}},
		},
		{
			name: "volatile",
			yaml: "orbs:\n  node: circleci/node@volatile\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 9, Sev: SeverityMedium}},
		},
		{
			name: "no version",
			yaml: "orbs:\n  node: circleci/node\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 9, Sev: SeverityMedium}},
		},
		{
			name: "dev version",
			yaml: "orbs:\n  node: circleci/node@dev:alpha\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 9, Sev: SeverityMedium}},
		},
		{
			name: "quoted",
			yaml: "orbs:\n  node: \"circleci/node@5\"\n",
			want: []row{{ID: "unpinned-orb", Line: 2, Col: 10, Sev: SeverityMedium}},
		},
		{
			name: "exact version",
			yaml: "orbs:\n  node: circleci/node@1.2.3\n",
			want: []row{},
		},
		{
			name: "inline orb definition",
			yaml: "orbs:\n  mine:\n    commands:\n      hello:\n        steps: []\n",
			want: []row{},
		},
		{
			name: "no orbs section",
			yaml: "jobs: {}\n",
			want: []row{},
		},
		{
			name: "two orbs, one pinned",
			yaml: "orbs:\n  a: x/a@1.0.0\n  b: x/b@1\n",
			want: []row{{ID: "unpinned-orb", Line: 3, Col: 6, Sev: SeverityMedium}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, "unpinned-orb")
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestUnpinnedOrbMessages(t *testing.T) {
	tests := []struct {
		orb  string
		want string
	}{
		{
			orb:  "circleci/node@5",
			want: "orb circleci/node@5 is a major-version alias",
		},
		{
			orb:  "circleci/node",
			want: "orb circleci/node has no version",
		},
		{
			orb:  "circleci/node@volatile",
			want: "orb circleci/node@volatile uses @volatile",
		},
		{
			orb:  "circleci/node@dev:x",
			want: "orb circleci/node@dev:x is a dev version",
		},
	}
	for _, tc := range tests {
		t.Run(tc.orb, func(t *testing.T) {
			doc, err := Parse([]byte("orbs:\n  o: "+tc.orb+"\n"), "c.yml")
			assert.NilError(t, err)
			report := Audit(doc, Options{})
			assert.Assert(t, cmp.Len(report.Findings, 1))
			assert.Check(t, cmp.Equal(report.Findings[0].Message, tc.want))
		})
	}
}

func TestUnpinnedImage(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "latest tag is high",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/base:latest\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 16, Sev: SeverityHigh}},
		},
		{
			name: "no tag is high",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/base\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 16, Sev: SeverityHigh}},
		},
		{
			name: "version tag is medium",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/node:20.11\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "registry port is not a tag",
			yaml: "jobs:\n  b:\n    docker:\n      - image: localhost:5000/app\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 16, Sev: SeverityHigh}},
		},
		{
			name: "digest",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/node:20.11@sha256:abc\n",
			want: []row{},
		},
		{
			name: "digest without a tag",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/node@sha256:abc\n",
			want: []row{},
		},
		{
			name: "machine image is skipped",
			yaml: "jobs:\n  b:\n    machine:\n      image: ubuntu-2204:current\n",
			want: []row{},
		},
		{
			name: "service image in a job",
			yaml: "jobs:\n  b:\n    docker:\n      - image: cimg/base@sha256:abc\n      - image: redis:7\n",
			want: []row{{ID: "unpinned-image", Line: 5, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "executor",
			yaml: "executors:\n  e:\n    docker:\n      - image: cimg/base:latest\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 16, Sev: SeverityHigh}},
		},
		{
			name: "quoted image",
			yaml: "jobs:\n  b:\n    docker:\n      - image: 'cimg/base:latest'\n",
			want: []row{{ID: "unpinned-image", Line: 4, Col: 17, Sev: SeverityHigh}},
		},
		{
			name: "parameter with an unpinned default",
			yaml: "jobs:\n  b:\n    parameters:\n      img:\n        type: string\n        default: cimg/base:latest\n" +
				"    docker:\n      - image: << parameters.img >>\n",
			want: []row{{ID: "unpinned-image", Line: 8, Col: 16, Sev: SeverityHigh}},
		},
		{
			name: "parameter with a pinned default",
			yaml: "jobs:\n  b:\n    parameters:\n      img:\n        type: string\n        default: cimg/base:1@sha256:abc\n" +
				"    docker:\n      - image: << parameters.img >>\n",
			want: []row{},
		},
		{
			name: "parameter without a default",
			yaml: "jobs:\n  b:\n    parameters:\n      img:\n        type: string\n" +
				"    docker:\n      - image: << parameters.img >>\n",
			want: []row{},
		},
		{
			name: "no jobs",
			yaml: "version: 2.1\n",
			want: []row{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, "unpinned-image")
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestUnpinnedImageParameterConfidence(t *testing.T) {
	src := "jobs:\n  b:\n    parameters:\n      img:\n        type: string\n        default: cimg/base:latest\n" +
		"    docker:\n      - image: << parameters.img >>\n      - image: redis:7\n"
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	report := Audit(doc, Options{})
	assert.Assert(t, cmp.Len(report.Findings, 2))
	assert.Check(t, cmp.Equal(report.Findings[0].Confidence, ConfidenceLow))
	assert.Check(t, cmp.Equal(report.Findings[1].Confidence, ConfidenceHigh))
}
