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

package configprompt

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

func TestNewRequest(t *testing.T) {
	const configPath = ".circleci/config.yml"
	tests := []struct {
		name           string
		existingConfig bool
		result         *reposcan.Result
		want           apiclient.ConfigPromptRequest
	}{
		{
			name: "detected stack",
			result: &reposcan.Result{
				Stack:        "go",
				Image:        "cimg/go",
				ImageVersion: "1.23.12",
				Setup:        []reposcan.SetupStep{{Name: "test", Command: "go test ./..."}},
			},
			want: apiclient.ConfigPromptRequest{
				Stack: &apiclient.ConfigPromptStack{
					Stack:        "go",
					Image:        "cimg/go",
					ImageVersion: "1.23.12",
					Setup:        []apiclient.ConfigPromptSetupStep{{Name: "test", Command: "go test ./..."}},
				},
				ConfigPath: configPath,
			},
		},
		{
			name:   "unknown stack sends no stack",
			result: &reposcan.Result{Stack: reposcan.StackUnknown},
			want:   apiclient.ConfigPromptRequest{ConfigPath: configPath},
		},
		{
			name: "nil result sends no stack",
			want: apiclient.ConfigPromptRequest{ConfigPath: configPath},
		},
		{
			name:           "existing config",
			existingConfig: true,
			want:           apiclient.ConfigPromptRequest{ConfigPath: configPath, HasExistingConfig: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tc.existingConfig {
				assert.NilError(t, os.MkdirAll(".circleci", 0o755))
				assert.NilError(t, os.WriteFile(configPath, []byte("version: 2.1\n"), 0o644))
			}

			assert.Check(t, cmp.DeepEqual(NewRequest(".", tc.result), tc.want))
		})
	}
}
