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

//go:build darwin

package publish_test

import (
	"os/exec"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// macOS hides ACLs from listxattr and keeps file flags in the stat
// structure, so both need their own check.
func TestInPlaceDarwinMetadata(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "an ACL", args: []string{"chmod", "+a", "everyone allow read"}},
		{name: "file flags", args: []string{"chflags", "nodump"}},
	}
	for _, tc := range tests {
		t.Run("refuses "+tc.name+" without --force", func(t *testing.T) {
			_, path := gitRepo(t)
			out, err := exec.Command(tc.args[0], append(tc.args[1:], path)...).CombinedOutput() //#nosec:G204 // fixed test commands
			assert.NilError(t, err, "%s", out)
			assert.Check(t, cmp.Equal(refusedCode(t, preflightInPlace(path, false)), "output.metadata"))
			assert.NilError(t, preflightInPlace(path, true), "--force accepts the loss")
		})
	}
}
