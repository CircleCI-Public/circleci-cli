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

// TestPack_YAML11Booleans is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/691: a boolean orb
// parameter defaulting to `on` packed to the string "on", so `orb validate` on
// the packed output rejected the parameter against its own declared type.
func TestPack_YAML11Booleans(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "vpn.yml"),
		"parameters:\n"+
			"  killswitch:\n    type: boolean\n    default: on\n"+
			"  verbose:\n    type: boolean\n    default: yes\n"+
			"  quiet:\n    type: boolean\n    default: off\n"+
			"  dryrun:\n    type: boolean\n    default: no\n"+
			"steps:\n  - run: echo hi\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `commands:
    vpn:
        parameters:
            dryrun:
                default: false
                type: boolean
            killswitch:
                default: true
                type: boolean
            quiet:
                default: false
                type: boolean
            verbose:
                default: true
                type: boolean
        steps:
            - run: echo hi
version: 2.1
`
	assert.Equal(t, packed, want)
}

// TestPack_YAML11Booleans_QuotedStaysString checks the other half of the
// contract: a source that quotes the value asked for a string, and packing must
// not decide otherwise.
func TestPack_YAML11Booleans_QuotedStaysString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "c.yml"),
		"parameters:\n"+
			"  double:\n    type: string\n    default: \"on\"\n"+
			"  single:\n    type: string\n    default: 'no'\n"+
			// y/n are strings to the config compiler, so they stay strings here.
			"  short:\n    type: string\n    default: n\n"+
			"steps:\n  - run: echo hi\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	assert.Check(t, strings.Contains(packed, `default: "on"`), "got:\n%s", packed)
	assert.Check(t, strings.Contains(packed, `default: "no"`), "got:\n%s", packed)
	// `n` stays a string. yaml.v3 quotes it on the way out because YAML 1.1
	// would read a bare n as a boolean — which is the point: the quoting is
	// what stops it being reinterpreted, and it is what the CLI already emitted
	// before this change, so no committed packed file churns.
	assert.Check(t, strings.Contains(packed, `default: "n"`), "got:\n%s", packed)
}

// TestPack_YAML11Booleans_KeysAreNotRetagged guards the mapping-key exclusion.
// An `on:` key retagged to a boolean would stop the document decoding into
// map[string]any at all.
func TestPack_YAML11Booleans_KeysAreNotRetagged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@config.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "jobs", "build.yml"), "environment:\n  on: value\n  off: other\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	assert.Check(t, strings.Contains(packed, `"on": value`), "got:\n%s", packed)
	assert.Check(t, strings.Contains(packed, `"off": other`), "got:\n%s", packed)
}

// TestPack_DuplicateKeysStillDetected guards a side effect of parsing through a
// yaml.Node: yaml.Unmarshal into a Node accepts duplicate mapping keys, and only
// the subsequent decode into a Go value reports them. Losing that would turn a
// merge conflict into silently dropped config.
func TestPack_DuplicateKeysStillDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "executors", "e.yml"),
		"docker:\n  - image: x\n    auth:\n      username: $U\n    auth:\n")

	_, err := pack.Pack(dir)
	assert.ErrorContains(t, err, `mapping key "auth" already defined at line 3`)
}

// TestPack_NestedDirectoriesFlattenIntoSection is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/755: an orb author
// wants to organise commands/ and jobs/ into subdirectories. Those files used to
// pack into commands.<dir>.<file>, producing a "command" that was really a map
// of commands, so the orb was invalid.
func TestPack_NestedDirectoriesFlattenIntoSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "top.yml"), "steps:\n  - run: echo top\n")
	writeFile(t, filepath.Join(dir, "commands", "aws", "login.yml"), "steps:\n  - run: echo login\n")
	writeFile(t, filepath.Join(dir, "commands", "gcp", "auth.yml"), "steps:\n  - run: echo auth\n")
	writeFile(t, filepath.Join(dir, "jobs", "build", "unit.yml"), "steps:\n  - run: echo unit\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `commands:
    auth:
        steps:
            - run: echo auth
    login:
        steps:
            - run: echo login
    top:
        steps:
            - run: echo top
