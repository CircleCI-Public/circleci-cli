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

package pipelineconfig

import (
	"context"
	"strings"
)

// Compiler compiles a config. One interface serves both expanding the input
// and checking every candidate output, so both use the same compilation.
type Compiler interface {
	Compile(ctx context.Context, in CompileInput) (CompileResult, error)
}

// CompileInput is what a compilation takes.
type CompileInput struct {
	ConfigYAML []byte
	// PipelineParameters is << pipeline.parameters.* >>.
	PipelineParameters map[string]any
}

// CompileResult is a successful compilation.
type CompileResult struct {
	CompiledYAML []byte
	// Compiler identifies what compiled it, e.g. "circleci 1.0.49536 (a4bcc413d3e1)".
	Compiler string
}

// InvalidConfigError reports that the compiler rejected the config. It is not
// a transport failure: the compiler ran and said no.
type InvalidConfigError struct {
	Diagnostics []string
}

func (e *InvalidConfigError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "config does not compile"
	}
	return "config does not compile: " + strings.Join(e.Diagnostics, "; ")
}

// TransportError reports that compilation could not be performed at all: the
// compile call failed. Err is the cause, so a caller can map it as an API
// error (an expired token, a server error).
type TransportError struct {
	Err error
}

func (e *TransportError) Error() string {
	const msg = "the compile API call failed"
	if e.Err == nil {
		return msg
	}
	return msg + ": " + e.Err.Error()
}

func (e *TransportError) Unwrap() error { return e.Err }
