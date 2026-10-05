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

package publish

import (
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// The ACL check must fail closed: any output that
// is not positively "this file, no ACL" or "this file, these entries" is an
// error.
func TestParseACL(t *testing.T) {
	st := &syscall.Stat_t{Ino: 42, Uid: 501, Gid: 20}
	const file = "42 -rw-r--r--@ 1 501  20  7 Sep 24 23:30 /x/config.yml\n"
	tests := []struct {
		name, stdout, stderr string
		acl                  bool
		err                  string
	}{
		{name: "no ACL", stdout: file},
		{name: "no marker", stdout: "42 -rw-r--r--  1 501  20  7 Sep 24 23:30 /x/config.yml\n"},
		{name: "an ACL", stdout: file + " 0: ABCDEFAB-CDEF-ABCD-EFAB-CDEF0000000C allow read\n", acl: true},
		{name: "ACL marker with entries", stdout: "42 -rw-r--r--+ 1 501  20  7 Sep 24 23:30 /x/config.yml\n 0: group:everyone deny delete\n", acl: true},
		{name: "empty", stdout: "", err: "no final newline"},
		{name: "only a newline", stdout: "\n", err: "unknown format"},
		{name: "missing final newline", stdout: file[:len(file)-1], err: "no final newline"},
		{name: "a blank trailing line", stdout: file + "\n", err: "not an ACL entry"},
		{name: "CRLF", stdout: file[:len(file)-1] + "\r\n", err: "CR or NUL"},
		{name: "stderr on success", stdout: file, stderr: "ls: acl unavailable\n", err: "stderr"},
		{name: "unknown line after the file", stdout: file + "something else\n", err: "not an ACL entry"},
		{name: "ACL marker without entries", stdout: "42 -rw-r--r--+ 1 501  20  7 Sep 24 23:30 /x/config.yml\n", err: "marker with no entries"},
		{name: "a different inode", stdout: "43 -rw-r--r--@ 1 501  20  7 Sep 24 23:30 /x/config.yml\n", err: "different file"},
		{name: "a different owner", stdout: "42 -rw-r--r--@ 1 502  20  7 Sep 24 23:30 /x/config.yml\n", err: "different file"},
		{name: "not a regular file", stdout: "42 prw-r--r--@ 1 501  20  0 Sep 24 23:30 /x/config.yml\n", err: "unknown format"},
		{name: "names instead of ids", stdout: "42 -rw-r--r--@ 1 user  staff  7 Sep 24 23:30 /x/config.yml\n", err: "unknown format"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acl, err := parseACL(tc.stdout, tc.stderr, st)
			if tc.err != "" {
				assert.Check(t, cmp.ErrorContains(err, tc.err))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(acl, tc.acl))
		})
	}
}
