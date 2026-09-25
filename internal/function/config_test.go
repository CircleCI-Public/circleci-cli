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
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	assert.NilError(t, err)
	return string(data)
}

var setupGoRef = function.Reference{
	Path:    "github.com/circleci-functions/setup-go",
	Version: "v0.5.1-684fd5b",
}

func TestAddPin(t *testing.T) {
	t.Run("A config with no functions: block gains one", func(t *testing.T) {
		path := write(t, "version: 2.1\n\njobs:\n  build:\n    steps:\n      - checkout\n")
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))
		assert.Check(t, cmp.Contains(read(t, path),
			"functions:\n  setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b"))
	})

	t.Run("An existing block gains an entry and keeps its comments", func(t *testing.T) {
		path := write(t, `version: 2.1

# functions we depend on
functions:
  # pinned deliberately
  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4
`)
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))

		got := read(t, path)
		assert.Check(t, cmp.Contains(got, "# functions we depend on"))
		assert.Check(t, cmp.Contains(got, "# pinned deliberately"))
		assert.Check(t, cmp.Contains(got, "setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4"))
		assert.Check(t, cmp.Contains(got, "setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b"))
	})

	t.Run("Declaring the same alias twice is refused and writes nothing", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions:\n  setup-go: github.com/circleci-functions/setup-go@v0.1.0\n")
		err := function.AddPin(path, "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorContains(err, "already has an entry"))
		assert.Check(t, cmp.Contains(read(t, path), "setup-go@v0.1.0"))
	})

	t.Run("Every byte the entry does not occupy is left alone", func(t *testing.T) {
		before := "version: 2.1\n\n# our orbs\norbs:\n  node: circleci/node@5.0.0\n\nfunctions:\n  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4\n\njobs:\n  build:\n    docker:\n      - image: \"cimg/base:2024.01\"\n    steps: [checkout]\n"
		path := write(t, before)
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))

		want := strings.Replace(before,
			"  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4\n",
			"  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4\n  setup-go: "+setupGoRef.String()+"\n",
			1)
		assert.Check(t, cmp.Equal(read(t, path), want))
	})

	t.Run("The block's own indentation is matched", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions:\n    setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4\njobs: {}\n")
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))
		assert.Check(t, cmp.Contains(read(t, path), "\n    setup-go: "+setupGoRef.String()+"\n"))
	})

	t.Run("CRLF line endings are not mixed", func(t *testing.T) {
		path := write(t, "version: 2.1\r\n\r\njobs: {}\r\n")
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))
		got := read(t, path)
		assert.Check(t, cmp.Contains(got, "\r\nfunctions:\r\n  setup-go: "+setupGoRef.String()+"\r\n"))
		assert.Check(t, !strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n"), "stray LF among CRLF endings")
	})

	t.Run("A functions: key that is not a map of declarations is refused", func(t *testing.T) {
		// The block name can hold YAML anchors, which ListPins tolerates. Adding
		// a second functions: key would make the file unparseable.
		before := "version: 2.1\nfunctions: &defs\n  - one\njobs: {}\n"
		path := write(t, before)
		err := function.AddPin(path, "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorContains(err, "not a plain map"))
		assert.Check(t, cmp.Equal(read(t, path), before))
	})

	t.Run("A bare functions: key gains its first entry", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions: # none yet\njobs: {}\n")
		assert.NilError(t, function.AddPin(path, "setup-go", setupGoRef))
		assert.Check(t, cmp.Equal(read(t, path),
			"version: 2.1\nfunctions: # none yet\n  setup-go: "+setupGoRef.String()+"\njobs: {}\n"))
	})

	t.Run("An explicit null is a value, and is refused", func(t *testing.T) {
		before := "version: 2.1\nfunctions: null\n"
		path := write(t, before)
		err := function.AddPin(path, "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorContains(err, "not a plain map"))
		assert.Check(t, cmp.Equal(read(t, path), before))
	})
}

func TestConflicts(t *testing.T) {
	path := write(t, `version: 2.1
orbs:
  setup-go: circleci/go@1.0.0
commands:
  build-it:
    steps:
      - checkout
functions:
  setup-node: github.com/circleci-functions/setup-node@v0.1.0
`)

	for alias, want := range map[string]string{
		"setup-go":   "orb",
		"build-it":   "command",
		"setup-node": "", // an existing declaration is a re-pin, not a clash
		"fresh":      "",
	} {
		t.Run(alias, func(t *testing.T) {
			got, err := function.Conflicts(path, alias)
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(got, want))
		})
	}
}

func TestValidateAlias(t *testing.T) {
	assert.Check(t, function.ValidateAlias("setup-go"))
	// A slash would be read as naming one of the function's commands.
	assert.Check(t, function.ValidateAlias("setup-go/cache") != nil)
	assert.Check(t, function.ValidateAlias("1st") != nil)
}

