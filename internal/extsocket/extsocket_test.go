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

package extsocket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// shortDeadline sets the Collect deadline for one test and restores it.
func shortDeadline(t *testing.T, d time.Duration) {
	t.Helper()
	old := collectDeadline
	collectDeadline = d
	t.Cleanup(func() { collectDeadline = old })
}

// setTempDir points os.TempDir at dir: TMPDIR on Unix, TMP on Windows.
func setTempDir(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("TMP", dir)
		return
	}
	t.Setenv("TMPDIR", dir)
}

// shortTempDir returns a new directory with a short path. t.TempDir is too
// long on macOS for a socket path to fit under maxSocketPath.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "t")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// newListener listens in a short temporary directory that holds nothing else.
func newListener(t *testing.T) *Listener {
	t.Helper()
	setTempDir(t, shortTempDir(t))
	l, err := Listen()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return l
}

// send dials the socket and writes b in a goroutine, so a write that fills
// the socket buffer cannot block the test. closeAfter=false keeps the
// connection open until the test ends.
func send(t *testing.T, path string, b []byte, closeAfter bool) {
	t.Helper()
	conn, err := net.Dial("unix", path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		// The server may stop reading early (oversized payload), so a
		// failed write is not an error here.
		_, _ = conn.Write(b)
		if closeAfter {
			_ = conn.Close()
		}
	}()
}

