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

package extension

import (
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/extension"
)

func TestAddReferenceCmds(t *testing.T) {
	parent := &cobra.Command{Use: "ext", GroupID: "extension", DisableFlagParsing: true, RunE: func(*cobra.Command, []string) error { return nil }}
	ref := &extension.Reference{
		Flags: []extension.ReferenceFlag{{Name: "local"}},
		Subcommands: []extension.ReferenceSub{
			{Name: "run",
				Use: "<suite> [flags]",
				Flags: []extension.ReferenceFlag{
					{Name: "runner", Type: "auto|jest", Default: "auto", Usage: "Runner"},
					{Name: "dry-run", Default: "true"},
					{Name: "runner", Type: "string"},
					{Name: "Bad_Flag"},
				}},
			{Name: "run"},
			{Name: "../escape"},
			{Name: "Upper"},
		},
	}

	addReferenceCmds(parent, ref)

	t.Run("only valid, unique subcommands become commands", func(t *testing.T) {
		assert.Assert(t, cmp.Len(parent.Commands(), 1))
		assert.Check(t, cmp.Equal(parent.Commands()[0].Use, "run <suite> [flags]"))
	})

	run := parent.Commands()[0]

	t.Run("the child runs the extension and counts as an extension command", func(t *testing.T) {
		assert.Check(t, run.RunE != nil)
		assert.Check(t, IsExtensionCmd(run))
	})

	t.Run("flags are typed from the reference", func(t *testing.T) {
		runner := run.Flags().Lookup("runner")
		assert.Assert(t, runner != nil)
		assert.Check(t, cmp.Equal(runner.Value.Type(), "string"))
		assert.Check(t, cmp.Equal(runner.DefValue, "auto"))
		assert.Check(t, cmp.Equal(runner.Usage, "Runner (auto|jest)"))

		dryRun := run.Flags().Lookup("dry-run")
		assert.Assert(t, dryRun != nil)
		assert.Check(t, cmp.Equal(dryRun.Value.Type(), "bool"))
		assert.Check(t, cmp.Equal(dryRun.DefValue, "true"))

		local := run.Flags().Lookup("local")
		assert.Assert(t, local != nil, "the extension's own flags are added to each subcommand")
		assert.Check(t, cmp.Equal(local.Value.Type(), "bool"))
	})

	t.Run("invalid flag names are skipped", func(t *testing.T) {
		assert.Check(t, cmp.Nil(run.Flags().Lookup("Bad_Flag")))
	})
}

func TestIsExtensionCmd(t *testing.T) {
	root := &cobra.Command{Use: "circleci"}
	ext := &cobra.Command{Use: "ext", GroupID: "extension", DisableFlagParsing: true}
	mgmt := &cobra.Command{Use: "extension", GroupID: "extension"}
	other := &cobra.Command{Use: "halt", DisableFlagParsing: true}
	root.AddCommand(ext, mgmt, other)

	assert.Check(t, IsExtensionCmd(ext))
	assert.Check(t, !IsExtensionCmd(mgmt), "the extension management command parses its own flags")
	assert.Check(t, !IsExtensionCmd(other), "disabling flag parsing alone does not make an extension")
	assert.Check(t, !IsExtensionCmd(root))
}
