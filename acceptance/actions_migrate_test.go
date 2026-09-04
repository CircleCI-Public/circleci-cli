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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func TestActionsMigrateCopiesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, ".github", "workflows", "run_tests.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(src), 0o750))
	assert.NilError(t, os.WriteFile(src, []byte("name: run tests\n"), 0o600))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"actions", "migrate", src},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	dest := filepath.Join(dir, ".circleci", "workflows", "cci-run_tests.yml")
	content, err := os.ReadFile(dest)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(content), "name: run tests\n"))
}

func TestActionsMigrateCreatesDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "build.yml")
	assert.NilError(t, os.WriteFile(src, []byte("name: build\n"), 0o600))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"actions", "migrate", src},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	_, err := os.Stat(filepath.Join(dir, ".circleci", "workflows", "cci-build.yml"))
	assert.Check(t, err)
}

func TestActionsMigrateFailsIfDestExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "run_tests.yml")
	assert.NilError(t, os.WriteFile(src, []byte("name: run tests\n"), 0o600))

	destDir := filepath.Join(dir, ".circleci", "workflows")
	assert.NilError(t, os.MkdirAll(destDir, 0o750))
	assert.NilError(t, os.WriteFile(filepath.Join(destDir, "cci-run_tests.yml"), []byte("existing\n"), 0o600))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"actions", "migrate", src},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestActionsMigrateForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "run_tests.yml")
	assert.NilError(t, os.WriteFile(src, []byte("name: new\n"), 0o600))

	destDir := filepath.Join(dir, ".circleci", "workflows")
	assert.NilError(t, os.MkdirAll(destDir, 0o750))
	assert.NilError(t, os.WriteFile(filepath.Join(destDir, "cci-run_tests.yml"), []byte("old\n"), 0o600))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"actions", "migrate", "--force", src},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	content, err := os.ReadFile(filepath.Join(destDir, "cci-run_tests.yml"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(string(content), "name: new\n"))
}

func TestActionsMigrateMissingSource(t *testing.T) {
	dir := t.TempDir()

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"actions", "migrate", "nonexistent.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
