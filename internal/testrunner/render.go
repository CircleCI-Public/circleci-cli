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

package testrunner

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// Render writes the human-readable status for a test run.
func Render(ctx context.Context, r RunResult) {
	switch r.Outcome {
	case OutcomePass:
		if r.Skipped {
			iostream.ErrPrintf(ctx, "%s No test command detected. Continuing.\n", iostream.SymbolWarn(ctx))
			return
		}
		iostream.ErrPrintf(ctx, "%s Tests passed.\n", iostream.SymbolOK(ctx))
	case OutcomeFail:
		iostream.ErrPrintf(ctx, "%s Tests failed with exit code %d.\n", iostream.SymbolFail(ctx), r.ExitCode)
	case OutcomeError:
		if r.Err != nil {
			iostream.ErrPrintf(ctx, "%s Could not run tests: %v\n", iostream.SymbolFail(ctx), r.Err)
			return
		}
		iostream.ErrPrintf(ctx, "%s Could not run tests.\n", iostream.SymbolFail(ctx))
	}
}
