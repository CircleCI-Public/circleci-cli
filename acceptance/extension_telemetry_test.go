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
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/analytics-go/v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

func TestExtensionTelemetry(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	const extName = "testextension"
	registry.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
			Ref: &extension.Reference{
				Subcommands: []extension.ReferenceSub{
					{Name: "setup", Reference: extension.Reference{
						Subcommands: []extension.ReferenceSub{{Name: "guide"}},
					}},
					{Name: "exit"},
				},
			},
		},
		fakes.ExtensionMeta{Arch: runtime.GOARCH, Sys: runtime.GOOS},
	)
	assert.Assert(t, t.Run("install", func(t *testing.T) {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"extension", "install", extName},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Assert(t, cmp.Equal(result.ExitCode, 0))
	}))

	const schemaExt = "config-optimize"
	registry.WithExtension(t,
		extension.Manifest{
			Name:       schemaExt,
			BinaryName: "circleci-" + schemaExt,
			Version:    "1.2.3",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{Arch: runtime.GOARCH, Sys: runtime.GOOS},
	)
	assert.Assert(t, t.Run("install config-optimize", func(t *testing.T) {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"extension", "install", schemaExt},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Assert(t, cmp.Equal(result.ExitCode, 0))
	}))

	// runIn executes the extension with telemetry on, in the given test
	// environment, and returns the properties of its command_invocation event and
	// the directory used as TMPDIR for the run, a fresh one when tmp is empty.
	// extDir, when set, is put on PATH so an extension the CLI does not manage can
	// be found.
	runIn := func(
		t *testing.T, env *testenv.TestEnv, extDir, tmp, name string, args ...string,
	) (binary.CLIResult, analytics.Properties, string) {
		t.Helper()
		env.Telemetry = true
		fs := fakesegment.New(ctx, telemetry.SegmentKey)
		fsSrv := httptest.NewServer(fs)
		t.Cleanup(fsSrv.Close)
		env.Extra["CIRCLE_TELEMETRY_ENDPOINT"] = fsSrv.URL
		if tmp == "" {
			tmp = shortTempDir(t)
		}
		setExtra(t, env, "TMPDIR", tmp)

		environ := env.Environ()
		if extDir != "" {
			environ = withExtDir(environ, extDir)
		}
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    append([]string{name}, args...),
			Env:     environ,
			WorkDir: t.TempDir(),
		})

		var props analytics.Properties
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			for _, batch := range fs.Batches() {
				for _, msg := range batch.Messages {
					if msg.Event == "command_invocation" {
						props = msg.Properties
						return poll.Success()
					}
				}
			}
			return poll.Continue("no command_invocation event yet")
		})
		return result, props, tmp
	}

	// runExt runs the installed extension name with telemetry on.
	runExt := func(t *testing.T, name string, args ...string) (binary.CLIResult, analytics.Properties, string) {
		t.Helper()
		return runIn(t, env, "", "", name, args...)
	}

	run := func(t *testing.T, args ...string) (binary.CLIResult, analytics.Properties) {
		t.Helper()
		result, props, _ := runExt(t, extName, args...)
		return result, props
	}

	t.Run("records a declared subcommand path", func(t *testing.T) {
		result, props := run(t, "setup", "guide", "--json")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["command"], "circleci "+extName))
		assert.Check(t, cmp.Equal(props["extension_subcommand"], "setup guide"))
		assert.Check(t, cmp.Equal(props["app_name"], "circleci-cli"))
		assert.Check(t, cmp.Equal(props["exit_code"], float64(0)))
	})

	t.Run("records no subcommand when the first arg is a value", func(t *testing.T) {
		_, props := run(t, "ci tests", "--doctor")
		_, ok := props["extension_subcommand"]
		assert.Check(t, !ok, "an undeclared arg must not be recorded: %v", props)
	})

	t.Run("records the extension's own exit code", func(t *testing.T) {
		result, props := run(t, "exit", "3")
		assert.Check(t, cmp.Equal(result.ExitCode, 3))
		assert.Check(t, cmp.Equal(props["extension_subcommand"], "exit"))
		assert.Check(t, cmp.Equal(props["exit_code"], float64(result.ExitCode)))
		assert.Check(t, cmp.Equal(props["error_code"], "extension.exited"))
	})

	// The handoff uses a Unix socket and TMPDIR, which Windows does not read.
	unixOnly := func(t *testing.T) {
		t.Helper()
		if runtime.GOOS == "windows" {
			t.Skip("the telemetry socket tests set TMPDIR, which Windows does not read")
		}
	}

	const (
		onePayload = `{"version":1,"attributes":{"suggestions":3}}`
		okPayload  = `{"version":1,"attributes":{"suggestions":3,"jobs_analyzed":10,"jobs_with_suggestions":2}}`
	)

	t.Run("accepted", func(t *testing.T) {
		unixOnly(t)
		setExtra(t, env, "CIRCLE_DEBUG", "1")
		result, props, tmp := runExt(t, schemaExt, "telemetry", "send", okPayload)
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "accepted"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{
			"ext_configoptimizer_suggestions":           float64(3),
			"ext_configoptimizer_jobs_analyzed":         float64(10),
			"ext_configoptimizer_jobs_with_suggestions": float64(2),
		}))
		assert.Check(t, cmp.Equal(props["extension_version"], "1.2.3"))
		assert.Check(t, cmp.Equal(props["app_name"], "circleci-cli"))
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))

		path, ok := socketLine(result.Stdout)
		assert.Assert(t, ok && path != "", "the extension must see a socket path: %q", result.Stdout)
		assert.Check(t, !strings.Contains(result.Stderr, path), "stderr must not hold the socket path")
		assert.Check(t, !strings.Contains(result.Stderr, okPayload), "stderr must not hold the payload")
		assert.Check(t, !strings.Contains(result.Stderr, `"suggestions"`), "stderr must not hold any of the payload")
	})

	t.Run("exits non-zero", func(t *testing.T) {
		unixOnly(t)
		result, props, tmp := runExt(t, schemaExt, "telemetry", "send-fail", onePayload)
		assert.Check(t, cmp.Equal(result.ExitCode, 3))
		assert.Check(t, cmp.Equal(props["exit_code"], float64(result.ExitCode)))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "accepted"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{"ext_configoptimizer_suggestions": float64(3)}))
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})

	t.Run("killed", func(t *testing.T) {
		unixOnly(t)
		result, props, tmp := runExt(t, schemaExt, "telemetry", "send-kill", onePayload)
		assert.Check(t, cmp.Equal(result.ExitCode, killedExitCode))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "accepted"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{"ext_configoptimizer_suggestions": float64(3)}))
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})

	t.Run("setup failed", func(t *testing.T) {
		unixOnly(t)
		long := filepath.Join(shortTempDir(t), strings.Repeat("d", 120))
		err := os.MkdirAll(long, 0o700)
		assert.NilError(t, err)
		result, props, _ := runIn(t, env, "", long, schemaExt, "telemetry", "send", onePayload)
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "setup_failed"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{}))
		path, ok := socketLine(result.Stdout)
		assert.Check(t, ok && path == "", "the extension must see no socket: %q", result.Stdout)
		assert.Check(t, cmp.DeepEqual(dirEntries(t, long), []string{}))
	})

	t.Run("holds connection", func(t *testing.T) {
		unixOnly(t)
		start := time.Now()
		result, props, tmp := runExt(t, schemaExt, "telemetry", "hold")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "read_failed"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{}))
		assert.Check(t, time.Since(start) < 8*time.Second, "the CLI must not wait for the open connection: %s", time.Since(start))
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})

	// Each row is a payload the extension sends and what the CLI makes of it.
	manyKeys := `{"version":1,"attributes":{"suggestions":3`
	for i := range 20 {
		manyKeys += `,"extra` + strings.Repeat("x", i) + `":1`
	}
	manyKeys += `}}`
	for _, tc := range []struct {
		name   string
		args   []string
		status string
		want   map[string]any
	}{
		{
			name:   "cut short",
			args:   []string{"telemetry", "cut", onePayload},
			status: "read_failed",
		},
		{
			name:   "never connects",
			args:   []string{"telemetry", "none"},
			status: "not_supported",
		},
		{
			name:   "value out of bounds",
			args:   []string{"telemetry", "send", `{"version":1,"attributes":{"suggestions":1000000}}`},
			status: "rejected",
		},
		{
			name: "one good, one bad",
			args: []string{
				"telemetry", "send",
				`{"version":1,"attributes":{"suggestions":3,"jobs_analyzed":1000000}}`,
			},
			status: "partially_accepted",
			want:   map[string]any{"ext_configoptimizer_suggestions": float64(3)},
		},
		{
			name:   "malformed payload, cut object",
			args:   []string{"telemetry", "send", `{"version":1,"attributes":` + " garbage]]"},
			status: "read_failed",
		},
		{
			name:   "malformed payload, not JSON",
			args:   []string{"telemetry", "send", "notjson"},
			status: "read_failed",
		},
		{
			name:   "unsupported version",
			args:   []string{"telemetry", "send", `{"version":2,"attributes":{"suggestions":3}}`},
			status: "rejected",
		},
		{
			name:   "empty attributes",
			args:   []string{"telemetry", "send", `{"version":1,"attributes":{}}`},
			status: "accepted",
		},
		{
			name: "oversize payload",
			args: []string{
				"telemetry", "send",
				`{"version":1,"attributes":{"suggestions":3,"no_suggestion_reason":"` + strings.Repeat("a", 9000) + `"}}`,
			},
			status: "read_failed",
		},
		{
			name:   "too many attributes",
			args:   []string{"telemetry", "send", manyKeys},
			status: "rejected",
		},
		{
			name:   "unknown name",
			args:   []string{"telemetry", "send", `{"version":1,"attributes":{"nope":1}}`},
			status: "rejected",
		},
		{
			name:   "unknown plus valid",
			args:   []string{"telemetry", "send", `{"version":1,"attributes":{"nope":1,"suggestions":3}}`},
			status: "partially_accepted",
			want:   map[string]any{"ext_configoptimizer_suggestions": float64(3)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unixOnly(t)
			result, props, tmp := runExt(t, schemaExt, tc.args...)
			assert.Check(t, cmp.Equal(result.ExitCode, 0))
			assert.Check(t, cmp.Equal(props["extension_attributes_status"], tc.status))
			want := tc.want
			if want == nil {
				want = map[string]any{}
			}
			assert.Check(t, cmp.DeepEqual(extProps(props), want))
			assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
		})
	}

	t.Run("extension without schema", func(t *testing.T) {
		unixOnly(t)
		result, props, tmp := runExt(t, extName, "telemetry", "send", onePayload)
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "not_supported"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{}))
		path, ok := socketLine(result.Stdout)
		assert.Check(t, ok && path == "", "the extension must see no socket: %q", result.Stdout)
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})

	t.Run("unmanaged", func(t *testing.T) {
		unixOnly(t)
		// A fresh environment, where testextension is not installed, so the CLI
		// finds only the binary on PATH.
		unmanaged := testenv.New(t)
		unmanaged.Token = testToken
		result, props, tmp := runIn(t, unmanaged, testBinaryDir, "", extName, "telemetry", "send", onePayload)
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "not_supported"))
		assert.Check(t, cmp.DeepEqual(extProps(props), map[string]any{}))
		path, ok := socketLine(result.Stdout)
		assert.Check(t, ok && path == "", "the extension must see no socket: %q", result.Stdout)
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})

	t.Run("inherited variable", func(t *testing.T) {
		unixOnly(t)
		setExtra(t, env, "CIRCLE_TELEMETRY_SOCKET", "/tmp/inherited.sock")
		result, props, _ := runExt(t, extName, "telemetry", "none")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["extension_attributes_status"], "not_supported"))
		path, ok := socketLine(result.Stdout)
		assert.Check(t, ok && path == "", "an inherited socket must be removed: %q", result.Stdout)
	})

	// With telemetry off no event arrives, so this cannot use runExt.
	t.Run("telemetry off", func(t *testing.T) {
		unixOnly(t)
		telemetryWas := env.Telemetry
		t.Cleanup(func() { env.Telemetry = telemetryWas })
		env.Telemetry = false
		tmp := t.TempDir()
		setExtra(t, env, "TMPDIR", tmp)
		setExtra(t, env, "CIRCLE_TELEMETRY_SOCKET", "/tmp/inherited.sock")

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{schemaExt, "telemetry", "send", onePayload},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		path, ok := socketLine(result.Stdout)
		assert.Check(t, ok && path == "", "the extension must see no socket: %q", result.Stdout)
		assert.Check(t, cmp.DeepEqual(dirEntries(t, tmp), []string{}))
	})
}

