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

	"github.com/CircleCI-Public/chunk-cli/envbuilder"
)

// EnvBuilderScanner detects the stack and setup commands for a repository
// using chunk-cli's env-builder library. It may make network calls (e.g. to
// Docker Hub) while resolving image versions, so callers should pass a context
// with a timeout if they need to bound the operation.
type EnvBuilderScanner struct{}

// NewDefaultScanner returns an EnvBuilderScanner ready for use.
func NewDefaultScanner() *EnvBuilderScanner {
	return &EnvBuilderScanner{}
}

func (s *EnvBuilderScanner) Scan(ctx context.Context, dir string) (*Result, error) {
	env, err := envbuilder.DetectEnvironment(ctx, dir)
	if err != nil {
		return nil, err
	}
	return resultFromEnvironment(env), nil
}

func resultFromEnvironment(env *envbuilder.Environment) *Result {
	if env == nil {
		return nil
	}
	r := &Result{
		Stack:        env.Stack,
		Image:        env.Image,
		ImageVersion: env.ImageVersion,
	}
	for _, step := range env.Setup {
		r.Setup = append(r.Setup, SetupStep{Name: step.Name, Command: step.Command})
	}
	return r
}
