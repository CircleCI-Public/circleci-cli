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

package engine_test

import (
	"context"
	"errors"
	"testing"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// fakeCompiler returns a fixed result or error.
type fakeCompiler struct {
	compiled []byte
	err      error
	calls    int
}

func (f *fakeCompiler) Compile(context.Context, pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	f.calls++
	if f.err != nil {
		return pipelineconfig.CompileResult{}, f.err
	}
	return pipelineconfig.CompileResult{CompiledYAML: f.compiled, Compiler: "fake"}, nil
}

func TestLoad(t *testing.T) {
	valid := []byte("version: 2.1\njobs:\n  build:\n    docker: [{image: cimg/base:current}]\n    steps: [checkout]\n")

	tests := []struct {
		name        string
		src         []byte
		compiler    *fakeCompiler
		wantCode    string
		wantExit    int
		wantMessage string
		wantCalls   int
	}{
		{
			name:        "invalid YAML is refused before compiling",
			src:         []byte("jobs: [unclosed\n"),
			compiler:    &fakeCompiler{},
			wantCode:    "config.input_invalid",
			wantExit:    clierrors.ExitValidationFail,
			wantMessage: "the input is not valid YAML",
			wantCalls:   0,
		},
		{
			name:        "a config the compiler rejects exits 7",
			src:         valid,
			compiler:    &fakeCompiler{err: &pipelineconfig.InvalidConfigError{Diagnostics: []string{"[#/jobs/build] required key [docker] not found"}}},
			wantCode:    "config.input_invalid",
			wantExit:    clierrors.ExitValidationFail,
			wantMessage: "the input config does not compile:\n  [#/jobs/build] required key [docker] not found",
			wantCalls:   1,
		},
		{
			name:        "cancellation exits 6",
			src:         valid,
			compiler:    &fakeCompiler{err: context.Canceled},
			wantCode:    "run.cancelled",
			wantExit:    clierrors.ExitCancelled,
			wantMessage: "cancelled",
			wantCalls:   1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine.Load(context.Background(), tc.compiler, tc.src, nil)
			cliErr, ok := errors.AsType[*clierrors.CLIError](err)
			assert.Assert(t, ok, "want a CLIError, got %v", err)
			assert.Check(t, cmp.Equal(cliErr.Code, tc.wantCode))
			assert.Check(t, cmp.Equal(cliErr.ExitCode, tc.wantExit))
			assert.Check(t, cmp.Contains(cliErr.Message, tc.wantMessage))
			assert.Check(t, cmp.Equal(tc.compiler.calls, tc.wantCalls))
		})
	}

	t.Run("a failed compile call keeps its cause for the command to map", func(t *testing.T) {
		cause := errors.New("401 Unauthorized")
		_, err := engine.Load(context.Background(), &fakeCompiler{err: &pipelineconfig.TransportError{Message: "the compile API call failed", Err: cause}}, valid, nil)
		_, ok := errors.AsType[*pipelineconfig.TransportError](err)
		assert.Check(t, ok, "want the TransportError, got %v", err)
		assert.Check(t, cmp.ErrorIs(err, cause))
	})
	t.Run("a valid config loads both copies", func(t *testing.T) {
		loaded, err := engine.Load(context.Background(), &fakeCompiler{compiled: valid}, valid, nil)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(loaded.Authored.Bytes(), valid))
		assert.Check(t, cmp.Len(loaded.Effective.Jobs, 1))
		assert.Check(t, cmp.Equal(loaded.Compiler, "fake"))
	})
}
