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

	"gopkg.in/yaml.v3"
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
	// An untagged build, which the API only lists on request.
	setupGoPseudo = "v0.5.2-0.20261007152818-72ffa3f7a023"
	// A build from the old publish flow, which suffixed versions with the short
	// commit. The API only lists it on request too.
	setupGoBuild = "v0.4.0-2ad832a"
	// The package description, which a version without its own falls back to.
	setupGoDescription = "Install a Go toolchain and link it onto PATH for later steps.\n\nExample:\n  circleci run setup-go@<version> -- --version 1.22"
)

// setupGoExample is the latest release's example config, as the toolkit
// renders it at release.
const setupGoExample = `functions:
  setup-go: ` + setupGoName + "@" + setupGoLatest + `
jobs:
  build:
    steps:
      - setup-go:
          with:
            version: "1.22"
`

func setupFunctionFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	// Added out of alphabetical order, so the golden shows the CLI passing the
	// server's order through rather than imposing one of its own. The released
	// version is not the highest published, so the two stay distinguishable.
	fake.AddFunction(setupGoName, setupGoDescription, setupGoLatest, "v0.9.0")
	fake.AddFunctionVersion(setupGoName, setupGoLatest, map[string]any{
		"name":        "setup-go",
		"description": "Install a Go toolchain and link it onto PATH for later steps.",
		"version":     setupGoLatest,
		"example":     setupGoExample,
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
	fake.AddFunctionVersion(setupGoName, setupGoPseudo, map[string]any{"name": "setup-go", "version": setupGoPseudo})
	fake.AddFunctionVersion(setupGoName, setupGoBuild, map[string]any{"name": "setup-go", "version": setupGoBuild})
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
	assert.Check(t, cmp.DeepEqual(out[0]["versions"], []any{setupGoLatest, "v0.9.0"}))
}

func TestFunctionList_Pseudo(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("Builds are left out by default", func(t *testing.T) {
		result := runFunction(t, env, "list", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out []map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out[0]["versions"], []any{setupGoLatest, "v0.9.0"}))
	})

	t.Run("--include-pseudo lists them", func(t *testing.T) {
		result := runFunction(t, env, "list", "--json", "--include-pseudo")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out []map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out[0]["versions"], []any{setupGoLatest, "v0.9.0", setupGoPseudo, setupGoBuild}))
	})
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
			"versions":       []any{setupGoLatest, "v0.9.0"},
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
			"commands": []any{},
			"example":  setupGoExample,
		}))
	})

	t.Run("An explicit version follows the version reference", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--version", "v0.9.0", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		delete(out, "id")
		assert.Check(t, cmp.DeepEqual(out, map[string]any{
			"name":           setupGoName,
			"description":    setupGoDescription,
			"version":        "v0.9.0",
			"latest_version": setupGoLatest,
			"versions":       []any{setupGoLatest, "v0.9.0"},
			"flags":          []any{},
			"commands":       []any{},
		}))
	})
}

// setupCommandsFake publishes functions whose arguments live under commands,
// using descriptors exactly as the registry published them.
func setupCommandsFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	for _, fn := range []struct{ name, version string }{
		{"setup-node", "v0.3.0"},
		{"setup-browser-tools", "v0.2.0"},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", "functions", fn.name+"@"+fn.version+".yaml"))
		assert.NilError(t, err)
		var descriptor map[string]any
		assert.NilError(t, yaml.Unmarshal(raw, &descriptor))

		name := "github.com/circleci-functions/" + fn.name
		fake.AddFunction(name, "", fn.version)
		fake.AddFunctionVersion(name, fn.version, descriptor)
	}

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

func TestFunctionGet_Commands(t *testing.T) {
	env := setupCommandsFake(t)

	t.Run("A default command supplies the arguments", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-node")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	})

	t.Run("A function that only groups commands lists every one", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-browser-tools")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	})
}

func TestFunctionGet_Commands_JSON(t *testing.T) {
	env := setupCommandsFake(t)

	t.Run("The default and every command are included", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-node", "--json",
			"--jq", "{flags: [.flags[].name], default, commands: [.commands[] | {name, description, flags: [.flags[].name]}]}")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out, map[string]any{
			"flags":   []any{"node-version", "node-version-file", "mirror", "mirror-token", "cache"},
			"default": "install",
			"commands": []any{
				map[string]any{
					"name":        "install",
					"description": "Install Node.js and put node, npm and npx on PATH. Runs when no command is given.",
					"flags":       []any{"node-version", "node-version-file", "mirror", "mirror-token", "cache"},
				},
				map[string]any{
					"name":        "install-packages",
					"description": "Install the project's dependencies with its package manager, caching what it downloads.",
					"flags":       []any{"app-dir", "package-manager", "override-command", "cache"},
				},
			},
		}))
	})

	t.Run("A command's flags carry their details", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-node", "--json",
			"--jq", `.commands[] | select(.name == "install-packages") | .flags[0]`)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out, map[string]any{
			"name": "app-dir", "type": "string", "default": ".",
			"description": "Directory containing package.json.",
		}))
	})

	t.Run("A function with no default has no default key", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-browser-tools", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out["flags"], []any{}))
		_, hasDefault := out["default"]
		assert.Check(t, !hasDefault, "default: %v", out["default"])
		commands, _ := out["commands"].([]any)
		assert.Check(t, cmp.Len(commands, 8))
	})
}

