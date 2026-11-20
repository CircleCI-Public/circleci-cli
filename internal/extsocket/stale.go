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

package extsocket

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var staleName = regexp.MustCompile(`^` + dirPrefix + `[0-9]+$`)

// cleanStale removes socket directories in base that a crashed run left
// behind. It is best effort: every error is ignored. It only removes what it
// can prove is its own leftover, so it never follows a symlink and never
// removes another user's directory or one that holds anything else.
func cleanStale(base string, now time.Time, uid int) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !staleName.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(base, e.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if !ownedByUser(info, uid) || now.Sub(info.ModTime()) <= staleAge {
			continue
		}
		children, err := os.ReadDir(path)
		if err != nil || len(children) > 1 || (len(children) == 1 && children[0].Name() != sockName) {
			continue
		}
		_ = os.RemoveAll(path)
	}
}
