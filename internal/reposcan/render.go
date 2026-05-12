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
	"context"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// Render writes a human-readable summary of the scan Result to the streams in
// ctx. On an empty result it prints a friendly fallback so the caller can
// continue.
func Render(ctx context.Context, r *Result) {
	if r.IsEmpty() {
		iostream.ErrPrintf(ctx, "%s No supported stack detected. You can still continue.\n",
			iostream.SymbolWarn(ctx))
		return
	}

	iostream.ErrPrintf(ctx, "%s Detected %s project (%s)\n",
		iostream.SymbolOK(ctx), r.Stack, image(r))

	for _, step := range r.Setup {
		iostream.ErrPrintf(ctx, "    %s: %s\n", step.Name, step.Command)
	}
}

func image(r *Result) string {
	if r.ImageVersion == "" {
		return r.Image
	}
	return r.Image + ":" + r.ImageVersion
}
