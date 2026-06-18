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

package main

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// sampleDistributions mirrors the shape of packagecloud's GET /distributions.json:
// a map keyed by package type, each a list of distributions with a versions array.
// It includes the generic "fat manifest" distros — "rpm_any" (rpm) and "any"
// (deb) — that the uploader targets, alongside concrete ones.
const sampleDistributions = `{
  "deb": [
    {"index_name": "ubuntu", "versions": [{"id": 190, "index_name": "focal"}]},
    {"index_name": "any",    "versions": [{"id": 35,  "index_name": "any"}]}
  ],
  "dsc": [
    {"index_name": "ubuntu", "versions": [{"id": 220, "index_name": "focal"}]}
  ],
  "rpm": [
    {"index_name": "el",      "versions": [{"id": 140, "index_name": "9"}]},
    {"index_name": "rpm_any", "versions": [{"id": 167, "index_name": "rpm_any"}]}
  ]
}`

func parseDistributions(t *testing.T) map[string][]distro {
	t.Helper()
	var dists map[string][]distro
	assert.NilError(t, json.Unmarshal([]byte(sampleDistributions), &dists))
	return dists
}

func TestFindDistroVersion(t *testing.T) {
	dists := parseDistributions(t)

	// The generic distros the uploader actually targets.
	for ext, name := range anyDistro {
		id, ok := findDistroVersion(dists, name)
		assert.Assert(t, ok, "expected to find generic distro %q for %s", name, ext)
		assert.Check(t, id > 0)
	}

	rpmAny, _ := findDistroVersion(dists, "rpm_any")
	assert.Check(t, cmp.Equal(rpmAny, 167))
	debAny, _ := findDistroVersion(dists, "any")
	assert.Check(t, cmp.Equal(debAny, 35))

	// "deb_any" does not exist — deb uses "any". This is the bug that broke CI.
	_, ok := findDistroVersion(dists, "deb_any")
	assert.Check(t, !ok, "deb_any must not resolve; deb's generic distro is \"any\"")
}
