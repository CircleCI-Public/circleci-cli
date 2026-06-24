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
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/zalando/go-keyring"
)

const timeout = 3 * time.Second

var (
	ErrNotFound = errors.New("password not found in keyring")

	// ErrUnavailable indicates the OS keyring backend cannot be reached (for
	// example a headless Linux/CI host with no D-Bus session bus). Callers
	// should treat secure storage as absent and fall back to the config file.
	ErrUnavailable = errors.New("keyring unavailable")

	// ErrAccessDenied is a specific ErrUnavailable: the session bus exists but
	// the sandbox or D-Bus policy refused the connection — most commonly a
	// strict snap whose password-manager-service interface is not connected.
	// Unlike a missing provider, this is something the user can fix, so callers
	// may surface a hint. It wraps ErrUnavailable so the file-storage fallback
	// still triggers.
	ErrAccessDenied = fmt.Errorf("%w: access to the secret service was denied", ErrUnavailable)
)

// Available reports whether the OS keyring backend is usable.
//
// On Unix the backend talks to the Secret Service over D-Bus. When no session
// bus is advertised, the underlying library shells out to "dbus-launch" to
// start one; on headless/CI machines that binary is absent and the call fails
// with a confusing `exec: "dbus-launch": executable file not found in $PATH`.
// We detect that situation up front so we never attempt the operation — and so
// the load path doesn't pay a doomed 3s timeout on every command.
//
// macOS (Keychain) and Windows (Credential Manager) have no such dependency and
// are always considered available.
func Available() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "netbsd", "openbsd", "dragonfly":
		// A reachable session bus is enough on its own.
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
			return true
		}
		// Otherwise the library can only reach the bus via dbus-launch.
		_, err := exec.LookPath("dbus-launch")
		return err == nil
	default:
		return true
	}
}

// classify maps "the Secret Service backend is absent or unreachable" failures
// to ErrUnavailable, so callers fall back to file storage instead of aborting
// the command.
//
// Available() only verifies that a session bus is advertised (or that
// dbus-launch exists) — not that a Secret Service provider (gnome-keyring,
// kwallet, KeePassXC, …) is actually registered on it. A minimal desktop, a
// fresh container, or a snap on a box without a password manager can have a
// live D-Bus session with no provider, in which case the backend fails with
//
//	org.freedesktop.DBus.Error.ServiceUnknown:
//	The name org.freedesktop.secrets was not provided by any .service files
//
// We also map raw connection failures (cannot dial the bus socket, or a missing
// dbus-launch the library tried to autostart). Genuine backend errors — a
// locked collection, an item not found — are NOT remapped and still surface to
// the caller.
func classify(err error) error {
	if err == nil {
		return nil
	}

	// A sandbox or D-Bus policy refused the connection — the user can fix this
	// (e.g. by connecting a snap interface), so flag it distinctly. At the
	// socket level this surfaces as EACCES on connect; at the D-Bus level as an
	// AccessDenied reply.
	var dbusErr dbus.Error
	if errors.As(err, &dbusErr) && dbusErr.Name == "org.freedesktop.DBus.Error.AccessDenied" {
		return ErrAccessDenied
	}
	if errors.Is(err, syscall.EACCES) {
		return ErrAccessDenied
	}

	// The session bus has no Secret Service provider, or one could not be
	// activated on demand.
	if errors.As(err, &dbusErr) {
		switch dbusErr.Name {
		case "org.freedesktop.DBus.Error.ServiceUnknown",
			"org.freedesktop.DBus.Error.NameHasNoOwner":
			return ErrUnavailable
		}
		if strings.HasPrefix(dbusErr.Name, "org.freedesktop.DBus.Error.Spawn.") {
			return ErrUnavailable
		}
	}

	// Could not even reach the bus: the socket connect was refused (no bus
	// running) or the autolaunch helper was missing.
	var opErr *net.OpError
	var execErr *exec.Error
	if errors.As(err, &opErr) || errors.As(err, &execErr) {
		return ErrUnavailable
	}

	return err
}

func Set(ctx context.Context, service, user, password string) error {
	if !Available() {
		return ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		ch <- keyring.Set(hostname(service), user, password)
	}()
	select {
	case err := <-ch:
		return classify(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func Get(ctx context.Context, service, user string) (password string, err error) {
	if !Available() {
		return "", ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type msg struct {
		password string
		err      error
	}

	ch := make(chan msg, 1)
	go func() {
		defer close(ch)
		password, err := keyring.Get(hostname(service), user)
		ch <- msg{password: password, err: err}
	}()
	select {
	case res := <-ch:
		if errors.Is(res.err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return res.password, classify(res.err)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func hostname(service string) string {
	return "com.circleci.cli:" + service
}

func Delete(ctx context.Context, service, user string) error {
	if !Available() {
		return ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		ch <- keyring.Delete(hostname(service), user)
	}()
	select {
	case err := <-ch:
		return classify(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}
