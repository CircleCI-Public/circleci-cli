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
)

func TestInstanceStatus(t *testing.T) {
	format := func(d time.Duration) string {
		return time.Now().Add(d).UTC().Format(time.RFC3339Nano)
	}

	t.Run("online when connected under 2 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-1*time.Minute)), "online")
	})

	t.Run("online at zero age", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(0)), "online")
	})

	t.Run("idle when connected 2 to 30 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-10*time.Minute)), "idle")
	})

	t.Run("offline when connected over 30 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-45*time.Minute)), "offline")
	})

	t.Run("unknown on empty string", func(t *testing.T) {
		assert.Equal(t, instanceStatus(""), "unknown")
	})

	t.Run("unknown on unparseable string", func(t *testing.T) {
		assert.Equal(t, instanceStatus("not-a-timestamp"), "unknown")
	})

	t.Run("accepts RFC3339 without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("accepts legacy Z-suffix format without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("accepts legacy Z-suffix format with nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05.999999999Z")
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("boundary: exactly 2 minutes is idle not online", func(t *testing.T) {
		ts := time.Now().Add(-2*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("boundary: exactly 30 minutes is offline not idle", func(t *testing.T) {
		ts := time.Now().Add(-30*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "offline")
	})

	t.Run("future timestamp is online", func(t *testing.T) {
		ts := time.Now().Add(1 * time.Minute).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "online") // age is negative, < 2min
	})
}
