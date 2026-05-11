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

package cmdinit

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestNewInitCmd_Registration(t *testing.T) {
	cmd := NewInitCmd()
	assert.Equal(t, cmd.Use, "init")
	assert.Check(t, cmd.RunE != nil, "init command must have a RunE")
	assert.Check(t, cmd.Short != "", "init command must have a Short description")
	assert.Check(t, cmd.Long != "", "init command must have a Long description")
	assert.Check(t, cmd.Example != "", "init command must have examples")
}

func TestNewInitCmd_HelpNamesAllFourSteps(t *testing.T) {
	cmd := NewInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	assert.NilError(t, cmd.Execute())

	help := buf.String()
	for _, phase := range []string{
		"Scan your repo",
		"Docker container",
		"Generate a config",
		"Sign up for CircleCI",
	} {
		assert.Check(t, cmp.Contains(help, phase),
			"--help output is missing onboarding phase %q", phase)
	}
}
