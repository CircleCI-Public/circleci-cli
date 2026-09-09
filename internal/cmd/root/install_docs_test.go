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

package root_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

// The install commands are published in two places for two different audiences:
//
//   - the `getting-started` help topic, which feeds `circleci help
//     getting-started`, llms.txt, /reference, the man page and the MCP tool
//     description; and
//   - docs/website/data/install.yaml, which the landing page renders as the
//     install picker and as its "All platforms" list.
//
// The second cannot be generated from the first: the picker needs per-platform
// structured data (a label, an OS to match against, an ordered command pair)
// that prose does not carry. So the two are kept in step by this test rather
// than by hope — a command added to the site and not to the help topic (or the
// reverse) fails here.
//
// Labels are deliberately not compared. The picker lists "macOS — Homebrew" and
// "Linux — Homebrew" separately so it can pre-select by OS, while the topic
// says "macOS or Linux — Homebrew" once; both are correct for their medium.
func TestInstallCommandsMatchHelpTopic(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")

	// The rendered topic, not the Go source: this is the exact text that reaches
	// llms.txt and /reference, and TestHelp keeps it current.
	topic, err := os.ReadFile(filepath.Join("testdata", "help", "circleci", "getting-started.txt"))
	assert.NilError(t, err)
	haystack := normalizeCommand(string(topic))

	sitePath := filepath.Join(repoRoot, "docs", "website", "data", "install.yaml")
	raw, err := os.ReadFile(sitePath)
	assert.NilError(t, err)

	var data struct {
		Groups []struct {
			Options []struct {
				Label    string   `yaml:"label"`
				Platform string   `yaml:"platform"`
				Cmds     []string `yaml:"cmds"`
			} `yaml:"options"`
		} `yaml:"groups"`
	}
	assert.NilError(t, yaml.Unmarshal(raw, &data))

	seen := 0
	for _, group := range data.Groups {
		for _, opt := range group.Options {
			assert.Assert(t, opt.Label != "", "every install option needs a label")
			assert.Assert(t, len(opt.Cmds) > 0 && len(opt.Cmds) < 3,
				"option %q: expected one or two commands, got %d", opt.Label, len(opt.Cmds))

			for _, cmd := range opt.Cmds {
				seen++
				assert.Assert(t, strings.Contains(haystack, normalizeCommand(cmd)),
					"install command %q (%s) is in docs/website/data/install.yaml but not in the "+
						"getting-started help topic (internal/cmd/root/help_topic.go). Add it there "+
						"and re-run `go test ./internal/cmd/root/ -update`.", cmd, opt.Label)
			}
		}
	}
	assert.Assert(t, seen >= 6, "expected the site to list at least six install commands, got %d", seen)
}

// normalizeCommand flattens a shell command so the site's copy and the help
// topic's copy compare equal. The site breaks the apt and rpm setup commands
// across lines with backslash continuations so they do not stretch the hero
// column; the help topic writes each on one line.
func normalizeCommand(s string) string {
	s = strings.ReplaceAll(s, "\\\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
