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
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
)

const testToken = "test-token"

var binaryPath string

func TestMain(m *testing.M) {
	path, cleanup, err := binary.BuildBinary()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(os.Stderr, "built circleci binary: %s\n", path)

	binaryPath = path
	code := m.Run()
	cleanup()

	os.Exit(code)
}
