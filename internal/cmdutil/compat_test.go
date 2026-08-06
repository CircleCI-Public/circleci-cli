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

package cmdutil

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// newCompatCmd builds `orb validate` in its real topology: an `orb` parent owning
// the auth shims, and a child owning the org shims. It returns the child, so the
// tests exercise the inheritance the shims depend on.
func newCompatCmd(org *string) *cobra.Command {
	root := &cobra.Command{Use: "orb"}
	AddV0AuthCompatFlags(root.PersistentFlags())

	cmd := &cobra.Command{Use: "validate", RunE: func(*cobra.Command, []string) error { return nil }}
	AddOrgFlag(cmd, org, OrgFlag{})
	AddV0OrgCompatFlags(cmd, org)
	root.AddCommand(cmd)
	return cmd
}

func TestApplyCompatFlags(t *testing.T) {
	t.Run("host and token become env vars", func(t *testing.T) {
		t.Setenv("CIRCLE_HOST", "")
		t.Setenv("CIRCLE_TOKEN", "")
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"--host", "https://ci.example.com", "--token", "secret"}))

		ApplyCompatFlags(cmd)

		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_HOST"), "https://ci.example.com"))
		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_TOKEN"), "secret"))
	})

	// 0.1.x precedence: an explicit --token beat CIRCLE_TOKEN. orb-tools' publish
	// job passes a token that is deliberately not the ambient CIRCLE_TOKEN, so the
	// flag has to win or it publishes as the wrong identity.
	t.Run("the flag wins over an existing env var", func(t *testing.T) {
		t.Setenv("CIRCLE_TOKEN", "from-env")
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"--token", "from-flag"}))

		ApplyCompatFlags(cmd)

		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_TOKEN"), "from-flag"))
	})

	t.Run("an env var survives when the flag is absent", func(t *testing.T) {
		t.Setenv("CIRCLE_TOKEN", "from-env")
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"orb.yml"}))

		ApplyCompatFlags(cmd)

		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_TOKEN"), "from-env"))
	})

	t.Run("unpassed flags leave the environment untouched", func(t *testing.T) {
		t.Setenv("CIRCLE_HOST", "")
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"orb.yml"}))

		ApplyCompatFlags(cmd)

		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_HOST"), ""))
	})

	t.Run("a same-named flag without the annotation is left alone", func(t *testing.T) {
		t.Setenv("CIRCLE_TOKEN", "")
		// `runner config --token` means an existing runner token, not an API token.
		cmd := &cobra.Command{Use: "config", RunE: func(*cobra.Command, []string) error { return nil }}
		cmd.Flags().String("token", "", "Use an existing token value")
		assert.NilError(t, cmd.ParseFlags([]string{"--token", "runner-token"}))

		ApplyCompatFlags(cmd)

		assert.Check(t, cmp.Equal(os.Getenv("CIRCLE_TOKEN"), ""))
	})
}

func TestAddV0OrgCompatFlags(t *testing.T) {
	t.Run("org-slug binds to org", func(t *testing.T) {
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"--org-slug", "gh/acme"}))
		assert.Check(t, cmp.Equal(org, "gh/acme"))
	})

	t.Run("org-id binds to org", func(t *testing.T) {
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"--org-id", "0000-uuid"}))
		assert.Check(t, cmp.Equal(org, "0000-uuid"))
	})

	// 0.1.x accepted both at once, so this must resolve rather than error.
	t.Run("org-id and org-slug together are accepted", func(t *testing.T) {
		var org string
		cmd := newCompatCmd(&org)
		assert.NilError(t, cmd.ParseFlags([]string{"--org-id", "gh/acme", "--org-slug", "gh/acme"}))
		assert.Check(t, cmp.Equal(org, "gh/acme"))
	})

	t.Run("shims stay out of help output", func(t *testing.T) {
		var org string
		cmd := newCompatCmd(&org)
		// Parse first: that is what folds the parent's persistent flags in.
		assert.NilError(t, cmd.ParseFlags(nil))
		for _, name := range []string{"host", "token", "org-id", "org-slug"} {
			f := cmd.Flags().Lookup(name)
			assert.Assert(t, f != nil, "expected %q to be registered", name)
			assert.Check(t, f.Hidden, "expected %q to be hidden", name)
		}
	})
}
