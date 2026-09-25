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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	assert.NilError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestListPins(t *testing.T) {
	t.Run("Entries are returned in config order", func(t *testing.T) {
		pins, err := function.ListPins(write(t, `version: 2.1
functions:
  setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b
  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4
`))
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(pins, []function.Pin{
			{Alias: "setup-go", Function: "github.com/circleci-functions/setup-go", Version: "v0.5.1-684fd5b"},
			{Alias: "setup-node", Function: "github.com/circleci-functions/setup-node", Version: "v0.1.0-cc33dd4"},
		}))
	})

	t.Run("A config with no block yields nothing", func(t *testing.T) {
		pins, err := function.ListPins(write(t, "version: 2.1\n"))
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(pins, 0))
	})

	t.Run("A functions: key holding anchors is left alone", func(t *testing.T) {
		pins, err := function.ListPins(write(t, "version: 2.1\nfunctions: &defaults\n  - checkout\n"))
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(pins, 0))
	})

	t.Run("A malformed value is reported rather than hidden", func(t *testing.T) {
		pins, err := function.ListPins(write(t, "version: 2.1\nfunctions:\n  setup-go: v0.1.0-abc1234\n"))
		assert.NilError(t, err)
		assert.Assert(t, cmp.Len(pins, 1))
		assert.Check(t, cmp.Equal(pins[0].Function, "v0.1.0-abc1234"))
		assert.Check(t, cmp.Equal(pins[0].Version, ""))
	})

	t.Run("A missing config file reports the path", func(t *testing.T) {
		_, err := function.ListPins(filepath.Join(t.TempDir(), "nope.yml"))
		assert.Check(t, cmp.ErrorContains(err, "No config file at"))
	})
}