// shortTempDir returns an empty directory with a path short enough to hold a
// Unix socket, which t.TempDir is not: it includes the test name, and a macOS
// temporary directory is already long.
func shortTempDir(t *testing.T) string {
	t.Helper()
	base := os.TempDir()
	if runtime.GOOS != "windows" {
		base = "/tmp"
	}
	dir, err := os.MkdirTemp(base, "e")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// setExtra sets an environment variable for the test and restores it after.
func setExtra(t *testing.T, env *testenv.TestEnv, key, value string) {
	t.Helper()
	old, had := env.Extra[key]
	env.Extra[key] = value
	t.Cleanup(func() {
		if had {
			env.Extra[key] = old
		} else {
			delete(env.Extra, key)
		}
	})
}

// extProps returns the properties an extension attribute set, those with the
// ext_ prefix.
func extProps(props analytics.Properties) map[string]any {
	got := map[string]any{}
	for k, v := range props {
		if strings.HasPrefix(k, "ext_") {
			got[k] = v
		}
	}
	return got
}

// socketLine returns the text after "socket " on the extension's first line of
// output, and whether that line was there.
func socketLine(stdout string) (string, bool) {
	first, _, _ := strings.Cut(stdout, "\n")
	return strings.CutPrefix(first, "socket ")
}

// dirEntries returns the names in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	assert.NilError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// killedExitCode is the exit code the CLI returns when the extension is killed:
// the process has no exit status, so *ErrExited carries -1 and the CLI exits 1.
const killedExitCode = 1