func TestFunctionGet_Pseudo(t *testing.T) {
	env := setupFunctionFake(t)

	t.Run("Builds are left out of the versions", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.DeepEqual(out["versions"], []any{setupGoLatest, "v0.9.0"}))
	})

	t.Run("A pseudo-version named by --version is found", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--version", setupGoPseudo, "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.Equal(out["version"], setupGoPseudo))
	})

	t.Run("A commit-suffixed build named by --version is found", func(t *testing.T) {
		result := runFunction(t, env, "get", "setup-go", "--version", setupGoBuild, "--json")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		assert.Check(t, cmp.Equal(out["version"], setupGoBuild))
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
		"step":     "- setup-go:\n    with:\n      version: \"1.22\"\n",
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
		result := runFunctionIn(t, env, dir, "add", "setup-go", "--as", "go", "--version", "v0.9.0")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), "go: "+setupGoName+"@v0.9.0"))
		// That version publishes no example, so only the step name is offered.
		assert.Check(t, cmp.Contains(result.Stdout, `Invoke it as a step named "go".`))
	})

	t.Run("A pseudo-version can be pinned by name", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		result := runFunctionIn(t, env, dir, "add", "setup-go", "--version", setupGoPseudo)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), setupGoName+"@"+setupGoPseudo))
	})

	t.Run("A commit-suffixed build can be pinned by name", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		result := runFunctionIn(t, env, dir, "add", "setup-go", "--version", setupGoBuild)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(readFnConfig(t, dir), setupGoName+"@"+setupGoBuild))
	})

	t.Run("The example step is invoked by the alias", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		result := runFunctionIn(t, env, dir, "add", "setup-go", "--as", "go")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stdout, "    - go:\n        with:\n"))
	})

	t.Run("--dry-run leaves the file byte-identical", func(t *testing.T) {
		dir := writeFnConfig(t, baseConfig)
		before := readFnConfig(t, dir)

		result := runFunctionIn(t, env, dir, "add", "setup-go", "--dry-run")
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stdout, "Would add"))
		assert.Check(t, cmp.Contains(result.Stdout, "Invoke it from a job's steps:\n\n  - setup-go:\n"))
		assert.Check(t, cmp.Equal(readFnConfig(t, dir), before))
	})

	t.Run("A missing config file reports the path", func(t *testing.T) {
		result := runFunction(t, env, "add", "setup-go")
		assert.Check(t, cmp.Equal(result.ExitCode, 5))
		assert.Check(t, cmp.Contains(result.Stderr, "No config file at"))
	})
}

func TestFunctionUpdate_JSON(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, "version: 2.1\n\nfunctions:\n  setup-go: "+setupGoName+"@v0.9.0\n")

	result := runFunctionIn(t, env, dir, "update", "setup-go", "--json")
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"alias":            "setup-go",
		"function":         setupGoName,
		"previous_version": "v0.9.0",
		"version":          setupGoLatest,
		"config":           ".circleci/config.yml",
	}))
	assert.Check(t, cmp.Contains(readFnConfig(t, dir), "setup-go: "+setupGoName+"@"+setupGoLatest))
}

func TestFunctionUpdateMalformed(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, "version: 2.1\n\nfunctions:\n  setup-go: v0.1.0\n")

	result := runFunctionIn(t, env, dir, "update", "setup-go")

	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestFunctionUpdate(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, `version: 2.1

functions:
  # pinned deliberately
  setup-go: `+setupGoName+`@v0.9.0
`)

	result := runFunctionIn(t, env, dir, "update", "setup-go")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("The reference moves and the comment survives", func(t *testing.T) {
		got := readFnConfig(t, dir)
		assert.Check(t, cmp.Contains(got, "setup-go: "+setupGoName+"@"+setupGoLatest))
		assert.Check(t, cmp.Contains(got, "# pinned deliberately"))
	})

	t.Run("Re-running it is a no-op", func(t *testing.T) {
		again := runFunctionIn(t, env, dir, "update", "setup-go")
		assert.Check(t, cmp.Equal(again.ExitCode, 0))
		assert.Check(t, cmp.Contains(again.Stdout, "already pinned"))
	})
}

func TestFunctionUpdateNotDeclared(t *testing.T) {
	env := setupFunctionFake(t)
	dir := writeFnConfig(t, baseConfig)

	result := runFunctionIn(t, env, dir, "update", "setup-go")

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, cmp.Contains(result.Stderr, "no entry in the functions block"))
}
