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
	"bytes"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/root"
	"github.com/CircleCI-Public/circleci-cli/internal/iostreamcobra"
)

// Golden subdirectories, one per rendering: `circleci <cmd> --help` and the
// terse usage shown on a parse error. Only the help pages carry a line budget.
const (
	usageGoldenDir = "usage"
	helpGoldenDir  = "help"
)

// helpLineBudget is the maximum number of lines a command's --help may occupy.
// Agents that read CLI help capture only the first ~40 lines; past that a page
// silently loses its flag table, arguments or examples, which is precisely the
// signal an agent came for. Prose is what yields — see
// agents/03-help-and-documentation.md.
const helpLineBudget = 40

// maxOverBudget bounds the allow-list below so it cannot quietly grow. It is a
// ratchet: lower it as entries are removed. Growing it is a deliberate act that
// needs a reason in review.
const maxOverBudget = 25

// unbudgeted commands are long-form by design. A reader reaching for them wants
// the whole inventory, and truncating it degrades gracefully. `circleci help
// <topic>` pages are exempt too, but by annotation rather than by name — see
// checkHelpLineBudget.
var unbudgeted = map[string]bool{
	"circleci": true, // root: the command inventory
}

// overBudget caps every command whose --help does not yet fit helpLineBudget.
//
// Each entry is a ceiling that may only ever go DOWN. To bring one down, trim
// the command's Long prose — never drop a flag row or an example to make budget.
// The test also fails once a command comes back inside the budget, which forces
// its entry to be deleted rather than left parked; without that, an allow-list
// rots into a permanent excuse.
var overBudget = map[string]int{
	"circleci/api":                    43,
	"circleci/config/process":         41,
	"circleci/context/get":            43,
	"circleci/context/secret/list":    42,
	"circleci/job/output/get":         42,
	"circleci/job/output/list":        43,
	"circleci/orb":                    41,
	"circleci/orb/init":               42,
	"circleci/orb/list":               48,
	"circleci/pipeline/create":        42,
	"circleci/pipeline/run":           42,
	"circleci/policy":                 42,
	"circleci/policy/eval":            50,
	"circleci/policy/logs":            52,
	"circleci/policy/push":            43,
	"circleci/policy/test":            48,
	"circleci/project":                42,
	"circleci/project/create":         41,
	"circleci/project/trigger/create": 41,
	"circleci/run/get":                53,
	"circleci/run/list":               49,
	"circleci/run/watch":              48,
	"circleci/testresult/get":         44,
	"circleci/testresult/list":        47,
	"circleci/workflow/list":          48,
}

func TestUsage(t *testing.T) {
	// Use temp XDG_DATA_HOME and PATH so extension discovery produces a stable, empty
	// set regardless of what extensions happen to be installed in the test
	// environment.
	fakeHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("PATH", fakeHome)

	// Avoid telemetry
	t.Setenv("DO_NOT_TRACK", "1")
	cmd := root.NewRootCmd("1.2.3")
	// Execute() lazily registers the default --help and --version flags; the
	// help/usage funcs read --version, so register them here since we invoke
	// Usage()/Help() directly without going through Execute().
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	// Use insecure storage so the test never touches the OS keychain. Parse it
	// rather than Set() so it merges into the command's flag set the same way a
	// real invocation does — IsSecureStorage reads cmd.Root().Flags().
	assert.NilError(t, cmd.ParseFlags([]string{"--insecure-storage"}))
	testSubCommandUsage(t, cmd.Name(), cmd, usageGoldenDir, func(cmd *cobra.Command) error {
		return cmd.Usage()
	})
}