// connect dials the socket and writes nothing.
func connect(t *testing.T, path string, closeAfter bool) {
	t.Helper()
	conn, err := net.Dial("unix", path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	if closeAfter {
		_ = conn.Close()
	}
}

// sized returns a valid payload of exactly n bytes, padding one attribute.
func sized(n int) []byte {
	const prefix = `{"version":1,"attributes":{"a":"`
	const suffix = `"}}`
	return []byte(prefix + strings.Repeat("x", n-len(prefix)-len(suffix)) + suffix)
}

func writes(s string, closeAfter bool) func(*testing.T, string) {
	return func(t *testing.T, path string) { send(t, path, []byte(s), closeAfter) }
}

func TestCollect(t *testing.T) {
	const short = 100 * time.Millisecond

	tests := []struct {
		name     string
		client   func(t *testing.T, path string)
		deadline time.Duration
		want     Payload
		wantErr  error
		leak     string // text the client wrote that the error must not contain
	}{
		{
			name:   "valid",
			client: writes(`{"version":1,"attributes":{"a":"x","n":2}}`, true),
			want: Payload{Version: 1, Attributes: map[string]json.RawMessage{
				"a": json.RawMessage(`"x"`),
				"n": json.RawMessage(`2`),
			}},
		},
		{
			name:   "missing version",
			client: writes(`{"attributes":{"a":1}}`, true),
			want: Payload{Attributes: map[string]json.RawMessage{
				"a": json.RawMessage(`1`),
			}},
		},
		{
			name:    "half payload",
			client:  writes(`{"version":1,"attrib`, true),
			wantErr: ErrUnreadable,
			leak:    "attrib",
		},
		{
			name:    "invalid JSON",
			client:  writes(`not json`, true),
			wantErr: ErrUnreadable,
			leak:    "not json",
		},
		{
			name:    "text after object",
			client:  writes(`{"version":1}x`, true),
			wantErr: ErrUnreadable,
		},
		{
			name:    "attributes not an object",
			client:  writes(`{"version":1,"attributes":[1]}`, true),
			wantErr: ErrUnreadable,
		},
		{
			name:    "9 KB",
			client:  writes(`{"version":1,"attributes":{"a":"`+strings.Repeat("x", 9000)+`"}}`, true),
			wantErr: ErrUnreadable,
			leak:    "xxxxxxxx",
		},
		{
			name:    "duplicate top-level key",
			client:  writes(`{"version":1,"version":2}`, true),
			wantErr: ErrUnreadable,
			leak:    `"version"`,
		},
		{
			name:    "duplicate attribute key",
			client:  writes(`{"version":1,"attributes":{"a":1,"a":2}}`, true),
			wantErr: ErrUnreadable,
			leak:    `"a"`,
		},
		{
			name:     "no client",
			deadline: short,
			wantErr:  ErrNoConnection,
		},
		{
			name:    "opens and closes",
			client:  func(t *testing.T, path string) { connect(t, path, true) },
			wantErr: ErrNoPayload,
		},
		{
			name:     "opens and stays silent",
			client:   func(t *testing.T, path string) { connect(t, path, false) },
			deadline: short,
			wantErr:  ErrUnreadable,
		},
		{
			name:   "exactly 8 KB",
			client: func(t *testing.T, path string) { send(t, path, sized(8192), true) },
			want: Payload{Version: 1, Attributes: map[string]json.RawMessage{
				"a": json.RawMessage(`"` + strings.Repeat("x", 8192-len(`{"version":1,"attributes":{"a":"`)-len(`"}}`)) + `"`),
			}},
		},
		{
			name:    "8 KB + 1",
			client:  func(t *testing.T, path string) { send(t, path, sized(8193), true) },
			wantErr: ErrUnreadable,
		},
		{
			name:    "version not whole",
			client:  writes(`{"version":1.5}`, true),
			wantErr: ErrUnreadable,
		},
		{
			name:    "version a string",
			client:  writes(`{"version":"1"}`, true),
			wantErr: ErrUnreadable,
		},
		{
			name:   "unknown top-level key",
			client: writes(`{"version":1,"extra":{"x":1}}`, true),
			want:   Payload{Version: 1},
		},
		{
			name:   "no attributes key",
			client: writes(`{"version":1}`, true),
			want:   Payload{Version: 1},
		},
		{
			name:     "partial then silent",
			client:   writes(`{"version":1`, false),
			deadline: short,
			wantErr:  ErrUnreadable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.deadline != 0 {
				shortDeadline(t, tc.deadline)
			}
			l := newListener(t)
			if tc.client != nil {
				tc.client(t, l.Path())
			}

			got, err := l.Collect(context.Background())

			if tc.wantErr != nil {
				assert.Check(t, errors.Is(err, tc.wantErr), "got error %v", err)
				if tc.leak != "" && err != nil {
					assert.Check(t, !strings.Contains(err.Error(), tc.leak), "error leaks payload: %v", err)
				}
				return
			}
			assert.Check(t, cmp.ErrorIs(err, nil))
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestCollectFirstConnectionOnly(t *testing.T) {
	l := newListener(t)
	send(t, l.Path(), []byte(`{"version":1}`), true)
	send(t, l.Path(), []byte(`{"version":2}`), true)

	got, err := l.Collect(context.Background())
	assert.Check(t, cmp.ErrorIs(err, nil))
	assert.Check(t, cmp.Equal(got.Version, 1))

	t.Run("second Collect", func(t *testing.T) {
		_, err := l.Collect(context.Background())
		assert.Check(t, cmp.ErrorIs(err, ErrNoConnection))
	})
	t.Run("later client is refused", func(t *testing.T) {
		conn, err := net.Dial("unix", l.Path())
		if err == nil {
			_ = conn.Close()
		}
		assert.Check(t, err != nil, "a client connected after Collect")
	})
}

func TestCollectCancelledContext(t *testing.T) {
	shortDeadline(t, 5*time.Second)
	l := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := l.Collect(ctx)

	assert.Check(t, err != nil)
	prompt := time.Since(start) < time.Second
	assert.Check(t, prompt, "Collect did not return promptly after cancel")
}

func TestCollectCancelledDuringRead(t *testing.T) {
	shortDeadline(t, 5*time.Second)
	l := newListener(t)
	connect(t, l.Path(), false)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := l.Collect(ctx)

	assert.Check(t, cmp.ErrorIs(err, ErrUnreadable))
	prompt := time.Since(start) < time.Second
	assert.Check(t, prompt, "Collect did not return promptly after cancel")
}

func TestListenPrivateDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the directory mode is not checked on Windows")
	}
	l := newListener(t)

	info, err := os.Stat(filepath.Dir(l.Path()))
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(info.Mode().Perm(), os.FileMode(0o700)))
	assert.Check(t, cmp.Regexp(`^cci[0-9]+$`, filepath.Base(filepath.Dir(l.Path()))))
	assert.Check(t, cmp.Equal(filepath.Base(l.Path()), "a.sock"))
}

func TestClose(t *testing.T) {
	l := newListener(t)
	dir := filepath.Dir(l.Path())

	err := l.Close()
	assert.Check(t, cmp.ErrorIs(err, nil))
	_, err = os.Stat(dir)
	assert.Check(t, os.IsNotExist(err), "directory still exists after Close")

	t.Run("second Close", func(t *testing.T) {
		err := l.Close()
		assert.Check(t, cmp.ErrorIs(err, nil))
	})
}

func TestListenPathTooLong(t *testing.T) {
	base := filepath.Join(shortTempDir(t), strings.Repeat("d", 120))
	err := os.Mkdir(base, 0o700)
	assert.NilError(t, err)
	setTempDir(t, base)

	l, err := Listen()

	assert.Check(t, l == nil)
	assert.Check(t, cmp.ErrorIs(err, ErrPathTooLong))
	entries, err := os.ReadDir(base)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(entries, 0))
}

