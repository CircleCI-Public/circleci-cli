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
	"bytes"
	"context"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func captureCtx() (context.Context, *bytes.Buffer) {
	var outBuf bytes.Buffer
	ctx := iostream.WithStreams(context.Background(), iostream.Streams{
		Out: &outBuf,
		Err: &bytes.Buffer{},
		In:  strings.NewReader(""),
	})
	return ctx, &outBuf
}

func TestRender_PopulatedStack_PrintsStackAndImage(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, &Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []SetupStep{
			{Name: "install", Command: "go mod download"},
			{Name: "test", Command: "go test ./..."},
		},
	})

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}

func TestRender_EmptyDetection_PrintsFallback(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, &Result{Stack: StackUnknown})

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}

func TestRender_EmptyString_PrintsFallback(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, &Result{Stack: ""})

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}

func TestRender_NilResult_PrintsFallback(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, nil)

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}

func TestRender_NoSetupSteps_OmitsCommandLines(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, &Result{
		Stack:        "ruby",
		Image:        "cimg/ruby",
		ImageVersion: "3.2",
	})

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}

func TestRender_SystemSetupStep_IsRendered(t *testing.T) {
	ctx, outBuf := captureCtx()

	Render(ctx, &Result{
		Stack:        "python",
		Image:        "cimg/python",
		ImageVersion: "3.12",
		Setup: []SetupStep{
			{Name: "system", Command: "sudo apt-get install -y libpq-dev"},
			{Name: "install", Command: "pip install -r requirements.txt"},
		},
	})

	assert.Check(t, golden.String(outBuf.String(), t.Name()+".txt"))
}
