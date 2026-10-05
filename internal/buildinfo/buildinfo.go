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

// Package buildinfo reads what the Go toolchain stamps into the binary when it
// is built from a git checkout.
package buildinfo

import (
	"runtime/debug"
	"time"
)

// AgeDays returns how many whole days before now the source of the running
// binary was committed. Release builds are cut from the tagged commit, so for
// them this is the age of the release.
//
// ok is false when the binary carries no commit time: go run, go install from
// the module proxy, or a build with -buildvcs=false.
func AgeDays(now time.Time) (days int, ok bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return 0, false
	}
	return ageDays(bi.Settings, now)
}

// ageDays is AgeDays over an explicit set of build settings, for tests.
func ageDays(settings []debug.BuildSetting, now time.Time) (int, bool) {
	for _, s := range settings {
		if s.Key != "vcs.time" {
			continue
		}
		committed, err := time.Parse(time.RFC3339, s.Value)
		if err != nil {
			return 0, false
		}
		// A commit time ahead of the local clock (clock skew) is not a negative age.
		return max(0, int(now.Sub(committed)/(24*time.Hour))), true
	}
	return 0, false
}