jobs:
    unit:
        steps:
            - run: echo unit
version: 2.1
`
	assert.Equal(t, packed, want)
}

// TestPack_DeeplyNestedDirectoriesFlatten checks that flattening is not limited
// to one level below a section.
func TestPack_DeeplyNestedDirectoriesFlatten(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "a", "b", "c", "deep.yml"), "steps:\n  - run: echo deep\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `commands:
    deep:
        steps:
            - run: echo deep
version: 2.1
`
	assert.Equal(t, packed, want)
}

// TestPack_NestedNameCollision covers the cost of flattening: two files in
// different subdirectories of one section would silently claim the same key, and
// one would win. Say so instead, naming both files. Checked in both directory
// orders, because os.ReadDir returns entries sorted and a plain merge would only
// catch one of them.
func TestPack_NestedNameCollision(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			// "login.yml" sorts before the "zzz" directory.
			name: "file before directory",
			files: map[string]string{
				filepath.Join("commands", "login.yml"):        "steps:\n  - run: echo a\n",
				filepath.Join("commands", "zzz", "login.yml"): "steps:\n  - run: echo b\n",
			},
		},
		{
			// The "aaa" directory sorts before "login.yml".
			name: "directory before file",
			files: map[string]string{
				filepath.Join("commands", "aaa", "login.yml"): "steps:\n  - run: echo b\n",
				filepath.Join("commands", "login.yml"):        "steps:\n  - run: echo a\n",
			},
		},
		{
			name: "two sibling directories",
			files: map[string]string{
				filepath.Join("commands", "aws", "login.yml"): "steps:\n  - run: echo a\n",
				filepath.Join("commands", "gcp", "login.yml"): "steps:\n  - run: echo b\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
			for path, content := range tt.files {
				writeFile(t, filepath.Join(dir, path), content)
			}

			_, err := pack.Pack(dir)
			assert.ErrorContains(t, err, `both define "login"`)
			assert.ErrorContains(t, err, "cannot share a name")
		})
	}
}

// TestPack_TopLevelDirectoriesStillBecomeSections guards the boundary: only
// directories *below* a section flatten. A directory at the pack root still
// opens a section, which is the whole existing layout convention.
func TestPack_TopLevelDirectoriesStillBecomeSections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "c.yml"), "steps:\n  - run: echo c\n")
	writeFile(t, filepath.Join(dir, "executors", "e.yml"), "docker:\n  - image: alpine\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	assert.Check(t, strings.Contains(packed, "commands:\n    c:"), "got:\n%s", packed)
	assert.Check(t, strings.Contains(packed, "executors:\n    e:"), "got:\n%s", packed)
}

// TestPack_CrossFileAnchors is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/341, using the layout
// and expected output from the report: an anchor defined in anchors/@anchors.yml
// aliased from commands/my-command.yml.
func TestPack_CrossFileAnchors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "anchors", "@anchors.yml"), "my-anchor: &my-anchor my-value\n")
	writeFile(t, filepath.Join(dir, "commands", "my-command.yml"), "description: *my-anchor\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `anchors:
    my-anchor: my-value
commands:
    my-command:
        description: my-value
`
	assert.Equal(t, packed, want)
}

// TestPack_CrossFileAnchors_Collections checks that an anchor on a map or a
// sequence survives the round trip, not just a scalar. Those are the ones worth
// sharing — a common docker executor, a filter block.
func TestPack_CrossFileAnchors_Collections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "anchors", "@anchors.yml"),
		"base: &base\n  docker:\n    - image: cimg/base:2024.01\n"+
			"tags: &tags\n  - main\n  - release\n")
	writeFile(t, filepath.Join(dir, "jobs", "build.yml"),
		"executor: *base\nbranches: *tags\nsteps:\n  - checkout\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	const want = `anchors:
    base:
        docker:
            - image: cimg/base:2024.01
    tags:
        - main
        - release