func TestListenOtherFailure(t *testing.T) {
	missing := filepath.Join(shortTempDir(t), "does-not-exist")
	setTempDir(t, missing)

	l, err := Listen()

	assert.Check(t, l == nil)
	assert.Assert(t, err != nil)
	assert.Check(t, !errors.Is(err, ErrPathTooLong))
	assert.Check(t, !strings.Contains(err.Error(), missing), "error contains the path: %v", err)
}

func TestCleanStale(t *testing.T) {
	now := time.Now()
	mkdir := func(t *testing.T, base, name string, age time.Duration, files ...string) string {
		t.Helper()
		dir := filepath.Join(base, name)
		err := os.Mkdir(dir, 0o700)
		assert.NilError(t, err)
		for _, f := range files {
			err := os.WriteFile(filepath.Join(dir, f), nil, 0o600)
			assert.NilError(t, err)
		}
		mod := now.Add(-age)
		err = os.Chtimes(dir, mod, mod)
		assert.NilError(t, err)
		return dir
	}
	const old = 48 * time.Hour

	tests := []struct {
		name        string
		setup       func(t *testing.T, base string) (path string)
		wantRemoved bool
	}{
		{
			name: "old and valid",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cci123", old, "a.sock")
			},
			wantRemoved: true,
		},
		{
			name: "23 hours old",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cci123", 23*time.Hour, "a.sock")
			},
		},
		{
			name: "name without cci prefix",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "other-123", old, "a.sock")
			},
		},
		{
			name: "name without digits",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cciabc", old, "a.sock")
			},
		},
		{
			name: "symlink to an old valid directory",
			setup: func(t *testing.T, base string) string {
				target := mkdir(t, base, "target", old, "a.sock")
				link := filepath.Join(base, "cci123")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("cannot create a symlink: %v", err)
				}
				return link
			},
		},
		{
			name: "extra file",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cci123", old, "a.sock", "extra.txt")
			},
		},
		{
			name: "old and empty",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cci123", old)
			},
			wantRemoved: true,
		},
		{
			name: "regular file",
			setup: func(t *testing.T, base string) string {
				path := filepath.Join(base, "cci123")
				err := os.WriteFile(path, nil, 0o600)
				assert.NilError(t, err)
				mod := now.Add(-old)
				err = os.Chtimes(path, mod, mod)
				assert.NilError(t, err)
				return path
			},
		},
		{
			name: "only a.sock as a regular file",
			setup: func(t *testing.T, base string) string {
				return mkdir(t, base, "cci123", old, "a.sock")
			},
			wantRemoved: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := shortTempDir(t)
			path := tc.setup(t, base)

			cleanStale(base, now, os.Getuid())

			_, err := os.Lstat(path)
			if tc.wantRemoved {
				assert.Check(t, os.IsNotExist(err), "%s was not removed", filepath.Base(path))
			} else {
				assert.Check(t, cmp.ErrorIs(err, nil), "%s was removed", filepath.Base(path))
			}
			if filepath.Base(path) == "cci123" && !tc.wantRemoved {
				// A symlink's target must survive too.
				if target, err := os.Readlink(path); err == nil {
					_, err := os.Lstat(target)
					assert.Check(t, cmp.ErrorIs(err, nil), "symlink target was removed")
				}
			}
		})
	}
}

func TestOwnedByUser(t *testing.T) {
	info, err := os.Stat(shortTempDir(t))
	assert.NilError(t, err)
	uid := os.Getuid()

	if runtime.GOOS == "windows" {
		assert.Check(t, ownedByUser(info, uid))
		assert.Check(t, ownedByUser(info, uid+1))
		return
	}
	assert.Check(t, ownedByUser(info, uid))
	assert.Check(t, !ownedByUser(info, uid+1))
}

func TestListenCleansStale(t *testing.T) {
	base := shortTempDir(t)
	stale := filepath.Join(base, "cci42")
	err := os.Mkdir(stale, 0o700)
	assert.NilError(t, err)
	mod := time.Now().Add(-48 * time.Hour)
	err = os.Chtimes(stale, mod, mod)
	assert.NilError(t, err)
	setTempDir(t, base)

	l, err := Listen()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	_, err = os.Lstat(stale)
	assert.Check(t, os.IsNotExist(err), "stale directory was not removed")
	_, err = os.Stat(filepath.Dir(l.Path()))
	assert.Check(t, cmp.ErrorIs(err, nil))
}
