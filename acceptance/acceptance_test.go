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

package acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testToken = "test-token"

// ignoreCommonHeaders excludes headers that are set uniformly by the API client
// (and therefore uninteresting to a per-command mutation assertion) so that
// request comparisons can focus on Method, URL, Authorization, User-Agent, and
// the request body.
var ignoreCommonHeaders = httprecorder.IgnoreHeaders(
	"Accept",
	"Accept-Encoding",
	"Content-Length",
	"Content-Type",
)

var (
	binaryPath     string
	testBinaryPath string
	testBinaryDir  string
)

func TestMain(m *testing.M) {
	var err error
	var cleanup, cleanup2 func()
	binaryPath, cleanup, err = binary.BuildBinary()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(os.Stderr, "built circleci binary: %s\n", binaryPath)

	testBinaryPath, cleanup2, err = binary.BuildBinaryOptions("circleci-testextension", filepath.Join("testdata", "circleci-testextension"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(os.Stderr, "built testapp binary: %s\n", testBinaryPath)
	testBinaryDir = filepath.Dir(testBinaryPath)

	code := m.Run()
	cleanup()
	cleanup2()

	os.Exit(code)
}
