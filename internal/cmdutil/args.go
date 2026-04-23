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
	"fmt"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// RequireArgs returns a structured CLIError if args contains fewer elements
// than the number of names provided. Each name describes an expected positional
// argument (e.g. "workflow-id", "resource-class") and appears in the error
// message as <name>.
//
// Use alongside cobra.MaximumNArgs(N) so that too many args are still rejected
// by Cobra, while the missing-arg case produces a structured error from RunE.
func RequireArgs(args []string, names ...string) error {
	if len(args) >= len(names) {
		return nil
	}
	var missing []string
	for i := len(args); i < len(names); i++ {
		missing = append(missing, "<"+names[i]+">")
	}
	noun := "argument"
	if len(missing) > 1 {
		noun = "arguments"
	}
	return clierrors.New("args.missing", "Missing required argument",
		fmt.Sprintf("Required %s missing: %s", noun, strings.Join(missing, " "))).
		WithExitCode(clierrors.ExitBadArguments)
}
