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

package runner

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestInstanceStatus(t *testing.T) {
	format := func(d time.Duration) string {
		return time.Now().Add(d).UTC().Format(time.RFC3339Nano)
	}

	t.Run("online when connected under 2 minutes ago", func(t *testing.T) {
		status := instanceStatus(format(-1 * time.Minute))
		assert.Check(t, cmp.Equal(status, "online"))
	})

	t.Run("online at zero age", func(t *testing.T) {
		status := instanceStatus(format(0))
		assert.Check(t, cmp.Equal(status, "online"))
	})

	t.Run("idle when connected 2 to 30 minutes ago", func(t *testing.T) {
		status := instanceStatus(format(-10 * time.Minute))
		assert.Check(t, cmp.Equal(status, "idle"))
	})

	t.Run("offline when connected over 30 minutes ago", func(t *testing.T) {
		status := instanceStatus(format(-45 * time.Minute))
		assert.Check(t, cmp.Equal(status, "offline"))
	})

	t.Run("unknown on empty string", func(t *testing.T) {
		status := instanceStatus("")
		assert.Check(t, cmp.Equal(status, "unknown"))
	})

	t.Run("unknown on unparseable string", func(t *testing.T) {
		status := instanceStatus("not-a-timestamp")
		assert.Check(t, cmp.Equal(status, "unknown"))
	})

	t.Run("accepts RFC3339 without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
		status := instanceStatus(ts)
		assert.Check(t, cmp.Equal(status, "idle"))
	})

	t.Run("accepts legacy Z-suffix format without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
		status := instanceStatus(ts)
		assert.Check(t, cmp.Equal(status, "idle"))
	})

	t.Run("accepts legacy Z-suffix format with nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05.999999999Z")
		status := instanceStatus(ts)
		assert.Check(t, cmp.Equal(status, "idle"))
	})

	t.Run("boundary: exactly 2 minutes is idle not online", func(t *testing.T) {
		ts := time.Now().Add(-2*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		status := instanceStatus(ts)
		assert.Check(t, cmp.Equal(status, "idle"))
	})

	t.Run("boundary: exactly 30 minutes is offline not idle", func(t *testing.T) {
		ts := time.Now().Add(-30*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		status := instanceStatus(ts)
		assert.Check(t, cmp.Equal(status, "offline"))
	})

	t.Run("future timestamp is online", func(t *testing.T) {
		ts := time.Now().Add(1 * time.Minute).UTC().Format(time.RFC3339Nano)
		status := instanceStatus(ts) // age is negative, < 2min
		assert.Check(t, cmp.Equal(status, "online"))
	})
}