func TestUpdatePin(t *testing.T) {
	t.Run("The version is replaced in place, keeping order and comments", func(t *testing.T) {
		path := write(t, `version: 2.1
functions:
  setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4
  # pinned deliberately
  setup-go: github.com/circleci-functions/setup-go@v0.1.0
`)
		assert.NilError(t, function.UpdatePin(path, "setup-go", setupGoRef))

		got := read(t, path)
		assert.Check(t, cmp.Contains(got, "setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b"))
		assert.Check(t, cmp.Contains(got, "# pinned deliberately"))
		assert.Check(t, cmp.Contains(got, "setup-node: github.com/circleci-functions/setup-node@v0.1.0-cc33dd4"))
		assert.Check(t, strings.Index(got, "setup-node") < strings.Index(got, "setup-go:"))
	})

	t.Run("An undeclared alias yields ErrNotPinned and adds nothing", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions:\n  setup-node: github.com/circleci-functions/setup-node@v0.1.0\n")
		err := function.UpdatePin(path, "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorIs(err, function.ErrNotPinned))
		assert.Check(t, !strings.Contains(read(t, path), "setup-go"))
	})

	t.Run("A config with no block at all is reported the same way", func(t *testing.T) {
		err := function.UpdatePin(write(t, "version: 2.1\n"), "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorIs(err, function.ErrNotPinned))
	})

	t.Run("A comment after a tab survives", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions:\n  setup-go: github.com/circleci-functions/setup-go@v0.1.0\t# keep\n")
		assert.NilError(t, function.UpdatePin(path, "setup-go", setupGoRef))
		assert.Check(t, cmp.Contains(read(t, path), "setup-go: "+setupGoRef.String()+"\t# keep\n"))
	})

	t.Run("A bare functions: key has nothing to update", func(t *testing.T) {
		err := function.UpdatePin(write(t, "version: 2.1\nfunctions:\n"), "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorIs(err, function.ErrNotPinned))
	})

	t.Run("An anchor on the value survives, so its aliases follow the update", func(t *testing.T) {
		path := write(t, `version: 2.1
functions:
  setup-go: &go github.com/circleci-functions/setup-go@v0.1.0 # keep
  other: *go
`)
		assert.NilError(t, function.UpdatePin(path, "setup-go", setupGoRef))

		got := read(t, path)
		assert.Check(t, cmp.Contains(got, "setup-go: &go github.com/circleci-functions/setup-go@v0.5.1-684fd5b # keep\n"))

		var cfg struct {
			Functions map[string]string `yaml:"functions"`
		}
		assert.NilError(t, yaml.Unmarshal([]byte(got), &cfg), "the rewritten config must still parse")
		assert.Check(t, cmp.Equal(cfg.Functions["other"], setupGoRef.String()))
	})

	t.Run("Only the reference changes, byte for byte", func(t *testing.T) {
		before := "version: 2.1\n\n# ours\nfunctions:\n    setup-go: github.com/circleci-functions/setup-go@v0.1.0  # do not bump\n\njobs: {}\n"
		path := write(t, before)
		assert.NilError(t, function.UpdatePin(path, "setup-go", setupGoRef))

		want := strings.Replace(before, "setup-go@v0.1.0", "setup-go@v0.5.1-684fd5b", 1)
		assert.Check(t, cmp.Equal(read(t, path), want))
	})

	t.Run("A quoted old value is replaced whole", func(t *testing.T) {
		path := write(t, "version: 2.1\nfunctions:\n  setup-go: \"github.com/circleci-functions/setup-go@v0.1.0\"  # why\njobs: {}\n")
		assert.NilError(t, function.UpdatePin(path, "setup-go", setupGoRef))
		assert.Check(t, cmp.Contains(read(t, path),
			"  setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b  # why\n"))
	})

	t.Run("A functions: key that is not a map of declarations is refused", func(t *testing.T) {
		before := "version: 2.1\nfunctions: &defs\n  - one\njobs: {}\n"
		path := write(t, before)
		err := function.UpdatePin(path, "setup-go", setupGoRef)
		assert.Check(t, cmp.ErrorContains(err, "not a plain map"))
		assert.Check(t, cmp.Equal(read(t, path), before))
	})
}

func TestFindPin(t *testing.T) {
	path := write(t, "version: 2.1\nfunctions:\n  setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b\n")

	pin, err := function.FindPin(path, "setup-go")
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(pin.Version, "v0.5.1-684fd5b"))

	_, err = function.FindPin(path, "setup-node")
	assert.Check(t, cmp.ErrorIs(err, function.ErrNotPinned))
}
