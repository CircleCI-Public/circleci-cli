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

package iostream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPromptLineReturnsWhenContextCancelled(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	streams := Streams{
		In:  reader,
		Err: io.Discard,
	}

	done := make(chan error, 1)
	go func() {
		_, err := streams.PromptLine(ctx, "Press Enter...")
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		assert.Check(t, errors.Is(err, context.Canceled), "got %v", err)
	case <-time.After(time.Second):
		t.Fatal("PromptLine did not return after context cancellation")
	}
}

func TestPromptLineReadsAndTrimsInput(t *testing.T) {
	var stderr bytes.Buffer
	streams := Streams{
		In:  bytes.NewBufferString("  hello  \n"),
		Err: &stderr,
	}

	got, err := streams.PromptLine(context.Background(), "Prompt: ")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got, "hello"))
	assert.Check(t, is.Equal(stderr.String(), "Prompt: "))
}