jobs:
    build:
        branches:
            - main
            - release
        executor:
            docker:
                - image: cimg/base:2024.01
        steps:
            - checkout
version: 2.1
`
	assert.Equal(t, packed, want)
}

// TestPack_SameFileAnchorsStillWork guards the ordinary case: anchors used within
// one file must not be disturbed by the cross-file machinery.
func TestPack_SameFileAnchorsStillWork(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "commands", "c.yml"),
		"parameters:\n  a: &shared\n    type: string\n    default: x\n  b: *shared\nsteps:\n  - run: echo hi\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	assert.Check(t, strings.Count(packed, "default: x") == 2, "got:\n%s", packed)
}

// TestPack_UnknownAnchorStillReported checks that an alias with no definition
// anywhere in the tree is still an error, and still names the anchor. Resolving
// across files must not turn a typo into silence.
func TestPack_UnknownAnchorStillReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "anchors", "@anchors.yml"), "my-anchor: &my-anchor my-value\n")
	target := filepath.Join(dir, "commands", "c.yml")
	writeFile(t, target, "description: *my-anchr\n")

	_, err := pack.Pack(dir)
	// The location leads, in the file:line: form parseError produces (issue 519),
	// with no line to report for an unresolved alias. Built from filepath.Join so
	// the separator is right on every platform.
	assert.ErrorContains(t, err, target+": unknown anchor 'my-anchr' referenced")
}

// TestPack_CrossFileAnchors_RetagsYAML11Booleans guards the seam between two
// features that landed separately: cross-file anchors (issue 341) and reading
// unquoted on/off as booleans (issue 691).
//
// A file that aliases a sibling's anchor cannot parse alone, so it is parsed a
// second time with the definitions prepended. That retry has to go through the
// same decode path as every other file — otherwise these files, and only these
// files, would silently miss the YAML 1.1 boolean retagging and pack `default: on`
// back to the string "on".
func TestPack_CrossFileAnchors_RetagsYAML11Booleans(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@orb.yml"), "version: 2.1\n")
	writeFile(t, filepath.Join(dir, "anchors", "@anchors.yml"), "flag: &flag on\n")
	writeFile(t, filepath.Join(dir, "commands", "vpn.yml"),
		"parameters:\n  killswitch:\n    type: boolean\n    default: *flag\nsteps:\n  - run: echo hi\n")

	packed, err := pack.Pack(dir)
	assert.NilError(t, err)

	// true, not "on" — in the anchor definition and at the alias site alike.
	assert.Check(t, strings.Contains(packed, "flag: true"), "got:\n%s", packed)
	assert.Check(t, strings.Contains(packed, "default: true"), "got:\n%s", packed)
	assert.Check(t, !strings.Contains(packed, `"on"`), "nothing should still be the string \"on\", got:\n%s", packed)
}

// TestPack_AnchorRetryReportsTheRealError checks the fallback: when a file has an
// unresolved alias *and* another problem, the reported error describes the real
// file. The retry parses a synthesised document whose line numbers do not line up
// with the user's file, so its error is never the one shown.
func TestPack_AnchorRetryReportsTheRealError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "anchors", "@anchors.yml"), "a: &shared value\n")
	writeFile(t, filepath.Join(dir, "commands", "c.yml"),
		"description: *shared\ndupe: 1\ndupe: 2\n")

	_, err := pack.Pack(dir)
	// The duplicate key is the surviving problem once the alias resolves, and its
	// line numbers describe c.yml, not the synthesised document.
	assert.ErrorContains(t, err, `mapping key "dupe" already defined at line 2`)
}

// TestPack_SingleFileAnchorsAreSelfContained checks that packing one file does
// not reach out to a sibling for anchors — there is no tree to draw from, and the
// error should say so plainly.
func TestPack_SingleFileAnchorsAreSelfContained(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "@anchors.yml"), "a: &shared value\n")
	target := filepath.Join(dir, "one.yml")
	writeFile(t, target, "description: *shared\n")

	_, err := pack.Pack(target)
	assert.ErrorContains(t, err, "unknown anchor 'shared' referenced")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}
