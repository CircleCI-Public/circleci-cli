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

package open

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestProjectURL(t *testing.T) {
	t.Run("github project", func(t *testing.T) {
		got, err := ProjectURL("gh/bar/foo")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "https://app.circleci.com/pipelines/gh/bar/foo"))
	})

	t.Run("bitbucket project", func(t *testing.T) {
		got, err := ProjectURL("bb/myorg/myrepo")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "https://app.circleci.com/pipelines/bb/myorg/myrepo"))
	})

	t.Run("gitlab project", func(t *testing.T) {
		got, err := ProjectURL("gl/my-group/my-project")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "https://app.circleci.com/pipelines/gl/my-group/my-project"))
	})

	t.Run("invalid slug", func(t *testing.T) {
		_, err := ProjectURL("invalid")
		assert.Check(t, err != nil, "expected error for invalid slug")
	})
}
