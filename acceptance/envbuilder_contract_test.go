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

package acceptance_test

import (
	"context"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// TestEnvBuilderEmitsTestStepContract pins the contract between
// `circleci config generate` and chunk-cli's env-builder: for a .NET project,
// env-builder must emit a setup step named "test" with a "dotnet test" command.
// If env-builder ever renames the step or stops emitting it, the generated
// config silently loses its test step — this test catches that regression at
// the chunk-cli upgrade.
func TestEnvBuilderEmitsTestStepContract(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)

	result, err := reposcan.NewDefaultScanner().Scan(context.Background(), dir)
	assert.NilError(t, err)

	cmd := result.SetupCommand("test")
	assert.Check(t, cmd != "", "env-builder should emit a 'test' setup step for .NET projects")
	assert.Check(t, strings.HasPrefix(cmd, "dotnet test"), "expected dotnet test command, got: %s", cmd)
}
