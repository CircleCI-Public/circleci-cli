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

package cmdutil

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

// ReadFlagValue resolves a flag that takes a document inline or by reference:
// @- reads stdin, @path reads the file, and anything else is
// returned as given. A read failure is returned as the bare io/os error.
func ReadFlagValue(ctx context.Context, v string) ([]byte, error) {
	switch {
	case v == "@-":
		return io.ReadAll(iostream.In(ctx))
	case strings.HasPrefix(v, "@"):
		return os.ReadFile(v[1:]) //#nosec:G304 // the path is the user's own flag value, not external input
	default:
		return []byte(v), nil
	}
}
