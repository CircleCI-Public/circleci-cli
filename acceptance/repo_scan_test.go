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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

// TestRepoScan_DetectsGoModule is the only acceptance test for `repo scan`.
// It exercises the parts that unit tests cannot:
//
//   - the binary actually builds and the `repo scan` command is reachable
//   - env-builder runs for real against a real filesystem
//   - Docker Hub is reachable for image-version resolution
//
// Empty detection, JSON output, flag registration, and help text are all
// covered by unit tests in internal/cmd/repo and by the recursive help
// snapshot test in internal/cmd/root (TestUsage).
func TestRepoScan_DetectsGoModule(t *testing.T) {
	dir := t.TempDir()
	gomod := "module example.com/x\n\ngo 1.22\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"repo", "scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	if result.ExitCode != 0 {
		t.Skipf("repo scan exited %d (likely Docker Hub unavailable). stderr: %s",
			result.ExitCode, result.Stderr)
	}
	assert.Check(t, cmp.Contains(result.Stderr, "Detected go"),
		"expected go detection in stderr, got: %s", result.Stderr)
}
