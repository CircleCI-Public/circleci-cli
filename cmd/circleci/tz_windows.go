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

//go:build windows

package main

import (
	"os"
	"time"

	// Windows has no system zoneinfo database, and a user's machine has no Go
	// installation to fall back on, so embed one for LoadLocation below.
	_ "time/tzdata"
)

// applyTZ sets time.Local from TZ. Go does this itself on Unix but ignores TZ
// on Windows, where it reads the zone from the registry instead; this makes the
// human output's timestamps (cmdutil.DisplayTime) follow TZ on every platform,
// as `circleci help environment` documents. An unknown zone keeps the system one.
func applyTZ() {
	tz := os.Getenv("TZ")
	if tz == "" {
		return
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		time.Local = loc
	}
}
