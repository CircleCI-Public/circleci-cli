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

// Package configprompt builds the request for the server-side config prompt that
// `circleci config generate` prints for a coding agent.
package configprompt

import (
	"os"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// NewRequest describes the repository at dir. A nil or empty result sends no stack.
func NewRequest(dir string, result *reposcan.Result) apiclient.ConfigPromptRequest {
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	req := apiclient.ConfigPromptRequest{ConfigPath: filepath.ToSlash(configPath)}
	if _, err := os.Stat(configPath); err == nil {
		req.HasExistingConfig = true
	}
	if result.IsEmpty() {
		return req
	}

	stack := &apiclient.ConfigPromptStack{
		Stack:        result.Stack,
		Image:        result.Image,
		ImageVersion: result.ImageVersion,
	}
	for _, step := range result.Setup {
		stack.Setup = append(stack.Setup, apiclient.ConfigPromptSetupStep{Name: step.Name, Command: step.Command})
	}
	req.Stack = stack
	return req
}
