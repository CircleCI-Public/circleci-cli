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

package keyring

import (
	"context"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

// On a headless Unix host with neither a session bus nor dbus-launch, the
// keyring must report unavailable and short-circuit every operation with
// ErrUnavailable rather than letting the backend shell out to a missing
// dbus-launch. Setting an empty PATH guarantees the LookPath probe fails.
func TestUnavailableOnHeadlessUnix(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" &&
		runtime.GOOS != "netbsd" && runtime.GOOS != "openbsd" &&
		runtime.GOOS != "dragonfly" {
		t.Skipf("secret-service backend not used on %s", runtime.GOOS)
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("PATH", "")

	assert.Check(t, !Available(), "expected keyring to be unavailable")

	ctx := context.Background()
	assert.ErrorIs(t, Set(ctx, "svc", "user", "secret"), ErrUnavailable)
	_, err := Get(ctx, "svc", "user")
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.ErrorIs(t, Delete(ctx, "svc", "user"), ErrUnavailable)
}

// A reachable session bus is sufficient for availability even when dbus-launch
// is absent, because the backend never needs to start a bus.
func TestAvailableWithSessionBus(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("session-bus heuristic only exercised on linux, not %s", runtime.GOOS)
	}

	t.Setenv("PATH", "")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus")

	assert.Check(t, Available(), "expected keyring to be available with a session bus")
}
