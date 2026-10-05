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

package registry_test

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/data"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
)

type fake string

func (f fake) Name() string               { return string(f) }
func (fake) Requires() []data.Requirement { return nil }
func (fake) Evaluate(context.Context, *pipelineconfig.Effective, data.View) ([]finding.Finding, error) {
	return nil, nil
}

func TestSelect(t *testing.T) {
	catalog := []module.Analyzer{fake("dlc"), fake("cache"), fake("storage")}

	t.Run("empty selects everything in order", func(t *testing.T) {
		got, err := registry.Select(catalog, nil)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(registry.Names(got), []string{"dlc", "cache", "storage"}))
	})
	t.Run("filters without reordering", func(t *testing.T) {
		got, err := registry.Select(catalog, []string{"storage", "dlc"})
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(registry.Names(got), []string{"dlc", "storage"}))
	})
	t.Run("unknown name is an error", func(t *testing.T) {
		_, err := registry.Select(catalog, []string{"dlc", "nope"})
		assert.Check(t, cmp.ErrorContains(err, "unknown module nope"))
	})
}

func TestCatalogReturnsAFreshSlice(t *testing.T) {
	deps := registry.Deps{Pricing: pricing.None{}}
	a := registry.Catalog(deps)
	b := registry.Catalog(deps)
	a = append(a, fake("mutated"))
	assert.Check(t, cmp.Len(b, len(a)-1), "mutating one catalog must not affect another")
}
