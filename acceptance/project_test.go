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
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func setupProjectFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddFollowedProject(map[string]any{
		"slug":     "gh/myorg/alpha",
		"username": "myorg",
		"reponame": "alpha",
		"vcs_type": "github",
		"name":     "alpha",
	})
	fake.AddFollowedProject(map[string]any{
		"slug":     "gh/myorg/beta",
		"username": "myorg",
		"reponame": "beta",
		"vcs_type": "github",
		"name":     "beta",
	})
	createdAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fake.AddEnvVar("gh/myorg/alpha", "DATABASE_URL", "xxxx", nil)
	fake.AddEnvVar("gh/myorg/alpha", "SECRET_KEY", "xxxx", &createdAt)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- project list ---

func TestProjectList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectList_JSON(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["slug"], "gh/myorg/alpha"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))

}

func TestProjectList_JSON_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestProjectList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestProjectList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- project follow ---

func TestProjectFollow(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "follow", "--project", "gh/myorg/newrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectFollow_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "follow", "--project", "gh/myorg/newrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectFollow_Idempotent(t *testing.T) {
	_, env := setupProjectFake(t)

	// Follow an already-followed project — should succeed.
	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "follow", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestProjectFollow_InvalidSlug(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "follow", "--project", "notaslug"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- env list (top-level alias) ---

func TestEnvList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvList_JSON(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["name"], "DATABASE_URL"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEnvList_JSON_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEnvList_Empty(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "list", "--project", "gh/myorg/beta"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// Also accessible via the deep path.
func TestProjectEnvList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectEnvList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"project", "envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- env set ---

func TestEnvSet(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "set", "NEW_VAR", "newvalue", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "set", "NEW_VAR", "newvalue", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Overwrite(t *testing.T) {
	_, env := setupProjectFake(t)

	// Overwrite existing var.
	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "set", "DATABASE_URL", "postgres://new", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Overwrite_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "set", "DATABASE_URL", "postgres://new", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- env delete ---

func TestEnvDelete(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "delete", "--force", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvDelete_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "delete", "--force", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvDelete_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "delete", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEnvDelete_NotFound(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "delete", "--force", "DOES_NOT_EXIST", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEnvDelete_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Args:    []string{"envvar", "delete", "--force", "FOO", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}
