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

package finding_test

import (
	"regexp"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
)

func TestNewID(t *testing.T) {
	target := finding.Target{
		Job:  "build",
		Path: []string{"jobs", "build", "machine", "docker_layer_caching"},
	}

	t.Run("does not change when the targeted value changes", func(t *testing.T) {
		// Identity is the target, not its value.
		before := finding.Finding{Module: "dlc", Target: target, Suggestion: finding.Suggestion{Op: finding.OpRemove, Value: true}}
		after := finding.Finding{Module: "dlc", Target: target, Suggestion: finding.Suggestion{Op: finding.OpSet, Value: false}}
		assert.Check(t, cmp.Equal(finding.NewID(before.Module, before.Target), finding.NewID(after.Module, after.Target)))
	})

	t.Run("is stable across calls", func(t *testing.T) {
		assert.Check(t, cmp.Equal(finding.NewID("dlc", target), finding.NewID("dlc", target)))
		assert.Check(t, cmp.Regexp("^"+regexp.QuoteMeta("dlc:"), finding.NewID("dlc", target)))
	})

	t.Run("differs by module, job and path", func(t *testing.T) {
		base := finding.NewID("dlc", target)
		otherPath := target
		otherPath.Path = []string{"jobs", "build", "docker_layer_caching"}
		otherJob := target
		otherJob.Job = "test"
		assert.Check(t, base != finding.NewID("cache", target))
		assert.Check(t, base != finding.NewID("dlc", otherPath))
		assert.Check(t, base != finding.NewID("dlc", otherJob))
	})

	t.Run("path parts cannot be confused by joining", func(t *testing.T) {
		a := finding.Target{Path: []string{"a.b", "c"}}
		b := finding.Target{Path: []string{"a", "b.c"}}
		// Both render as "a.b.c" for display, but each part is hashed separately.
		assert.Check(t, cmp.Equal(a.PathString(), b.PathString()))
		assert.Check(t, finding.NewID("dlc", a) != finding.NewID("dlc", b))
	})
}
