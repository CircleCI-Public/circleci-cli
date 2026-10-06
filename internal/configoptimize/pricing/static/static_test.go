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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing/static"
)

var _ pricing.Provider = (*static.Table)(nil)

// pricingYAML is FIXTURE DATA, NOT TRUTH: real rates depend on plan and
// contract. It is here only so the lookups have a table to read.
const pricingYAML = `
flat_charges:
  dlc: 200
credits_per_minute:
  - {platform: docker, class: small, generation: gen1, credits: 5}
  - {platform: docker, class: medium, generation: gen1, credits: 10}
  - {platform: docker, class: large, generation: gen1, credits: 20}
ladders:
  - {platform: docker, generation: gen1, classes: [small, medium, large]}
`

// TestParse covers the lookups on a parsed table; the built-in table is
// loaded by every report in acceptance/config_optimize_test.go.
func TestParse(t *testing.T) {
	table, err := static.Parse([]byte(pricingYAML))
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
		_, err := static.Parse([]byte("flat_charges:\n  dlc: 200\ncredit_per_minute: []\n"))
		assert.Check(t, cmp.ErrorContains(err, "credit_per_minute"))
	})
}
