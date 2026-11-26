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

// Package extsocket is the private Unix socket the CLI listens on before an
// extension starts. The extension reports telemetry to it, and the CLI reads
// one bounded JSON payload from it after the extension exits.
//
// On Unix the socket sits in a directory only the current user can open. On
// Windows the directory is in the per-user temporary directory. The package
// never logs or prints the socket path or the payload.
package extsocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// ErrPathTooLong means the socket path would not fit a Unix socket address.
	ErrPathTooLong = errors.New("socket path is too long")
	// ErrNoConnection means no client connected before the deadline.
	ErrNoConnection = errors.New("no connection to the telemetry socket")
	// ErrNoPayload means a client connected and closed without sending data.
	ErrNoPayload = errors.New("telemetry connection sent no data")
	// ErrUnreadable means the connection failed, timed out, or did not carry
	// a single well-formed JSON object no longer than maxPayload.
	ErrUnreadable = errors.New("telemetry payload is unreadable")
)

const (
	maxPayload    = 8 << 10
	maxSocketPath = 100 // under 104 on macOS and 108 elsewhere
	staleAge      = 24 * time.Hour
	dirPrefix     = "circleci-extsock-"
	sockName      = "a.sock"
)

var (
	// collectDeadline bounds the whole of Collect. A variable so tests can
	// shorten it.
	collectDeadline = time.Second
	// acceptWait is how long Collect waits for a queued connection. A client
	// that connected before the extension exited is queued already, so a longer
	// wait only delays a run where the extension never connected.
	acceptWait = 50 * time.Millisecond
)

// Payload is what an extension sent. The package judges nothing about the
// version or the attributes: the caller does.
type Payload struct {
	// Version is the payload version, or 0 when the payload had none.
	Version int
	// Attributes holds the raw JSON value of each attribute, or nil when the
	// payload had no attributes key.
	Attributes map[string]json.RawMessage
}

// Listener is a listening socket in a private directory.
type Listener struct {
	dir  string
	ln   *net.UnixListener
	once sync.Once
}

// Listen removes stale directories left by crashed runs, creates a private
// directory and listens on a socket in it. Close the Listener to remove the
// directory.
func Listen() (*Listener, error) {
	cleanStale(os.TempDir(), time.Now(), os.Getuid(), processAlive)

	dir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("%s%d-", dirPrefix, os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("creating telemetry socket directory: %w", withoutPath(err))
	}
	path := filepath.Join(dir, sockName)
	if len(path) >= maxSocketPath {
		_ = os.RemoveAll(dir)
		return nil, ErrPathTooLong
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("listening on telemetry socket: %w", withoutPath(err))
	}
	return &Listener{dir: dir, ln: ln}, nil
}

// withoutPath unwraps the errors the os and net packages return, which carry
// the path in their message, down to the underlying error.
func withoutPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Err != nil {
		var se *os.SyscallError
		if errors.As(oe.Err, &se) {
			return se.Err
		}
		return oe.Err
	}
	return err
}

// Path returns the socket path.
func (l *Listener) Path() string {
	return filepath.Join(l.dir, sockName)
}

// Close closes the listener and removes the directory. It is safe to call twice.
func (l *Listener) Close() error {
	l.once.Do(func() {
		_ = l.ln.Close() // already closed after Collect
		_ = os.RemoveAll(l.dir)
	})
	return nil
}
