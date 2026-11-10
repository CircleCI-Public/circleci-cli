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
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestTimestamps(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	assert.NilError(t, err)
	// 14:30:45 UTC on a winter date, so New York is on EST (UTC-5).
	instant := time.Date(2026, 1, 15, 14, 30, 45, 0, time.UTC)

	t.Run("display uses the local zone and names it", func(t *testing.T) {
		withLocal(t, newYork)
		assert.Check(t, cmp.Equal(DisplayTime(instant), "2026-01-15 09:30 EST"))
		assert.Check(t, cmp.Equal(DisplayTimeSeconds(instant), "2026-01-15 09:30:45 EST"))
	})

	t.Run("display in UTC", func(t *testing.T) {
		withLocal(t, time.UTC)
		assert.Check(t, cmp.Equal(DisplayTime(instant), "2026-01-15 14:30 UTC"))
	})

	t.Run("display parses API strings", func(t *testing.T) {
		withLocal(t, newYork)
		assert.Check(t, cmp.Equal(DisplayTimeString("2026-01-15T14:30:45.123Z"), "2026-01-15 09:30 EST"))
		assert.Check(t, cmp.Equal(DisplayTimeString("2026-01-15T14:30:45+00:00"), "2026-01-15 09:30 EST"))
	})

	t.Run("display leaves unparseable strings alone", func(t *testing.T) {
		assert.Check(t, cmp.Equal(DisplayTimeString("yesterday"), "yesterday"))
		assert.Check(t, cmp.Equal(DisplayTimeString(""), ""))
	})

	t.Run("JSON is RFC 3339 in UTC whatever the input zone", func(t *testing.T) {
		withLocal(t, newYork)
		assert.Check(t, cmp.Equal(JSONTime(instant.In(newYork)), "2026-01-15T14:30:45Z"))
	})

	t.Run("zero time renders empty", func(t *testing.T) {
		assert.Check(t, cmp.Equal(DisplayTime(time.Time{}), ""))
		assert.Check(t, cmp.Equal(DisplayTimeSeconds(time.Time{}), ""))
		assert.Check(t, cmp.Equal(JSONTime(time.Time{}), ""))
	})
}

// withLocal swaps time.Local for the duration of a subtest. Not parallel-safe,
// so these subtests must not call t.Parallel.
func withLocal(t *testing.T, loc *time.Location) {
	t.Helper()
	orig := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = orig })
}
