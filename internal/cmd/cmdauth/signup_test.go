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

package cmdauth

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNewSignupCmd_Metadata(t *testing.T) {
	cmd := newSignupCmd()

	assert.Check(t, is.Equal(cmd.Use, "signup"))
	assert.Check(t, cmd.Short != "", "Short must be set")
	assert.Check(t, cmd.Long != "", "Long must be set")
	assert.Check(t, cmd.Example != "", "Example must be set")
	assert.Check(t, cmd.RunE != nil, "RunE must be wired")

	// Per CLAUDE.md, every command needs at least 3 examples.
	// Each example begins with a shell prompt ("$ ") on a fresh line.
	exampleCount := strings.Count(cmd.Example, "\n$ ")
	assert.Check(t, exampleCount >= 3,
		"expected ≥3 examples, got %d in:\n%s", exampleCount, cmd.Example)

	// Args == cobra.NoArgs — pass an unexpected positional arg and expect error.
	err := cmd.Args(cmd, []string{"unexpected-positional"})
	assert.Check(t, err != nil, "signup must reject positional args")
}
