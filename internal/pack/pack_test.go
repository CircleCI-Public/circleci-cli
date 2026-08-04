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

package pack_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/pack"
)

// TestPack_IndentationMatchesLegacyCLI is the regression test for the second half
// of https://github.com/CircleCI-Public/circleci-cli/issues/1636: packed output
// is routinely committed, so its indentation is part of the contract. Four spaces
// with indented block sequences is what the 0.1.x CLI emitted; dropping to two
// rewrote every line of every committed packed config.
func TestPack_IndentationMatchesLegacyCLI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@config.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "jobs", "build.yml"),
		"docker:\n  - image: cimg/base:2024.01\nsteps:\n  - checkout\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `jobs:
    build:
        docker:
            - image: cimg/base:2024.01
        steps:
            - checkout
version: 2.1
`
	assert.Equal(t, packed, want)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}
