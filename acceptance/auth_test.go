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
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func setupAuthFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":         "user-uuid-1234",
		"name":       "Test User",
		"login":      "testuser",
		"avatar_url": "https://example.com/avatar.png",
	})
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- auth me ---

func TestAuthMe(t *testing.T) {
	_, env := setupAuthFake(t)

	result := binary.RunCLI(t, []string{"auth", "me"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestAuthMe_JSON(t *testing.T) {
	_, env := setupAuthFake(t)

	result := binary.RunCLI(t, []string{"auth", "me", "--json"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["login"], "testuser"))
	assert.Check(t, cmp.Equal(out["name"], "Test User"))
	assert.Check(t, cmp.Equal(out["id"], "user-uuid-1234"))
}

func TestAuthMe_NoToken(t *testing.T) {
	_, env := setupAuthFake(t)
	env.Token = ""

	result := binary.RunCLI(t, []string{"auth", "me"}, env.Environ(), t.TempDir())

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, cmp.Contains(result.Stderr, "token"))
}

// --- auth logout ---

func TestAuthLogout(t *testing.T) {
	_, env := setupAuthFake(t)

	result := binary.RunCLI(t, []string{"auth", "logout"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "token"))
}
