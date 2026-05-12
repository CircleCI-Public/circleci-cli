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

package reposcan

import (
	"testing"

	"github.com/CircleCI-Public/chunk-cli/envbuilder"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestResultFromEnvironment_MapsAllFields(t *testing.T) {
	env := &envbuilder.Environment{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []envbuilder.Step{
			{Name: "install", Command: "go mod download"},
			{Name: "test", Command: "go test ./..."},
		},
	}

	got := resultFromEnvironment(env)

	assert.Equal(t, got.Stack, "go")
	assert.Equal(t, got.Image, "cimg/go")
	assert.Equal(t, got.ImageVersion, "1.22")
	assert.DeepEqual(t, got.Setup, []SetupStep{
		{Name: "install", Command: "go mod download"},
		{Name: "test", Command: "go test ./..."},
	})
}

func TestResultFromEnvironment_UnknownStack_IsEmpty(t *testing.T) {
	env := &envbuilder.Environment{Stack: "unknown", Image: "unknown", ImageVersion: "unknown"}

	got := resultFromEnvironment(env)

	assert.Check(t, got.IsEmpty(), "result for unknown stack should be empty: %+v", got)
}

func TestResultFromEnvironment_NilSetup_IsHandled(t *testing.T) {
	env := &envbuilder.Environment{Stack: "ruby", Image: "cimg/ruby", ImageVersion: "3.2"}

	got := resultFromEnvironment(env)

	assert.Equal(t, got.Stack, "ruby")
	assert.Check(t, cmp.Len(got.Setup, 0))
}

func TestResultFromEnvironment_NilEnvironment_ReturnsNil(t *testing.T) {
	assert.Check(t, resultFromEnvironment(nil) == nil)
}

func TestNewDefaultScanner_ReturnsNonNil(t *testing.T) {
	assert.Check(t, NewDefaultScanner() != nil)
}