func TestHelp(t *testing.T) {
	// Use temp XDG_DATA_HOME and PATH so extension discovery produces a stable, empty
	// set regardless of what extensions happen to be installed in the test
	// environment.
	fakeHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("PATH", fakeHome)

	// Avoid telemetry
	t.Setenv("DO_NOT_TRACK", "1")
	cmd := root.NewRootCmd("1.2.3")
	// Execute() lazily registers the default --help and --version flags; the
	// help/usage funcs read --version, so register them here since we invoke
	// Usage()/Help() directly without going through Execute().
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	// Use insecure storage so the test never touches the OS keychain. Parse it
	// rather than Set() so it merges into the command's flag set the same way a
	// real invocation does — IsSecureStorage reads cmd.Root().Flags().
	assert.NilError(t, cmd.ParseFlags([]string{"--insecure-storage"}))
	testSubCommandUsage(t, cmd.Name(), cmd, helpGoldenDir, func(cmd *cobra.Command) error {
		return cmd.Help()
	})

	entries := len(overBudget)
	assert.Check(t, entries <= maxOverBudget,
		"the over-budget allow-list has %d entries, above the %d bound; it must only ever shrink",
		entries, maxOverBudget)
}

func testSubCommandUsage(t *testing.T, prefix string, parent *cobra.Command, baseDir string, f func(*cobra.Command) error) {
	t.Helper()
	t.Run(parent.Name(), func(t *testing.T) {
		// Execute() registers --help lazily on the command it runs. We invoke
		// Help() directly, so without this the rendered flag set differs from a
		// real invocation's and the line counts below would understate what a
		// user actually sees.
		parent.InitDefaultHelpFlag()

		bb := new(bytes.Buffer)
		parent.SetOut(bb)
		parent.SetErr(bb)

		ctx := iostreamcobra.FromCmd(context.Background(), parent, "")
		parent.SetContext(ctx)

		err := f(parent)
		assert.NilError(t, err)

		usageString := bb.String()

		assert.Check(t, golden.String(usageString, path.Join(baseDir, fmt.Sprintf("%s.txt", prefix))))
		if baseDir == helpGoldenDir {
			checkHelpLineBudget(t, prefix, parent, usageString)
		}

		for _, cmd := range parent.Commands() {
			// Skip hidden commands (man, setup, the completion shell scripts)
			// whose output is nondeterministic or not user-facing.
			//
			// Help topics are the exception: they are hidden from the command
			// listing but are user-facing documentation reached as
			// `circleci help <topic>`, and they feed the published reference and
			// llms.txt, so their goldens must stay in sync as the CLI changes.
			if cmd.Hidden && cmd.Annotations[root.HelpTopicAnnotation] == "" {
				continue
			}
			testSubCommandUsage(t, path.Join(prefix, cmd.Name()), cmd, baseDir, f)
		}
	})
}

// checkHelpLineBudget asserts that a command's rendered --help fits within
// helpLineBudget, or within its allow-listed cap. The failure messages name the
// remedy on purpose: the cheap way to make budget is to delete examples, which
// are the most-read part of a help page and the last thing that should go.
func checkHelpLineBudget(t *testing.T, prefix string, cmd *cobra.Command, helpText string) {
	t.Helper()

	// Help topics are explicitly long-form reference material, not command help.
	if unbudgeted[prefix] || cmd.Annotations[root.HelpTopicAnnotation] != "" {
		return
	}

	// Count real lines: the rendered page ends in a newline, which would
	// otherwise register as an extra empty line.
	got := len(strings.Split(strings.TrimRight(helpText, "\n"), "\n"))

	limit, allowed := overBudget[prefix]
	if !allowed {
		assert.Check(t, got <= helpLineBudget,
			"%s --help is %d lines; the budget is %d. Trim Long — never Flags or Examples. "+
				"If the flag table alone cannot fit, add an overBudget entry and justify it in review.",
			prefix, got, helpLineBudget)
		return
	}

	assert.Check(t, got <= limit,
		"%s --help is %d lines, over its allow-listed cap of %d. Trim Long or Examples "+
			"— never Flags. Do not raise the cap.",
		prefix, got, limit)

	assert.Check(t, got > helpLineBudget,
		"%s --help is now %d lines, within the %d budget — delete its overBudget entry "+
			"and lower maxOverBudget.",
		prefix, got, helpLineBudget)
}
