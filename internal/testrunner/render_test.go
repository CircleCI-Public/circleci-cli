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
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func captureRenderCtx() (context.Context, *bytes.Buffer) {
	var errBuf bytes.Buffer
	ctx := iostream.WithStreams(context.Background(), iostream.Streams{
		Out: &bytes.Buffer{},
		Err: &errBuf,
		In:  strings.NewReader(""),
	})
	return ctx, &errBuf
}

func TestRender_Pass(t *testing.T) {
	ctx, errBuf := captureRenderCtx()

	Render(ctx, RunResult{Outcome: OutcomePass})

	assert.Check(t, golden.String(errBuf.String(), t.Name()+".txt"))
}

func TestRender_Fail(t *testing.T) {
	ctx, errBuf := captureRenderCtx()

	Render(ctx, RunResult{Outcome: OutcomeFail, ExitCode: 1})

	assert.Check(t, golden.String(errBuf.String(), t.Name()+".txt"))
}

func TestRender_Error(t *testing.T) {
	ctx, errBuf := captureRenderCtx()

	Render(ctx, RunResult{Outcome: OutcomeError, Err: errors.New("Docker is required")})

	assert.Check(t, golden.String(errBuf.String(), t.Name()+".txt"))
}

func TestRender_Skipped(t *testing.T) {
	ctx, errBuf := captureRenderCtx()

	Render(ctx, RunResult{Outcome: OutcomePass, Skipped: true})

	assert.Check(t, golden.String(errBuf.String(), t.Name()+".txt"))
}
