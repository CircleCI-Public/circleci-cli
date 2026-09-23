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

package acceptance_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	setupGoName   = "github.com/circleci-functions/setup-go"
	setupGoLatest = "v0.5.1-684fd5b"
	// The package description, which a version without its own falls back to.
	setupGoDescription = "Install a Go toolchain and link it onto PATH for later steps.\n\nExample:\n  circleci run setup-go@<version> -- --version 1.22"
)

func setupFunctionFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	// Added out of alphabetical order, so the golden shows the CLI passing the
	// server's order through rather than imposing one of its own. The released
	// version is not the highest published, so the two stay distinguishable.
	fake.AddFunction(setupGoName, setupGoDescription, setupGoLatest, "v0.9.0-aaa1111")
	fake.AddFunctionVersion(setupGoName, setupGoLatest, map[string]any{
		"name":        "setup-go",
		"description": "Install a Go toolchain and link it onto PATH for later steps.",
		"version":     setupGoLatest,
		"flags": []any{
			map[string]any{
				"name": "version", "type": "string", "default": "stable",
				"description": "Go version spec: a release (1.22) | stable.",
			},
			map[string]any{
				"name": "cache", "type": "bool", "default": true,
				"description": "Use the job cache.\nTurn off for hermetic builds.",
			},
		},
	})
	fake.AddFunction("github.com/circleci-functions/setup-browser-tools",
		"Install browsers and browser-testing tools.", "v0.1.0-f29f758")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

func runFunction(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return runFunctionIn(t, env, t.TempDir(), args...)
}

func runFunctionIn(t *testing.T, env *testenv.TestEnv, dir string, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"function"}, args...),
		Env:     env.Environ(),
		WorkDir: dir,
	})
}

// writeFnConfig puts body at .circleci/config.yml under a fresh work dir.
func writeFnConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".circleci", "config.yml"), []byte(body), 0o644))
	return dir
}

func TestFunctionList(t *testing.T) {
	env := setupFunctionFake(t)

	result := runFunction(t, env, "list")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestFunctionList_JSON(t *testing.T) {
	env := setupFunctionFake(t)

	result := runFunction(t, env, "list", "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Assert(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["name"], setupGoName))
	assert.Check(t, cmp.Equal(out[0]["latest_version"], setupGoLatest))
	// --json carries the whole description; only the table is summarized.
	assert.Check(t, cmp.Contains(out[0]["description"].(string), "Example:"))
	assert.Check(t, cmp.DeepEqual(out[0]["versions"], []any{setupGoLatest, "v0.9.0-aaa1111"}))
}

func TestFunctionListAPIUnavailable(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetFunctionListStatus(http.StatusNotFound)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := runFunction(t, env, "list")

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	// An installation that does not serve functions must not read as an empty
	// catalogue, and must not be blamed on the user's network.
	assert.Check(t, cmp.Contains(result.Stderr, "Could not list published functions."))
	assert.Check(t, cmp.Contains(result.Stderr, "CIRCLE_HOST"))
}

func TestFunctionListEmpty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := runFunction(t, env, "list")

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stderr, "No published functions found."))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
}

func TestFunctionGet(t *testing.T) {
	env := setupFunctionFake(t)

	result := runFunction(t, env, "get", "setup-go")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestFunctionGet_JSON(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("A bare name defaults to the latest release", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		// The fake assigns a random id, so only its presence is stable.
		id, _ := out["id"].(string)
		assert.Check(t, cmp.Regexp(`^[0-9a-f-]{36}$`, id))
		delete(out, "id")
		assert.Check(t, cmp.DeepEqual(out, map[string]any{
			"name":           setupGoName,
			"description":    "Install a Go toolchain and link it onto PATH for later steps.",
			"version":        setupGoLatest,
			"latest_version": setupGoLatest,
			"versions":       []any{setupGoLatest, "v0.9.0-aaa1111"},
			"flags": []any{
				map[string]any{
					"name": "version", "type": "string", "default": "stable",
					"description": "Go version spec: a release (1.22) | stable.",
				},
				map[string]any{
					"name": "cache", "type": "bool", "default": "true",
					"description": "Use the job cache.\nTurn off for hermetic builds.",
				},
			},
		}))
	})

	t.Run("An explicit version follows the version reference", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--version", "v0.9.0-aaa1111", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		delete(out, "id")
		assert.Check(t, cmp.DeepEqual(out, map[string]any{
			"name":           setupGoName,
			"description":    setupGoDescription,
			"version":        "v0.9.0-aaa1111",
			"latest_version": setupGoLatest,
			"versions":       []any{setupGoLatest, "v0.9.0-aaa1111"},
			"flags":          []any{},
		}))
	})
}

