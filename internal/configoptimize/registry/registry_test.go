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
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
)

type fake string

func (f fake) Name() string { return string(f) }
func (fake) Evaluate(context.Context, *pipelineconfig.Effective) ([]finding.Finding, error) {
	return nil, nil
}

// names are the modules' names in order.
func names(ms []module.Analyzer) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name())
	}
	return out
}

// TestParseChecks and TestSelect are the two halves of --only: check names
// to module names, then module names to the catalog's modules, in order.
func TestSelect(t *testing.T) {
	catalog := []module.Analyzer{fake("dlc"), fake("cache"), fake("storage")}

	t.Run("empty selects everything in order", func(t *testing.T) {
		got := registry.Select(catalog, nil)
		assert.Check(t, cmp.DeepEqual(names(got), []string{"dlc", "cache", "storage"}))
	})
	t.Run("filters without reordering", func(t *testing.T) {
		got := registry.Select(catalog, []string{"storage", "dlc"})
		assert.Check(t, cmp.DeepEqual(names(got), []string{"dlc", "storage"}))
	})
}

func TestParseChecks(t *testing.T) {
	tests := []struct {
		name string
		only []string
		want []string
	}{
		{name: "none means every module", only: nil, want: nil},
		{name: "check names map to modules", only: []string{"resource-class"}, want: []string{"resourceclass"}},
		{name: "spaces around a name are ignored", only: []string{" resource-class "}, want: []string{"resourceclass"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := registry.ParseChecks(tc.only)
			assert.Check(t, err)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
	t.Run("an unknown name lists the known ones", func(t *testing.T) {
		_, err := registry.ParseChecks([]string{"nope"})
		unknown, ok := errors.AsType[*registry.UnknownCheckError](err)
		assert.Assert(t, ok, "want an UnknownCheckError, got %v", err)
		assert.Check(t, cmp.DeepEqual(unknown.Known, []string{"resource-class"}))
		assert.Check(t, cmp.ErrorContains(err, `"nope" is not a check`))
	})
}
