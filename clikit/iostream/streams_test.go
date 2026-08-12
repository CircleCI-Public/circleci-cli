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

package iostream

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// clearTerminalEnv resets every environment variable that influences terminal
// detection so a test starts from a known-interactive baseline regardless of
// where it runs (notably, CI sets CI=true on the very machine running these).
func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CI", "CIRCLECI", "CIRCLE_NO_INTERACTIVE", "NO_COLOR", "CIRCLE_NO_COLOR"} {
		t.Setenv(k, "")
	}
	t.Setenv("TERM", "xterm-256color")
}

func TestColorDisabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "interactive defaults", env: nil, want: false},
		{name: "NO_COLOR", env: map[string]string{"NO_COLOR": "1"}, want: true},
		{name: "CIRCLE_NO_COLOR", env: map[string]string{"CIRCLE_NO_COLOR": "1"}, want: true},
		{name: "TERM=dumb outside CircleCI", env: map[string]string{"TERM": "dumb"}, want: true},
		// CircleCI sets TERM=dumb but renders ANSI, so color stays on there.
		{name: "TERM=dumb in CircleCI", env: map[string]string{"TERM": "dumb", "CIRCLECI": "true"}, want: false},
		// An explicit opt-out still wins, even in CircleCI.
		{name: "NO_COLOR wins in CircleCI", env: map[string]string{"TERM": "dumb", "CIRCLECI": "true", "NO_COLOR": "1"}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearTerminalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			assert.Check(t, cmp.Equal(colorDisabled(), tc.want))
		})
	}
}

func TestInteractiveEnvDisabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "no env", env: nil, want: false},
		{name: "CI set", env: map[string]string{"CI": "true"}, want: true},
		{name: "CIRCLE_NO_INTERACTIVE set", env: map[string]string{"CIRCLE_NO_INTERACTIVE": "1"}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearTerminalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			assert.Check(t, cmp.Equal(interactiveEnvDisabled(), tc.want))
		})
	}
}

func TestBackgroundQueryableNonTerminal(t *testing.T) {
	clearTerminalEnv(t)

	// A pipe is not a terminal; querying it would be pointless, so it must be
	// reported non-queryable even in an otherwise interactive environment.
	pr, pw, err := os.Pipe()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	assert.Check(t, !backgroundQueryable(pr, pw))
}
