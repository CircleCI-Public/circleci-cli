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

package pipeline

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseParams(t *testing.T) {
	t.Run("nil on empty input", func(t *testing.T) {
		result, err := parseParams(nil)
		assert.NilError(t, err)
		assert.Check(t, is.Nil(result))
	})

	t.Run("nil on empty slice", func(t *testing.T) {
		result, err := parseParams([]string{})
		assert.NilError(t, err)
		assert.Check(t, is.Nil(result))
	})

	t.Run("string value", func(t *testing.T) {
		result, err := parseParams([]string{"env=staging"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["env"], "staging"))
	})

	t.Run("bool true", func(t *testing.T) {
		result, err := parseParams([]string{"run_e2e=true"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["run_e2e"], true))
	})

	t.Run("bool false", func(t *testing.T) {
		result, err := parseParams([]string{"skip_tests=false"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["skip_tests"], false))
	})

	t.Run("bool is case-sensitive", func(t *testing.T) {
		result, err := parseParams([]string{"flag=True"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["flag"], "True")) // not coerced to bool
	})

	t.Run("integer value", func(t *testing.T) {
		result, err := parseParams([]string{"count=42"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["count"], int64(42)))
	})

	t.Run("zero is integer not string", func(t *testing.T) {
		result, err := parseParams([]string{"retries=0"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["retries"], int64(0)))
	})

	t.Run("negative integer", func(t *testing.T) {
		result, err := parseParams([]string{"offset=-5"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["offset"], int64(-5)))
	})

	t.Run("empty value is a string", func(t *testing.T) {
		result, err := parseParams([]string{"key="})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["key"], ""))
	})

	t.Run("value containing equals sign", func(t *testing.T) {
		result, err := parseParams([]string{"expr=a=b"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["expr"], "a=b")) // only first = is the delimiter
	})

	t.Run("multiple params", func(t *testing.T) {
		result, err := parseParams([]string{"env=prod", "count=3", "debug=true"})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(result["env"], "prod"))
		assert.Check(t, is.Equal(result["count"], int64(3)))
		assert.Check(t, is.Equal(result["debug"], true))
	})

	t.Run("error on missing equals", func(t *testing.T) {
		_, err := parseParams([]string{"noequals"})
		assert.Check(t, is.ErrorContains(err, "noequals"))
	})

	t.Run("error on empty key", func(t *testing.T) {
		_, err := parseParams([]string{"=value"})
		assert.Check(t, is.ErrorContains(err, "=value"))
	})
}
