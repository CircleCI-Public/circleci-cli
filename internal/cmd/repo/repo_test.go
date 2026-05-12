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

package repo

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewRepoCmd_Registration(t *testing.T) {
	cmd := NewRepoCmd()
	assert.Equal(t, cmd.Name(), "repo")
	assert.Check(t, cmd.Short != "", "repo command must have a Short description")
	assert.Check(t, cmd.Long != "", "repo command must have a Long description")
}

func TestNewRepoCmd_HasScanSubcommand(t *testing.T) {
	cmd := NewRepoCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "scan" {
			found = true
			break
		}
	}
	assert.Check(t, found, "repo group should have a 'scan' subcommand")
}
