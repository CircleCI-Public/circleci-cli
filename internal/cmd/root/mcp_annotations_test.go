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
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/root"
)

// TestDestructiveHintHasForceFlag enforces that every command marked
// "destructiveHint": "true" exposes a --force / -f flag. Without this guard a
// developer could annotate a new destructive command without wiring the
// confirmation prompt, or remove the flag while leaving the annotation behind.
func TestDestructiveHintHasForceFlag(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("PATH", fakeHome)
	t.Setenv("DO_NOT_TRACK", "1")

	cmd := root.NewRootCmd("test")

	var violations []string
	walkCommands(cmd, func(c *cobra.Command) {
		if c.Annotations["destructiveHint"] != "true" {
			return
		}
		f := c.Flags().ShorthandLookup("f")
		if f == nil || f.Name != "force" {
			violations = append(violations, fmt.Sprintf("%s: has destructiveHint but no --force / -f flag", c.CommandPath()))
		}
	})

	assert.Assert(t, len(violations) == 0,
		"commands with destructiveHint must expose --force (-f):\n%v", violations)
}

// TestForceFlagHasDestructiveHint is the inverse: every command with a
// --force / -f flag should be marked destructiveHint so MCP clients know it
// can mutate state. Commands that use --force for non-destructive purposes
// (e.g. overwriting a local file) are listed in the allowlist below.
func TestForceFlagHasDestructiveHint(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("PATH", fakeHome)
	t.Setenv("DO_NOT_TRACK", "1")

	// Commands whose --force is NOT a destructive API operation (e.g. overwrite
	// a local config file). Add to this list only with a comment explaining why.
	allowlist := map[string]bool{
		"circleci project link": true, // --force overwrites .circleci/config.yml locally, no API mutation
	}

	cmd := root.NewRootCmd("test")

	var violations []string
	walkCommands(cmd, func(c *cobra.Command) {
		f := c.Flags().ShorthandLookup("f")
		if f == nil || f.Name != "force" {
			return
		}
		if allowlist[c.CommandPath()] {
			return
		}
		if c.Annotations["destructiveHint"] != "true" {
			violations = append(violations, fmt.Sprintf("%s: has --force but no destructiveHint annotation", c.CommandPath()))
		}
	})

	assert.Assert(t, len(violations) == 0,
		"commands with --force (-f) must have destructiveHint annotation (or be added to the allowlist):\n%v", violations)
}

func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, fn)
	}
}
