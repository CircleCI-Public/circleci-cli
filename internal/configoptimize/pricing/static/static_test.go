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

package static_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing/static"
)

var _ pricing.Provider = (*static.Table)(nil)

func load(t *testing.T, name string) (*static.Table, error) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	return static.Parse(data)
}

func TestParse(t *testing.T) {
	table, err := load(t, "pricing.yml")
	assert.NilError(t, err)

	t.Run("flat charge", func(t *testing.T) {
		got, ok := table.FlatCharge("dlc")
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(got, 200.0))
		_, ok = table.FlatCharge("nope")
		assert.Check(t, !ok)
	})
	t.Run("credits per minute", func(t *testing.T) {
		got, ok := table.CreditsPerMinute("docker", "medium", "gen1")
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(got, 10.0))
		_, ok = table.CreditsPerMinute("docker", "medium", "gen2")
		assert.Check(t, !ok, "a rate must not leak across generations")
	})
	t.Run("ladder", func(t *testing.T) {
		got, ok := table.LadderBelow("docker", "large", "gen1")
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(got, "medium"))
		_, ok = table.LadderBelow("docker", "small", "gen1")
		assert.Check(t, !ok, "nothing below the smallest class")
		_, ok = table.LadderBelow("docker", "large", "gen2")
		assert.Check(t, !ok)
	})
	t.Run("unknown key is an error", func(t *testing.T) {
		_, err := load(t, "typo.yml")
		assert.Check(t, cmp.ErrorContains(err, "credit_per_minute"))
	})
}

func TestLoadBuiltIn(t *testing.T) {
	d, sha, err := static.Load("")
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(sha, 64))
	c, ok := d.CreditsPerMinute("docker", "xlarge", "gen1")
	assert.Check(t, ok)
	assert.Check(t, cmp.Equal(c, 40.0))
	below, ok := d.LadderBelow("docker", "2xlarge", "gen1")
	assert.Check(t, ok)
	assert.Check(t, cmp.Equal(below, "xlarge"))
}
