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
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// harmlessFlags are file flags macOS sets on its own whose loss changes
// nothing the user relies on: transparent compression and document-ID
// tracking.
const harmlessFlags = unix.UF_COMPRESSED | unix.UF_TRACKED

// hiddenMetadata lists what a replacement would drop that
// extendedAttributes cannot see: file flags, and an ACL, which macOS keeps
// out of listxattr and, without cgo, is only readable through ls -e.
func hiddenMetadata(path string, st *syscall.Stat_t) ([]string, error) {
	var found []string
	if flags := st.Flags &^ harmlessFlags; flags != 0 {
		found = append(found, fmt.Sprintf("file flags (%#x)", flags))
	}
	// -n prints numeric owners, -i the inode, -q replaces unprintable
	// characters in the name, so a name cannot add lines.
	cmd := exec.Command("/bin/ls", "-ledniq", "--", path) //#nosec:G204 // fixed ls invocation
	cmd.Env = []string{"LC_ALL=C"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ls: %w", err)
	}
	acl, err := parseACL(stdout.String(), stderr.String(), st)
	if err != nil {
		return nil, err
	}
	if acl {
		found = append(found, "an ACL")
	}
	return found, nil
}

var (
	// lsFile is the file's own line: inode, a regular file's mode, the
	// marker (' ', '@' for xattrs, '+' for an ACL without xattrs), link
	// count, uid, gid, size, then the date and name.
	lsFile = regexp.MustCompile(`^(\d+) -[-rwxsStT]{9}([ @+]) +\d+ +(\d+) +(\d+) +\d+ \S`)
	// lsACLEntry is one ACL entry line, e.g. " 0: <uuid> allow read".
	lsACLEntry = regexp.MustCompile(`^ \d+: \S`)
)

// parseACL reads ls -ledniq output. It fails closed: anything other than
// one line for this very file (same inode, uid and gid), optionally
// followed by ACL entry lines, is an error, never "no ACL".
func parseACL(stdout, stderr string, st *syscall.Stat_t) (bool, error) {
	unrecognised := func(why string) error {
		return errors.New("unrecognised ls -e output (" + why + ")")
	}
	switch {
	case stderr != "":
		return false, unrecognised("it wrote to stderr")
	case !strings.HasSuffix(stdout, "\n"):
		return false, unrecognised("no final newline")
	case strings.ContainsAny(stdout, "\r\x00"):
		return false, unrecognised("a CR or NUL byte")
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	m := lsFile.FindStringSubmatch(lines[0])
	if m == nil {
		return false, unrecognised("the file's line has an unknown format")
	}
	if m[1] != strconv.FormatUint(st.Ino, 10) || m[3] != strconv.FormatUint(uint64(st.Uid), 10) ||
		m[4] != strconv.FormatUint(uint64(st.Gid), 10) {
		return false, unrecognised("it describes a different file")
	}
	for _, l := range lines[1:] {
		if !lsACLEntry.MatchString(l) {
			return false, unrecognised("a line that is not an ACL entry")
		}
	}
	acl := len(lines) > 1
	if m[2] == "+" && !acl {
		return false, unrecognised("an ACL marker with no entries")
	}
	return acl, nil
}
