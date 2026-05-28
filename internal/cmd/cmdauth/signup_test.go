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

package cmdauth

import (
	"bytes"
	"context"
	stderrors "errors"
	"strings"
	"testing"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNewSignupCmd_Metadata(t *testing.T) {
	cmd := newSignupCmd()

	assert.Check(t, is.Equal(cmd.Use, "signup"))
	assert.Check(t, cmd.Short != "", "Short must be set")
	assert.Check(t, cmd.Long != "", "Long must be set")
	assert.Check(t, cmd.Example != "", "Example must be set")
	assert.Check(t, cmd.RunE != nil, "RunE must be wired")

	// Per CLAUDE.md, every command needs at least 3 examples.
	// Each example begins with a shell prompt ("$ ") on a fresh line.
	exampleCount := strings.Count(cmd.Example, "\n$ ")
	assert.Check(t, exampleCount >= 3,
		"expected ≥3 examples, got %d in:\n%s", exampleCount, cmd.Example)

	// Args == cobra.NoArgs — pass an unexpected positional arg and expect error.
	err := cmd.Args(cmd, []string{"unexpected-positional"})
	assert.Check(t, err != nil, "signup must reject positional args")
}

func TestSignupIfNeeded_AlreadyAuthenticated(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":    "user-uuid-1234",
		"name":  "Test User",
		"login": "testuser",
	})
	t.Setenv("CIRCLECI_HOST", fake.URL())
	t.Setenv("CIRCLECI_TOKEN", "test-token")

	var stdout bytes.Buffer
	ctx := testIOContext(&stdout)

	err := SignupIfNeeded(ctx, "", true, false)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(stdout.String(), "✓ Already signed in as testuser"))
}

func TestSignupIfNeeded_StaleToken(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	t.Setenv("CIRCLECI_HOST", fake.URL())
	t.Setenv("CIRCLECI_TOKEN", "stale-token")

	var stdout bytes.Buffer
	ctx := testIOContext(&stdout)

	err := SignupIfNeeded(ctx, "", true, false)
	var cliErr *clierrors.CLIError
	assert.Assert(t, stderrors.As(err, &cliErr), "expected CLIError, got %T: %v", err, err)
	assert.Check(t, is.Equal(cliErr.Code, "auth.signup.stale_token"))
	assert.Check(t, is.Equal(cliErr.ExitCode, clierrors.ExitAuthError))
}

func testIOContext(stdout *bytes.Buffer) context.Context {
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(""))
	return iostream.FromCmd(context.Background(), cmd)
}
