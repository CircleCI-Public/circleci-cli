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

package cmdinit

import (
	"context"
	"errors"
	"os"
)

// ProjectCreator creates the CircleCI project and returns the first pipeline URL.
type ProjectCreator interface {
	Create(ctx context.Context, token, gitRemoteURL string) (pipelineURL string, err error)
}

type stubProjectCreator struct{}

// NewStubProjectCreator returns the temporary WEBXP-992 project creation stub.
func NewStubProjectCreator() ProjectCreator {
	return stubProjectCreator{}
}

func (stubProjectCreator) Create(context.Context, string, string) (string, error) {
	return "", errors.New("project creation is not yet wired (WEBXP-992)")
}

type fakeProjectCreator struct{}

func (fakeProjectCreator) Create(context.Context, string, string) (string, error) {
	switch env := fakeProjectCreatorEnv(); env {
	case "", "success":
		return "https://app.circleci.com/pipelines/github/example/initfixture/1", nil
	default:
		return "", errors.New(env)
	}
}

func fakeProjectCreatorEnv() string {
	return os.Getenv("CIRCLECI_INIT_FAKE_PROJECT_CREATOR")
}
