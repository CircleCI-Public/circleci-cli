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

// Package telemetry sends an extension's typed telemetry attributes to the
// CircleCI CLI that started it.
//
// The extension calls [FromEnv] once, early in main. It then calls the Set*
// methods as it learns things, and calls [Telemetry.Send] once at its
// top-level return, before os.Exit. Telemetry is best effort: nothing in this
// package returns an error, and it never changes the extension's exit code.
package telemetry

import (
	"encoding/json"
	"net"
	"os"
	"regexp"
	"slices"
	"sync"
	"time"
)

const (
	envSocket      = "CIRCLE_TELEMETRY_SOCKET"
	version        = 1
	maxAttributes  = 20
	maxPayloadSize = 8 * 1024
	timeout        = time.Second
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

var (
	once   sync.Once
	shared *Telemetry
)

// Telemetry collects an extension's typed attributes and sends them to the CLI.
// A nil *Telemetry, or one made while the variable was unset, does nothing.
type Telemetry struct {
	socket string
	mu     sync.Mutex
	attrs  map[string]any
	sent   bool
}

type payload struct {
	Version    int            `json:"version"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// FromEnv reads CIRCLE_TELEMETRY_SOCKET and removes it from the process
// environment, so child processes the extension starts do not inherit it.
// Every call returns the same *Telemetry; call it once, early in main.
func FromEnv() *Telemetry {
	once.Do(func() {
		socket := os.Getenv(envSocket)
		_ = os.Unsetenv(envSocket)
		shared = &Telemetry{socket: socket, attrs: map[string]any{}}
	})
	return shared
}

// SetInt records an integer attribute. A later call with the same name
// replaces the earlier value.
func (t *Telemetry) SetInt(name string, v int) { t.set(name, v) }

// SetBool records a boolean attribute. A later call with the same name
// replaces the earlier value.
func (t *Telemetry) SetBool(name string, v bool) { t.set(name, v) }

// SetEnum records v under name when v is one of allowed. Any other v, or an
// empty allowed list, is ignored. A later call with the same name replaces the
// earlier value.
func (t *Telemetry) SetEnum(name, v string, allowed ...string) {
	if !slices.Contains(allowed, v) {
		return
	}
	t.set(name, v)
}

func (t *Telemetry) set(name string, v any) {
	if t == nil || t.socket == "" || !namePattern.MatchString(name) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sent {
		return
	}
	if _, exists := t.attrs[name]; !exists && len(t.attrs) >= maxAttributes {
		return
	}
	t.attrs[name] = v
}

// Send writes the collected attributes to the CLI in one JSON object. A second
// call, and any Set call after the first, does nothing. Send does not block Set
// calls from other goroutines while it writes to the socket, and it returns
// without waiting when nobody listens.
func (t *Telemetry) Send() {
	if t == nil || t.socket == "" {
		return
	}
	t.mu.Lock()
	if t.sent {
		t.mu.Unlock()
		return
	}
	t.sent = true
	b, err := json.Marshal(payload{Version: version, Attributes: t.attrs})
	if err != nil || len(b) > maxPayloadSize {
		b, _ = json.Marshal(payload{Version: version})
	}
	t.mu.Unlock()
	conn, err := net.DialTimeout("unix", t.socket, timeout)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_, _ = conn.Write(b)
}
