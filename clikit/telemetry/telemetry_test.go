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

package telemetry_test

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/telemetry"
)

// listen starts a Unix socket listener in a short directory (t.TempDir()
// can exceed the 104-byte macOS socket path limit) and returns the socket
// path and a function that returns every payload sent so far, in order.
func listen(t *testing.T) (string, func() []string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ck")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "t.sock")
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	assert.NilError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return path, func() []string {
		var got []string
		for {
			// Send is synchronous, so a connection it made is already queued.
			_ = l.SetDeadline(time.Now().Add(200 * time.Millisecond))
			c, err := l.Accept()
			if err != nil {
				return got
			}
			_ = c.SetReadDeadline(time.Now().Add(time.Second))
			b, _ := io.ReadAll(c)
			_ = c.Close()
			got = append(got, string(b))
		}
	}
}

// names returns n attribute names a01, a02, ... in order.
func names(n int) []string {
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("a%02d", i))
	}
	return out
}

// attrsJSON renders the payload for names a01..aN each holding value v(i).
func attrsJSON(n int, v func(int) int) string {
	parts := make([]string, 0, n)
	for i, name := range names(n) {
		parts = append(parts, fmt.Sprintf("%q:%d", name, v(i+1)))
	}
	return `{"version":1,"attributes":{` + strings.Join(parts, ",") + `}}`
}

func TestSend(t *testing.T) {
	long := strings.Repeat("a", 32)

	tests := []struct {
		name  string
		env   string
		calls func(*telemetry.Telemetry)
		want  []string
	}{
		{
			name: "worked example",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetInt("suggestions", 3)
				tel.SetEnum("no_suggestion_reason", "saving_too_small", "saving_too_small", "no_data")
				tel.Send()
			},
			want: []string{`{"version":1,"attributes":{"no_suggestion_reason":"saving_too_small","suggestions":3}}`},
		},
		{
			name:  "bool value",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) { tel.SetBool("cache_hit", true) },
			want:  []string{`{"version":1,"attributes":{"cache_hit":true}}`},
		},
		{
			name:  "no attributes",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) {},
			want:  []string{`{"version":1}`},
		},
		{
			name: "same name replaced",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetInt("n", 1)
				tel.SetInt("n", 2)
			},
			want: []string{`{"version":1,"attributes":{"n":2}}`},
		},
		{
			name: "same name, other type",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetInt("n", 1)
				tel.SetBool("n", true)
			},
			want: []string{`{"version":1,"attributes":{"n":true}}`},
		},
		{
			name: "invalid names ignored",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetInt("", 1)
				tel.SetInt("Bad", 1)
				tel.SetInt("1a", 1)
				tel.SetInt("a-b", 1)
				tel.SetInt(strings.Repeat("a", 33), 1)
				tel.SetInt("ok", 1)
			},
			want: []string{`{"version":1,"attributes":{"ok":1}}`},
		},
		{
			name:  "32-char name accepted",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) { tel.SetInt(long, 1) },
			want:  []string{`{"version":1,"attributes":{"` + long + `":1}}`},
		},
		{
			name: "21 attributes",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				for i, name := range names(21) {
					tel.SetInt(name, i+1)
				}
			},
			want: []string{attrsJSON(20, func(i int) int { return i })},
		},
		{
			name: "replace at the cap",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				for _, name := range names(20) {
					tel.SetInt(name, 1)
				}
				tel.SetInt("a01", 9)
				tel.SetInt("a21", 1)
			},
			want: []string{attrsJSON(20, func(i int) int {
				if i == 1 {
					return 9
				}
				return 1
			})},
		},
		{
			name: "Set after Send",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetInt("a", 1)
				tel.Send()
				tel.SetInt("b", 2)
				tel.Send()
			},
			want: []string{`{"version":1,"attributes":{"a":1}}`},
		},
		{
			name: "oversize payload",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				big := strings.Repeat("x", 1000)
				for _, name := range names(20) {
					tel.SetEnum(name, big, big)
				}
			},
			want: []string{`{"version":1}`},
		},
		{
			name:  "enum value allowed",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) { tel.SetEnum("r", "b", "a", "b") },
			want:  []string{`{"version":1,"attributes":{"r":"b"}}`},
		},
		{
			name:  "enum value not allowed",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) { tel.SetEnum("r", "c", "a", "b") },
			want:  []string{`{"version":1}`},
		},
		{
			name:  "enum with no allowed values",
			env:   "socket",
			calls: func(tel *telemetry.Telemetry) { tel.SetEnum("r", "a") },
			want:  []string{`{"version":1}`},
		},
		{
			name: "enum not allowed keeps earlier value",
			env:  "socket",
			calls: func(tel *telemetry.Telemetry) {
				tel.SetEnum("r", "a", "a")
				tel.SetEnum("r", "z", "a")
			},
			want: []string{`{"version":1,"attributes":{"r":"a"}}`},
		},
		{
			name: "nil receiver",
			env:  "unset",
			calls: func(*telemetry.Telemetry) {
				var tel *telemetry.Telemetry
				tel.SetInt("a", 1)
				tel.SetBool("b", true)
				tel.SetEnum("c", "x", "x")
				tel.Send()
			},
			want: nil,
		},
		{
			name:  "unset variable",
			env:   "unset",
			calls: func(tel *telemetry.Telemetry) { tel.SetInt("a", 1) },
			want:  nil,
		},
		{
			name:  "empty variable",
			env:   "empty",
			calls: func(tel *telemetry.Telemetry) { tel.SetInt("a", 1) },
			want:  nil,
		},
		{
			name:  "nobody listening",
			env:   "stale",
			calls: func(tel *telemetry.Telemetry) { tel.SetInt("a", 1) },
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			telemetry.Reset()
			path, received := listen(t)
			switch tt.env {
			case "socket":
				t.Setenv("CIRCLE_TELEMETRY_SOCKET", path)
			case "empty":
				t.Setenv("CIRCLE_TELEMETRY_SOCKET", "")
			case "stale":
				dir, err := os.MkdirTemp("", "ck")
				assert.NilError(t, err)
				t.Cleanup(func() { _ = os.RemoveAll(dir) })
				t.Setenv("CIRCLE_TELEMETRY_SOCKET", filepath.Join(dir, "gone.sock"))
			case "unset":
				if old, ok := os.LookupEnv("CIRCLE_TELEMETRY_SOCKET"); ok {
					t.Cleanup(func() { _ = os.Setenv("CIRCLE_TELEMETRY_SOCKET", old) })
				}
				_ = os.Unsetenv("CIRCLE_TELEMETRY_SOCKET")
			}

			tel := telemetry.FromEnv()
			tt.calls(tel)
			start := time.Now()
			tel.Send()
			assert.Check(t, time.Since(start) < 3*time.Second, "Send must not hang")

			got := received()
			assert.Check(t, cmp.DeepEqual(got, tt.want))
		})
	}
}

