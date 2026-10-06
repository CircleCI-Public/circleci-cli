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

package cache

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// env -S is split the way GNU env does; input it would reject gives no
// words, so no command is claimed from it.
func TestSplitString(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    []string
		refused bool // env refuses it
	}{
		{name: "npm ci; --version", text: "npm ci; --version", want: []string{"npm", "ci;", "--version"}},
		{name: `npm\_ci`, text: `npm\_ci`, want: []string{"npm", "ci"}},
		{name: `a\tb`, text: `a\tb`, want: []string{"a\tb"}},
		{name: `'a b' "c d"`, text: `'a b' "c d"`, want: []string{"a b", "c d"}},
		{name: "an unknown escape", text: `n\pm ci`, refused: true},
		{name: "a trailing backslash", text: `npm ci \`, refused: true},
		{name: "an unclosed quote", text: `'npm ci`, refused: true},
		{name: "the empty string", text: "", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := splitString(tc.text)
			if tc.refused {
				assert.Check(t, !ok, "%q should be refused", tc.text)
				return
			}
			assert.Check(t, ok, "%q", tc.text)
			assert.Check(t, cmp.DeepEqual(got, tc.want), "%q", tc.text)
		})
	}
}

// A job is a candidate when a step that runs installs a package manager's
// dependencies and nothing in the job is uninspectable. Commands are read
// as shell words, through wrappers and function calls.
func TestCandidate(t *testing.T) {
	tests := []struct {
		name   string
		script string
		when   string
		want   bool
	}{
		{name: "an install", script: "npm ci", want: true},
		{name: "an install only on failure", script: "npm ci", when: "on_fail"},
		{name: "a wrapper option with a value", script: "env -u GOPROXY go mod download", want: true},
		{name: "a long wrapper option", script: "sudo --user circleci npm ci", want: true},
		{name: "sudo --chroot takes a value", script: "sudo --chroot /tmp npm ci", want: true},
		{name: "env --argv0 takes a value", script: "env --argv0 custom-name npm ci", want: true},
		{name: "env -S splits its string", script: "env -S 'npm ci'", want: true},
		{name: "env -S then reads assignments", script: "env -S 'NODE_ENV=test npm ci'", want: true},
		{name: "env -S with an empty string", script: "env -S '' npm ci", want: true},
		{name: "env -S does not split on ;", script: "env -S 'npm ci; --version'"},
		{name: "env refuses an invalid split string", script: `env -S 'bad\q' npm ci`},
		{name: "a path-qualified Maven wrapper with a goal", script: "/workspace/mvnw package", want: true},
		{name: "a Maven flag before the goal", script: "mvn -itr verify", want: true},
		{name: "a Maven version query", script: "mvn --version"},
		{name: "a version query after an option's value", script: "mvn -f pom.xml --version"},
		{name: "a Maven option's value is not a goal", script: "mvn --color never"},
		{name: "a called function", script: "install_deps() {\n  npm ci\n}\ninstall_deps", want: true},
		{name: "a function never called", script: "install_deps() {\n  npm ci\n}\necho never called"},
		{name: "a call before the definition", script: "install_deps\ninstall_deps() { npm ci; }"},
		{name: "a call runs the definition in force", script: "install_deps() { true; }\ninstall_deps\ninstall_deps() { npm ci; }"},
		{name: "an unreviewed CircleCI function blocks it", script: "circleci run setup-go\ngo mod download"},
		{name: "a wrapped CircleCI function blocks it", script: "env circleci run setup-go\ngo mod download"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run := map[string]string{"command": tc.script}
			if tc.when != "" {
				run["when"] = tc.when
			}
			// JSON is YAML, and encoding it quotes the script.
			cfg, err := json.Marshal(map[string]any{"version": 2, "jobs": map[string]any{
				"j": map[string]any{"docker": []any{map[string]string{"image": "x"}}, "steps": []any{map[string]any{"run": run}}},
			}})
			assert.NilError(t, err)
			eff, err := pipelineconfig.NewEffective(cfg)
			assert.NilError(t, err)
			a := Analyze(eff)
			assert.Assert(t, cmp.Len(a.Jobs, 1))
			jv := a.Jobs[0]
			assert.Check(t, cmp.Equal(jv.Candidate(), tc.want), "installs %v, blocked %v", jv.Installs, jv.Blocked)
		})
	}
}
