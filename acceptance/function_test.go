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
	fake.AddFunction("github.com/circleci-functions/setup-browser-tools",
		"Install browsers and browser-testing tools.", "v0.1.0-f29f758")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

func runFunction(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"function"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
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
			"description":    setupGoDescription,
			"version":        setupGoLatest,
			"latest_version": setupGoLatest,
			"versions":       []any{setupGoLatest, "v0.9.0-aaa1111"},
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
