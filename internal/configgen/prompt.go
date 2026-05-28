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

package configgen

import (
	"fmt"
	"strings"
)

const writeFailedPromptTemplate = "Writing the generated `.circleci/config.yml` failed.\n" +
	"\n" +
	"Project stack: %s\n" +
	"Image: %s\n" +
	"Target path: %s\n" +
	"Error: %s\n" +
	"\n" +
	"Please help me diagnose and fix this. Common causes:\n" +
	"  • Write permissions on the `.circleci/` directory or its parent\n" +
	"  • Low disk space\n" +
	"  • Read-only filesystem\n" +
	"\n" +
	"Once fixed, I will re-run the command to continue.\n"

// RenderWriteFailedPrompt returns the POC prompt shown when writing the
// generated config to disk fails. The user is expected to paste this into an
// AI assistant to diagnose the filesystem-level cause.
func RenderWriteFailedPrompt(stack, image, targetPath, errMsg string) string {
	if stack == "" || stack == "unknown" {
		stack = "(could not detect)"
	}
	if image == "" || strings.Contains(image, "unknown") {
		image = "(none — stack not detected)"
	}
	return fmt.Sprintf(writeFailedPromptTemplate, stack, image, targetPath, errMsg)
}
