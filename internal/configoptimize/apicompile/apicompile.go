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

// Package apicompile compiles configs for config optimize through the
// CircleCI compile API, the same call circleci config process makes.
package apicompile

import (
	"context"
	"errors"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Compiler is a pipelineconfig.Compiler backed by the compile API. OrgID enables
// private and namespaced orb resolution; empty compiles against public orbs.
type Compiler struct {
	Client *apiclient.Client
	OrgID  string
}

// Compile implements pipelineconfig.Compiler. A config the API rejects is a
// *pipelineconfig.InvalidConfigError; a failed call is a *pipelineconfig.TransportError.
func (c Compiler) Compile(ctx context.Context, in pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	res, err := c.Client.Compile(ctx, apiclient.CompileInput{
		ConfigYAML:         string(in.ConfigYAML),
		OrgID:              c.OrgID,
		PipelineValues:     configcmd.LocalPipelineValues(in.PipelineParameters),
		PipelineParameters: in.PipelineParameters,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return pipelineconfig.CompileResult{}, err
		}
		return pipelineconfig.CompileResult{}, &pipelineconfig.TransportError{Err: err}
	}
	if !res.Valid {
		return pipelineconfig.CompileResult{}, &pipelineconfig.InvalidConfigError{Diagnostics: res.Errors}
	}
	return pipelineconfig.CompileResult{CompiledYAML: []byte(res.CompiledYAML), Compiler: "CircleCI compile API"}, nil
}
