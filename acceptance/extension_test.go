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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func TestExtensionInstall_OfficialExtensionNotFound(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testsuite"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 6))
	assert.Check(t, cmp.Len(result.Stdout, 0))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionInstall_OfficialExtensionNotFound_AcceptsFlagsAndArgs(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testsuite", "ci tests", "--doctor"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 6))
	assert.Check(t, cmp.Len(result.Stdout, 0))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionInstall_OfficialExtensionNotFound_Interactive(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testsuite"

	f.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{
			Arch: runtime.GOARCH,
			Sys:  runtime.GOOS,
		},
	)
	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testsuite"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	_, err := console.ExpectString(fmt.Sprintf("%q is not installed. Install %q now?", extName, extName))
	assert.NilError(t, err)

	_, err = console.Send("Y\r")
	assert.NilError(t, err)

	// expect the test binary to run after installation.
	_, err = console.ExpectString("args []")
	assert.NilError(t, err)
}

func TestExtensionInstall_InvalidExtensionName(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", "^&*"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionInstall_ExtensionNotFound(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", "testextension"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionInstall_ExtensionBinaryNotAvailableForPlatform(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"

	f.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{
			Arch: "fakearch",
			Sys:  "fakeos",
		},
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 5))

	data := struct {
		GOOS   string
		GOARCH string
	}{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
	goldenTemplate(t, result.Stderr, t.Name()+".stderr.tmpl", data)
}

func TestExtensionInstall_DownloadFailed(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"

	f.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{
			Version: "mismatchVersion",
			Arch:    runtime.GOARCH,
			Sys:     runtime.GOOS,
		},
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionInstall_ChecksumMismatch(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"

	f.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{
			Sha256: "not-a-valid-sha-256",
			Arch:   runtime.GOARCH,
			Sys:    runtime.GOOS,
		},
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	data := struct {
		ExpectedSha string
		ActualSha   string
	}{
		ExpectedSha: "not-a-valid-sha-256",
		ActualSha:   f.Manifest("circleci-" + extName).Sha256,
	}
	goldenTemplate(t, result.Stderr, t.Name()+".stderr.tmpl", data)
}

// TestExtensionDispatch verifies that an unknown command is transparently
// dispatched to circleci-<name> when the binary exists in PATH.
func TestExtension(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	installExtension(t, f, env, extName, runtime.GOARCH, runtime.GOOS)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{extName, "arg1", "arg2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// TestExtensionTokenFromConfig verifies that CIRCLE_TOKEN is injected from
// the CLI config file when the token is not already present in the environment.
// This is distinct from TestExtensionEnvInjection, which only exercises the
// env-passthrough path in buildEnv.
func TestExtensionTokenFromConfig(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	installExtension(t, f, env, extName, runtime.GOARCH, runtime.GOOS)

	// Write the token to the config file. Do NOT set env.Token — the token
	// must reach the extension via the CLI's config-loading path.
	configDir := filepath.Join(env.HomeDir, ".config", "circleci")
	err := os.MkdirAll(configDir, 0o755)
	assert.NilError(t, err)
	err = os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("token: "+testToken+"\n"), 0o600)
	assert.NilError(t, err)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--insecure-storage", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionExitCodePropagated verifies that the extension's exit code is
// propagated back to the caller unchanged.
func TestExtensionExitCodePropagated(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	installExtension(t, f, env, extName, runtime.GOARCH, runtime.GOOS)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{extName, "exit", "123"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 123))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionNotFoundShowsOriginalError verifies that when no matching
// extension exists, the original "unknown command" error from Cobra is shown
// and the ErrNotFound message (which names the missing binary) does not leak.
func TestExtensionNotFoundShowsOriginalError(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"no-such-command-xyz"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionNestedUnknownNotIntercepted verifies that unknown subcommands
// within a known group (e.g. "circleci pipeline foo") are not dispatched to
// any extension — only top-level unknown commands are intercepted.
func TestExtensionNestedUnknownNotIntercepted(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "foo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// The key assertion: the extension script prints "args:" — if that appears,
	// the extension was wrongly invoked. Whether the group shows help or an error
	// is pre-existing behavior outside this feature's scope.
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionRemove(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	installExtension(t, f, env, extName, runtime.GOARCH, runtime.GOOS)

	// Verify the extension directory exists before removal.
	extDir := filepath.Join(env.HomeDir, ".local", "share", "circleci", "extensions", "circleci-"+extName)
	_, statErr := os.Stat(extDir)
	assert.NilError(t, statErr)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "remove", "--force", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// Verify the extension directory was actually removed from disk.
	_, statErr = os.Stat(extDir)
	assert.Check(t, os.IsNotExist(statErr), "expected extension directory %q to be removed", extDir)
}

func TestExtensionRemove_NotInstalled(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "remove", "--force", "testextension"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionRemove_RequiresForce(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	// In non-interactive mode (no TTY), --force is required.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "remove", "testextension"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionRemove_InvalidName(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "remove", "--force", "^&*"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestUnmanagedExtension(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testextension", "arg1", "arg2"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestUnmanagedExtensionTokenFromConfig(t *testing.T) {
	env := testenv.New(t)

	// Write the token to the config file. Do NOT set env.Token — the token
	// must reach the extension via the CLI's config-loading path.
	configDir := filepath.Join(env.HomeDir, ".config", "circleci")
	err := os.MkdirAll(configDir, 0o755)
	assert.NilError(t, err)
	err = os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("token: "+testToken+"\n"), 0o600)
	assert.NilError(t, err)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--insecure-storage", "testextension"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestUnmanagedExtensionExitCodePropagated verifies that the extension's exit code is
// propagated back to the caller unchanged.
func TestUnmanagedExtensionExitCodePropagated(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testextension", "exit", "123"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 123))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestUnmanagedExtensionNotFoundShowsOriginalError verifies that when no matching
// extension exists, the original "unknown command" error from Cobra is shown
// and the ErrNotFound message (which names the missing binary) does not leak.
func TestUnmanagedExtensionNotFoundShowsOriginalError(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"no-such-command-xyz"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestUnmanagedExtensionNestedUnknownNotIntercepted verifies that unknown subcommands
// within a known group (e.g. "circleci pipeline foo") are not dispatched to
// any extension — only top-level unknown commands are intercepted.
func TestUnmanagedExtensionNestedUnknownNotIntercepted(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "testextension"},
		Env:     withExtDir(env.Environ(), binaryPath),
		WorkDir: t.TempDir(),
	})

	// The key assertion: the extension script prints "args:" — if that appears,
	// the extension was wrongly invoked. Whether the group shows help or an error
	// is pre-existing behavior outside this feature's scope.
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestExtensionDocsEnrichReference(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	f.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
			Ref: &extension.Reference{
				Use:   "<command>",
				Short: "Documented test extension",
				Long:  "A test extension with enriched command documentation.",
				Subcommands: []extension.ReferenceSub{
					{
						Name: "do",
						Reference: extension.Reference{
							Use:   "<name> [flags]",
							Short: "Do a thing",
							Long:  "Do a thing with the given name.",
							Args: []extension.ReferenceArg{
								{Name: "<name>", Help: "The name to operate on."},
							},
							Flags: []extension.ReferenceFlag{
								{Name: "parallel", Shorthand: "p", Type: "int", Default: "4", Usage: "Parallel workers"},
							},
						},
					},
				},
			},
		},
		fakes.ExtensionMeta{Arch: runtime.GOARCH, Sys: runtime.GOOS},
	)

	installResult := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Assert(t, cmp.Equal(installResult.ExitCode, 0))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"help", "reference"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 0))

	// The reference documents every command, find the extension section.
	section := findSection(t, result.Stdout, "### `circleci testextension <command>`")
	assert.Check(t, golden.String(section, t.Name()+".txt"))
}

func TestExtensionNoDocsFallsBackToBaseReference(t *testing.T) {
	f := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = f.URL()

	extName := "testextension"
	installExtension(t, f, env, extName, runtime.GOARCH, runtime.GOOS)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"help", "reference"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 0))

	// With no docs the extension renders via the base reference path, so scope
	// the golden to its section to keep it stable against unrelated command
	// changes.
	section := findSection(t, result.Stdout, "### `circleci "+extName+"`")
	assert.Check(t, golden.String(section, t.Name()+".txt"))
}

// referenceSection returns the slice of `help reference` output documenting a
// single top-level command: from its heading line up to (but not including) the
// next heading at the same or higher level. Deeper subcommand headings are kept.
func findSection(t *testing.T, doc, heading string) string {
	t.Helper()

	sectionHead := heading + "\n"
	lvl := headingLevel(sectionHead)

	sb := strings.Builder{}
	found := false

	for l := range strings.Lines(doc) {
		if l == sectionHead {
			found = true
			sb.WriteString(l)
			continue
		}

		if found {
			if headingLevel(l) == lvl {
				break
			}
			sb.WriteString(l)
		}
	}
	assert.Assert(t, len(sb.String()) > 0, "heading %q not found in reference:\n%s", heading, doc)

	return sb.String()
}

// headingLevel reports the Markdown heading level of a line (the count of
// leading '#' characters followed by a space), or 0 if the line is not a
// heading.
func headingLevel(line string) int {
	before, after, found := strings.Cut(line, " ")
	if !found || len(after) == 0 {
		return 0
	}

	if strings.Count(before, "#") == len(before) {
		return len(before)
	}

	return 0
}

// withExtDir prepends extDir to PATH in the given environ slice.
func withExtDir(environ []string, extDir string) []string {
	out := make([]string, 0, len(environ)+1)
	out = append(out, environ...)
	for i, v := range out {
		if strings.HasPrefix(v, "PATH=") {
			out[i] = "PATH=" + extDir + string(os.PathListSeparator) + v[len("PATH="):]
			return out
		}
	}
	return append(out, "PATH="+extDir)
}

func installExtension(t *testing.T, registry *fakes.ExtensionRegistry, env *testenv.TestEnv, extName, arch, sys string) {
	registry.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
		},
		fakes.ExtensionMeta{
			Arch: arch,
			Sys:  sys,
		},
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Assert(t, cmp.Equal(result.ExitCode, 0))
}
