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
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/root"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func TestUsage(t *testing.T) {
	// Clear PATH so extension discovery produces a stable, empty set regardless
	// of what extensions happen to be installed in the test environment.
	t.Setenv("PATH", "")
	// Avoid telemetry
	t.Setenv("DO_NOT_TRACK", "1")
	cmd := root.NewRootCmd("1.2.3")
	// Use insecure storage so the test never touches the OS keychain. Parse it
	// rather than Set() so it merges into the command's flag set the same way a
	// real invocation does — IsSecureStorage reads cmd.Root().Flags().
	assert.NilError(t, cmd.ParseFlags([]string{"--insecure-storage"}))
	testSubCommandUsage(t, cmd.Name(), cmd, "usage", func(cmd *cobra.Command) error {
		return cmd.Usage()
	})
}

func TestHelp(t *testing.T) {
	// Clear PATH so extension discovery produces a stable, empty set regardless
	// of what extensions happen to be installed in the test environment.
	t.Setenv("PATH", "")
	// Avoid telemetry
	t.Setenv("DO_NOT_TRACK", "1")
	cmd := root.NewRootCmd("1.2.3")
	// Use insecure storage so the test never touches the OS keychain. Parse it
	// rather than Set() so it merges into the command's flag set the same way a
	// real invocation does — IsSecureStorage reads cmd.Root().Flags().
	assert.NilError(t, cmd.ParseFlags([]string{"--insecure-storage"}))
	testSubCommandUsage(t, cmd.Name(), cmd, "help", func(cmd *cobra.Command) error {
		return cmd.Help()
	})
}

func testSubCommandUsage(t *testing.T, prefix string, parent *cobra.Command, baseDir string, f func(*cobra.Command) error) {
	t.Helper()
	t.Run(parent.Name(), func(t *testing.T) {
		bb := new(bytes.Buffer)
		parent.SetOut(bb)
		parent.SetErr(bb)

		ctx := iostream.FromCmd(context.Background(), parent, "")
		parent.SetContext(ctx)

		err := f(parent)
		assert.NilError(t, err)

		usageString := bb.String()

		assert.Check(t, golden.String(usageString, path.Join(baseDir, fmt.Sprintf("%s.txt", prefix))))
		for _, cmd := range parent.Commands() {
			testSubCommandUsage(t, path.Join(prefix, cmd.Name()), cmd, baseDir, f)
		}
	})
}
