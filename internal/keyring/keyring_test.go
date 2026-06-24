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
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
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

// classify converts "the backend is absent or unreachable" failures into
// ErrUnavailable so callers fall back to file storage, while leaving genuine
// backend errors untouched.
func TestClassify(t *testing.T) {
	// A live session bus with no Secret Service provider — the error reported
	// on minimal desktops, fresh containers, and snaps on boxes without a
	// password manager.
	serviceUnknown := dbus.Error{
		Name: "org.freedesktop.DBus.Error.ServiceUnknown",
		Body: []any{"The name org.freedesktop.secrets was not provided by any .service files"},
	}

	unavailable := map[string]error{
		"no secret service provider": serviceUnknown,
		"wrapped service unknown":    fmt.Errorf("set token: %w", serviceUnknown),
		"name has no owner":          dbus.Error{Name: "org.freedesktop.DBus.Error.NameHasNoOwner"},
		"provider failed to spawn":   dbus.Error{Name: "org.freedesktop.DBus.Error.Spawn.ExecFailed"},
		"bus socket denied":          &net.OpError{Op: "dial", Net: "unix", Err: errors.New("connect: permission denied")},
		"dbus-launch missing":        &exec.Error{Name: "dbus-launch", Err: exec.ErrNotFound},
	}
	for name, err := range unavailable {
		t.Run(name, func(t *testing.T) {
			result := classify(err)
			assert.ErrorIs(t, result, ErrUnavailable)
			assert.Check(t, !errors.Is(result, ErrAccessDenied), "must not be flagged access-denied")
		})
	}

	// A sandbox/policy denial the user can fix (e.g. an unconnected snap
	// interface): EACCES on the socket connect, or a D-Bus AccessDenied reply.
	// These map to ErrAccessDenied, which is itself an ErrUnavailable so the
	// fallback still triggers.
	accessDenied := map[string]error{
		"socket connect EACCES": &net.OpError{
			Op:  "dial",
			Net: "unix",
			Err: os.NewSyscallError("connect", syscall.EACCES),
		},
		"dbus access denied": dbus.Error{Name: "org.freedesktop.DBus.Error.AccessDenied"},
	}
	for name, err := range accessDenied {
		t.Run(name, func(t *testing.T) {
			result := classify(err)
			assert.ErrorIs(t, result, ErrAccessDenied)
			assert.ErrorIs(t, result, ErrUnavailable)
		})
	}

	// classify(nil) must stay nil.
	assert.NilError(t, classify(nil))

	// Genuine backend errors must NOT be remapped to ErrUnavailable; they pass
	// through so the caller still sees the real failure. (dbus.Error is
	// uncomparable, so assert via errors.Is, which guards comparability.)
	passthrough := map[string]error{
		"locked collection": dbus.Error{Name: "org.freedesktop.Secret.Error.IsLocked"},
		"generic error":     errors.New("boom"),
	}
	for name, err := range passthrough {
		t.Run(name, func(t *testing.T) {
			assert.Check(t, !errors.Is(classify(err), ErrUnavailable), "must not be remapped to ErrUnavailable")
		})
	}
}