func TestFunctionGetNotFound(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("An unpublished name exits not-found", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-nothing")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("The legacy prefix is rejected before the network", func(t *testing.T) {
		result := runFunction(t, env, "get", "circleci/setup-go")
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("An unpublished version lists what is published", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--version", "v9.9.9")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("A function with no versions says so", func(t *testing.T) {
		fake := fakes.NewCircleCI(t)
		fake.AddFunction(setupGoName, "Install a Go toolchain.")
		env := testenv.New(t)
		env.Token = testToken
		env.CircleCIURL = fake.URL()

		result := runFunction(t, env, "get", "setup-go")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})
}

func TestFunctionListPinned(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("The local config is read instead of the API", func(t *testing.T) {
		dir := writeFnConfig(t, "version: 2.1\n\nfunctions:\n  go: "+setupGoName+"@"+setupGoLatest+"\n")

		result := runFunctionIn(t, env, dir, "list", "--pinned", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out []map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out, []map[string]any{{
			"alias":    "go",
			"function": setupGoName,
			"version":  setupGoLatest,
		}}))
	})

	t.Run("A config with no declarations says so", func(t *testing.T) {
		dir := writeFnConfig(t, "version: 2.1\n")
		result := runFunctionIn(t, env, dir, "list", "--pinned")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Contains(result.Stderr, "No functions declared"))
	})
}

func readFnConfig(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml")) //nolint:gosec // test-controlled path
	assert.NilError(t, err)
	return string(data)
}

const baseConfig = `version: 2.1

jobs:
  build:
    steps:
      - checkout
`

func TestFunctionAdd(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, baseConfig)

	result := runFunctionIn(t, env, dir, "add", "setup-go")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("The declaration is written as a full reference", func(t *testing.T) {
		assert.Check(t, cmp.Contains(readFnConfig(t, dir),
			"functions:\n  setup-go: "+setupGoName+"@"+setupGoLatest))
	})

	t.Run("The rest of the config survives", func(t *testing.T) {
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), "- checkout"))
	})

	t.Run("Declaring it twice is refused", func(t *testing.T) {
		again := runFunctionIn(t, env, dir, "add", "setup-go")
		assert.Check(t, cmp.Equal(again.ExitCode, 2))
		assert.Check(t, cmp.Contains(again.Stderr, "already has an entry"))
	})

	t.Run("and a dry run reports the same refusal", func(t *testing.T) {
		again := runFunctionIn(t, env, dir, "add", "setup-go", "--dry-run")
		assert.Check(t, cmp.Equal(again.ExitCode, 2))
		assert.Check(t, cmp.Contains(again.Stderr, "already has an entry"))
	})
}

func TestFunctionAdd_JSON(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, baseConfig)

	result := runFunctionIn(t, env, dir, "add", "setup-go", "--json")
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"alias":    "setup-go",
		"function": setupGoName,
		"version":  setupGoLatest,
		"config":   ".circleci/config.yml",
	}))
	assert.Check(t, cmp.Contains(readFnConfig(t, dir), "setup-go: "+setupGoName+"@"+setupGoLatest))
}

func TestFunctionAddAliasConflict(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, `version: 2.1

orbs:
  setup-go: circleci/go@1.0.0
`)

	result := runFunctionIn(t, env, dir, "add", "setup-go")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(readFnConfig(t, dir), "setup-go-fn: "+setupGoName+"@"+setupGoLatest))
	assert.Check(t, cmp.Contains(result.Stderr, `Aliased as "setup-go-fn"`))
	assert.Check(t, cmp.Contains(result.Stderr, "already names an orb"))

	t.Run("The orb entry is untouched", func(t *testing.T) {
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), "setup-go: circleci/go@1.0.0"))
	})
}

func TestFunctionAddExplicitAliasConflict(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, "version: 2.1\n\norbs:\n  go: circleci/go@1.0.0\n")
	before := readFnConfig(t, dir)

	result := runFunctionIn(t, env, dir, "add", "setup-go", "--as", "go")

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Equal(readFnConfig(t, dir), before), "an explicit alias must not be renamed")
}

func TestFunctionAddOptions(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("--as chooses the alias and --version the pin", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		result := runFunctionIn(t, env, dir, "add", "setup-go", "--as", "go", "--version", "v0.9.0-aaa1111")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), "go: "+setupGoName+"@v0.9.0-aaa1111"))
	})

	t.Run("--dry-run leaves the file byte-identical", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		before := readFnConfig(t, dir)

		result := runFunctionIn(t, env, dir, "add", "setup-go", "--dry-run")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stdout, "Would add"))
		assert.Check(t, cmp.Equal(readFnConfig(t, dir), before))
	})

	t.Run("A missing config file reports the path", func(t *testing.T) {
		result := runFunction(t, env, "add", "setup-go")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, cmp.Contains(result.Stderr, "No config file at"))
	})
}
