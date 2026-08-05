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
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// TestPack_ParseErrorIsClickable is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/519: a YAML problem in
// one file of a packed tree has to say which file, at which line, in a form the
// terminal turns into a link. yaml.v3's own shape buries the filename in prose
// and pushes the line onto a continuation line.
func TestPack_ParseErrorIsClickable(t *testing.T) {
	tests := []struct {
		name    string
		content string
		// line and message are composed with the file path at assert time, using
		// the platform separator — a hardcoded "executors/executorA.yml" passes
		// on Unix and fails on Windows.
		line    int
		message string
	}{
		{
			// The MWE from the issue: a key repeated after a bad merge.
			name: "duplicate mapping key",
			content: "docker:\n  - image: not-relevant\n    auth:\n" +
				"      username: $DOCKERHUB_USER\n      password: $DOCKERHUB_ACCESSTOKEN\n    auth:\n",
			line:    6,
			message: `mapping key "auth" already defined at line 3`,
		},
		{
			name:    "syntax error carries its line",
			content: "docker:\n  - image: x\nsteps:\n\t- run: echo hi\n",
			line:    4,
			message: "found character that cannot start any token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
			target := filepath.Join(dir, "executors", "executorA.yml")
			writeFile(t, target, tt.content)

			_, err := pack.Pack(dir)
			assert.ErrorContains(t, err, fmt.Sprintf("%s:%d: %s", target, tt.line, tt.message))

			// The location has to lead the message — a filename mentioned
			// halfway through prose is not clickable.
			assert.Equal(t, strings.HasPrefix(err.Error(), target+":"), true,
				"error should start with the file path, got: %s", err)
		})
	}
}

// TestPack_ParseErrorWithoutLine covers the messages yaml.v3 emits with no line
// number at all. Those still get a location, just a file-level one, rather than
// losing the filename entirely.
func TestPack_ParseErrorWithoutLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	target := filepath.Join(dir, "commands", "bad.yml")
	writeFile(t, target, "a: b: c\n")

	_, err := pack.Pack(dir)
	assert.ErrorContains(t, err, target+": mapping values are not allowed in this context")
	// No bare "yaml: " prefix left over once the location leads.
	assert.Equal(t, strings.Contains(err.Error(), "yaml: "), false,
		"the yaml.v3 prefix should be replaced by the location, got: %s", err)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}
