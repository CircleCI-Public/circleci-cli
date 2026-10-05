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

package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Loaded is a config ready for analysis.
type Loaded struct {
	Authored   *pipelineconfig.Authored
	Effective  *pipelineconfig.Effective
	Mutability *pipelineconfig.Mutability
	// Compiler identifies what compiled the effective copy.
	Compiler string
	// Compiled is the compiler's raw output, for the apply checks.
	Compiled []byte
}

// Load parses and compiles a pipelineconfig. It refuses a config that does not
// compile: every later compile failure then belongs to a change.
func Load(ctx context.Context, compiler pipelineconfig.Compiler, src []byte, params map[string]any) (*Loaded, error) {
	authored, err := pipelineconfig.Parse(src)
	if err != nil {
		return nil, inputInvalid("the input is not valid YAML", []string{err.Error()})
	}
	res, err := compiler.Compile(ctx, pipelineconfig.CompileInput{ConfigYAML: src, PipelineParameters: params})
	if err != nil {
		return nil, compileError(err)
	}
	eff, err := pipelineconfig.NewEffective(res.CompiledYAML)
	if err != nil {
		return nil, clierrors.New("api.compile_output_unreadable", "Compiler output unreadable", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}
	return &Loaded{
		Authored:   authored,
		Effective:  eff,
		Mutability: pipelineconfig.NewMutability(authored, eff),
		Compiler:   res.Compiler,
		Compiled:   res.CompiledYAML,
	}, nil
}

func compileError(err error) error {
	if invalid, ok := errors.AsType[*pipelineconfig.InvalidConfigError](err); ok {
		return inputInvalid("the input config does not compile", invalid.Diagnostics)
	}
	if unsupported, ok := errors.AsType[*pipelineconfig.UnsupportedError](err); ok {
		return clierrors.New("compile.unsupported", "Compiler cannot represent the input", unsupported.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return clierrors.New("run.cancelled", "Cancelled", "compilation was cancelled").
			WithExitCode(clierrors.ExitCancelled)
	case errors.Is(err, context.DeadlineExceeded):
		return clierrors.New("run.timeout", "Timed out", "compilation timed out").
			WithExitCode(clierrors.ExitTimeout)
	}
	// A failed call goes back as it is: the command maps it the way
	// circleci config process maps an API error.
	return err
}

func inputInvalid(what string, diagnostics []string) error {
	msg := what
	if len(diagnostics) > 0 {
		msg = fmt.Sprintf("%s:\n  %s", what, strings.Join(diagnostics, "\n  "))
	}
	return clierrors.New("config.input_invalid", "Input config is invalid", msg).
		WithSuggestions("Run `circleci config validate` to see the errors, fix them, then run again").
		WithExitCode(clierrors.ExitValidationFail)
}