func TestFromEnv(t *testing.T) {
	t.Run("removes the variable", func(t *testing.T) {
		telemetry.Reset()
		path, _ := listen(t)
		t.Setenv("CIRCLE_TELEMETRY_SOCKET", path)
		telemetry.FromEnv()
		_, ok := os.LookupEnv("CIRCLE_TELEMETRY_SOCKET")
		assert.Check(t, !ok, "variable must be gone so child processes do not inherit it")
	})
	t.Run("second call returns the same client", func(t *testing.T) {
		telemetry.Reset()
		path, received := listen(t)
		t.Setenv("CIRCLE_TELEMETRY_SOCKET", path)
		first := telemetry.FromEnv()
		second := telemetry.FromEnv()
		assert.Check(t, cmp.Equal(first, second))
		second.SetInt("a", 1)
		first.Send()
		got := received()
		assert.Check(t, cmp.DeepEqual(got, []string{`{"version":1,"attributes":{"a":1}}`}))
	})
	t.Run("concurrent Set", func(t *testing.T) {
		telemetry.Reset()
		path, received := listen(t)
		t.Setenv("CIRCLE_TELEMETRY_SOCKET", path)
		tel := telemetry.FromEnv()
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); tel.SetInt("n", i) }()
		}
		wg.Wait()
		tel.Send()
		got := received()
		assert.Check(t, cmp.Len(got, 1))
	})
}
