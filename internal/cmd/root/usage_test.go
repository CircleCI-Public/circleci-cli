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
	"path"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/root"
)

func TestUsage(t *testing.T) {
	cmd := root.NewRootCmd("1.2.3")
	testSubCommandUsage(t, cmd.Name(), cmd)
}

func testSubCommandUsage(t *testing.T, prefix string, parent *cobra.Command) {
	t.Helper()
	t.Run(parent.Name(), func(t *testing.T) {
		golden.Assert(t, parent.UsageString(), path.Join("usage", fmt.Sprintf("%s.txt", prefix)))
		for _, cmd := range parent.Commands() {
			testSubCommandUsage(t, fmt.Sprintf("%s/%s", prefix, cmd.Name()), cmd)
		}
	})
}
