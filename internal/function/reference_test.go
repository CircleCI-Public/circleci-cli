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

package function_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

func TestValidatePath(t *testing.T) {
	for _, path := range []string{
		"github.com/circleci-functions/setup-go",
		"github.com/myorg/a/b/c",
	} {
		t.Run("valid: "+path, func(t *testing.T) {
			assert.Check(t, function.ValidatePath(path))
		})
	}

	for what, path := range map[string]string{
		"no host":        "circleci-functions/setup-go",
		"no dot in host": "github/circleci-functions/setup-go",
		"one segment":    "github.com/setup-go",
		"trailing slash": "github.com/org/name/",
		"unanchored":     "github.com/org/name\nx",
	} {
		t.Run("invalid: "+what, func(t *testing.T) {
			assert.Check(t, function.ValidatePath(path) != nil, "expected %q to be rejected", path)
		})
	}
}

func TestValidateVersion(t *testing.T) {
	for _, v := range []string{"v0.5.1-684fd5b", "v1.0.0", "v10.20.30-rc.1"} {
		t.Run("valid: "+v, func(t *testing.T) {
			assert.Check(t, function.ValidateVersion(v))
		})
	}

	for what, v := range map[string]string{
		"no leading v": "0.5.1",
		"not semver":   "v1.0",
		"volatile":     "volatile",
		"unanchored":   "v1.0.0\n",
	} {
		t.Run("invalid: "+what, func(t *testing.T) {
			assert.Check(t, function.ValidateVersion(v) != nil, "expected %q to be rejected", v)
		})
	}
}

func TestExpandName(t *testing.T) {
	t.Run("A bare name expands against the default org", func(t *testing.T) {
		got, err := function.ExpandName("setup-go")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "github.com/circleci-functions/setup-go"))
	})

	t.Run("A full path is used verbatim", func(t *testing.T) {
		got, err := function.ExpandName("github.com/myorg/my-fn")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "github.com/myorg/my-fn"))
	})

	t.Run("A partial path is rejected rather than expanded", func(t *testing.T) {
		// Expanding would silently produce a different function than asked for.
		_, err := function.ExpandName("myorg/my-fn")
		assert.Check(t, cmp.ErrorContains(err, "not valid"))
	})
}
