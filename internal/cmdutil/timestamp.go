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

import "time"

// Timestamps follow one rule (agents/04-output.md): JSON carries RFC 3339 in
// UTC, and human output shows local time — the zone the TZ environment variable
// selects — with the zone named, so the reader never has to guess it.

const (
	displayLayout        = "2006-01-02 15:04 MST"
	displayLayoutSeconds = "2006-01-02 15:04:05 MST"
)

// DisplayTime renders t for human output in the local time zone, to the
// minute. The zero time renders as "".
func DisplayTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(displayLayout)
}

// DisplayTimeSeconds is DisplayTime to the second, for single-resource detail
// views where the extra precision is worth the width.
func DisplayTimeSeconds(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(displayLayoutSeconds)
}

// DisplayTimeString is DisplayTime for a timestamp the API hands over as a
// string. A value that does not parse as RFC 3339 is returned unchanged, so a
// surprising format is still shown rather than dropped.
func DisplayTimeString(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return DisplayTime(t)
}

// JSONTime renders t for JSON output: RFC 3339 in UTC. The zero time renders
// as "", for fields that are empty when the API leaves the time out.
func JSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
