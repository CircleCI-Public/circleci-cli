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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

func TestCompletionInstallZsh(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/zsh"

	zshrc := filepath.Join(env.HomeDir, ".zshrc")
	err := os.WriteFile(zshrc, []byte("# zshrc\n"), 0o644)
	assert.NilError(t, err)

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	data, err := os.ReadFile(zshrc)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "# circleci shell completion"),
		".zshrc should contain tag, got: %s", string(data))
	assert.Check(t, cmp.Contains(string(data), "circleci completion zsh"),
		".zshrc should contain source line, got: %s", string(data))
}

func TestCompletionInstallBash(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/bash"

	bashrc := filepath.Join(env.HomeDir, ".bashrc")
	err := os.WriteFile(bashrc, []byte("# bashrc\n"), 0o644)
	assert.NilError(t, err)

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	data, err := os.ReadFile(bashrc)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "# circleci shell completion"),
		".bashrc should contain tag, got: %s", string(data))
	assert.Check(t, cmp.Contains(string(data), "circleci completion bash"),
		".bashrc should contain source line, got: %s", string(data))
}

func TestCompletionInstallBashProfile(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/bash"

	// macOS: .bash_profile exists, .bashrc does not
	bashProfile := filepath.Join(env.HomeDir, ".bash_profile")
	err := os.WriteFile(bashProfile, []byte("# bash_profile\n"), 0o644)
	assert.NilError(t, err)

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	data, err := os.ReadFile(bashProfile)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "circleci completion bash"),
		".bash_profile should contain source line, got: %s", string(data))
}

func TestCompletionInstallBashCreatesRCFile(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/bash"
	// Neither .bashrc nor .bash_profile exists — should create .bash_profile

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	bashProfile := filepath.Join(env.HomeDir, ".bash_profile")
	info, err := os.Stat(bashProfile)
	assert.NilError(t, err, ".bash_profile should have been created")
	perm := info.Mode().Perm()
	assert.Check(t, cmp.Equal(perm, os.FileMode(0o644)))
	data, err := os.ReadFile(bashProfile)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "circleci completion bash"),
		"created file should contain source line, got: %s", string(data))
}

func TestCompletionInstallIdempotent(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/zsh"

	zshrc := filepath.Join(env.HomeDir, ".zshrc")
	err := os.WriteFile(zshrc, []byte("# zshrc\n"), 0o644)
	assert.NilError(t, err)

	// First install
	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())
	assert.Equal(t, result.ExitCode, 0, "first install failed")
	dataAfterFirst, err := os.ReadFile(zshrc)
	assert.NilError(t, err)

	// Second install should be a no-op
	result = binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())
	assert.Equal(t, result.ExitCode, 0, "second install failed")
	assert.Check(t, cmp.Contains(result.Stderr, "already installed"),
		"expected 'already installed' message, got stderr: %s", result.Stderr)
	dataAfterSecond, err := os.ReadFile(zshrc)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(dataAfterFirst), string(dataAfterSecond)),
		"rc file should be unchanged on second install")
}

func TestCompletionInstallUnsupportedShell(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/fish"

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Check(t, result.ExitCode != 0, "expected non-zero exit for unsupported shell")
	assert.Check(t, cmp.Contains(result.Stderr, "unsupported shell"),
		"expected unsupported shell error, got: %s", result.Stderr)
}

func TestCompletionInstallEmptyShell(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = ""

	result := binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())

	assert.Check(t, result.ExitCode != 0, "expected non-zero exit for empty SHELL")
}

func TestCompletionUninstallZsh(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/zsh"

	zshrc := filepath.Join(env.HomeDir, ".zshrc")
	original := "# existing config\nexport FOO=bar\n"
	err := os.WriteFile(zshrc, []byte(original), 0o644)
	assert.NilError(t, err)

	// Install then uninstall
	binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())
	result := binary.RunCLI(t, []string{"completion", "uninstall"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	data, err := os.ReadFile(zshrc)
	assert.NilError(t, err)
	assert.Check(t, !strings.Contains(string(data), "# circleci shell completion"),
		"tag should be removed, got: %s", string(data))
	assert.Check(t, !strings.Contains(string(data), "circleci completion zsh"),
		"source line should be removed, got: %s", string(data))
}

func TestCompletionUninstallPreservesOtherContent(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/bash"

	bashrc := filepath.Join(env.HomeDir, ".bashrc")
	original := "# my config\nexport PATH=/usr/local/bin:$PATH\nalias ll='ls -la'\n"
	err := os.WriteFile(bashrc, []byte(original), 0o644)
	assert.NilError(t, err)

	binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())
	result := binary.RunCLI(t, []string{"completion", "uninstall"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	data, err := os.ReadFile(bashrc)
	assert.NilError(t, err)
	content := string(data)
	assert.Check(t, cmp.Contains(content, "export PATH=/usr/local/bin:$PATH"),
		"existing content should be preserved, got: %s", content)
	assert.Check(t, cmp.Contains(content, "alias ll='ls -la'"),
		"existing content should be preserved, got: %s", content)
}

func TestCompletionUninstallNoBlockPresent(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/zsh"

	zshrc := filepath.Join(env.HomeDir, ".zshrc")
	err := os.WriteFile(zshrc, []byte("export BAR=baz\n"), 0o644)
	assert.NilError(t, err)

	result := binary.RunCLI(t, []string{"completion", "uninstall"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "uninstall with no block should succeed")
	data, err := os.ReadFile(zshrc)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "export BAR=baz"),
		"content should be unchanged, got: %s", string(data))
}

func TestCompletionInstallUninstallRoundTrip(t *testing.T) {
	env := testenv.New(t)
	env.Extra["SHELL"] = "/bin/zsh"

	zshrc := filepath.Join(env.HomeDir, ".zshrc")
	original := "# existing config\nexport FOO=bar\n"
	err := os.WriteFile(zshrc, []byte(original), 0o644)
	assert.NilError(t, err)

	binary.RunCLI(t, []string{"completion", "install"}, env.Environ(), t.TempDir())
	binary.RunCLI(t, []string{"completion", "uninstall"}, env.Environ(), t.TempDir())

	data, err := os.ReadFile(zshrc)
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "export FOO=bar"),
		"original content should survive round-trip, got: %s", string(data))
	assert.Check(t, !strings.Contains(string(data), "circleci completion"),
		"completion lines should be gone, got: %s", string(data))
}

func TestCompletionBashGeneratesScript(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, []string{"completion", "bash"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, len(result.Stdout) > 0, "expected bash completion output")
	assert.Check(t, strings.Contains(result.Stdout, "complete") || strings.Contains(result.Stdout, "bash"),
		"expected bash completion markers, got: %s", result.Stdout[:min(200, len(result.Stdout))])
}

func TestCompletionZshGeneratesScript(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, []string{"completion", "zsh"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, len(result.Stdout) > 0, "expected zsh completion output")
	assert.Check(t, strings.Contains(result.Stdout, "compdef") || strings.Contains(result.Stdout, "#compdef"),
		"expected zsh completion markers, got: %s", result.Stdout[:min(200, len(result.Stdout))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
