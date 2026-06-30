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
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func TestJobOutputGet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobStdout(testJobID, 0, 103, []byte("hello from stdout\n"))
	fake.AddJobStderr(testJobID, 0, 103, []byte("hello from stderr\n"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", testJobID, "--step-num", "103"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_Execution(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobStdout(testJobID, 1, 103, []byte("execution 1 stdout\n"))
	fake.AddJobStderr(testJobID, 1, 103, []byte("execution 1 stderr\n"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", testJobID, "--step-num", "103", "--execution", "1"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_StripsANSIWhenNotTerminal(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobStdout(testJobID, 0, 103, golden.Get(t, "tty/input.txt"))
	fake.AddJobStderr(testJobID, 0, 103, []byte(""))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", testJobID, "--step-num", "103"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_StripANSIFalseKeepsRawWhenNotTerminal(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobStdout(testJobID, 0, 103, golden.Get(t, "tty/input.txt"))
	fake.AddJobStderr(testJobID, 0, 103, []byte(""))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", testJobID, "--step-num", "103", "--strip-ansi=false"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	// --strip-ansi=false forces the raw bytes through even though stdout is not
	// a terminal, so the output should match the unmodified input verbatim.
	assert.Check(t, cmp.Equal(result.Stdout, string(golden.Get(t, "tty/input.txt"))))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_MissingStepNum(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_InvalidJobID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", "not-a-uuid", "--step-num", "103"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "get", "00000000-0000-0000-0000-000000000000", "--step-num", "103"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// output list

func TestJobOutputList(t *testing.T) {
	fake, env := setupJobGetFake(t)
	// Step 103 has a Docker-style carriage-return progress redraw that must
	// collapse to its final state. Step 101 has plain output. Step 0 has none.
	fake.AddJobStdout(testJobID, 0, 101, []byte("Cloning into 'repo'...\nDone.\n"))
	fake.AddJobStdout(testJobID, 0, 103, []byte(
		"layer: Downloading [==>]\r\x1b[Klayer: Downloading [====>]\r\x1b[Klayer: Download complete\n"+
			"\x1b[32mPASS\x1b[0m\n"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputList_JSON(t *testing.T) {
	fake, env := setupJobGetFake(t)
	fake.AddJobStdout(testJobID, 0, 101, []byte("Cloning into 'repo'...\nDone.\n"))
	fake.AddJobStdout(testJobID, 0, 103, []byte(
		"layer: Downloading [==>]\r\x1b[Klayer: Download complete\n\x1b[32mPASS\x1b[0m\n"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestJobOutputList_LargeOutputIsTruncated verifies that a step with a very
// large output is capped in the rendered markdown (so glamour stays fast) while
// the full output is still available via --json.
func TestJobOutputList_LargeOutputIsTruncated(t *testing.T) {
	fake, env := setupJobGetFake(t)
	var big strings.Builder
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&big, "log line %d\n", i)
	}
	fake.AddJobStdout(testJobID, 0, 103, []byte(big.String()))

	rendered := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(rendered.ExitCode, 0))
	// The tail is kept, the head is hidden, and the user is told how to get more.
	assert.Check(t, cmp.Contains(rendered.Stdout, "log line 599"))
	assert.Check(t, !strings.Contains(rendered.Stdout, "log line 0\n"))
	assert.Check(t, cmp.Contains(rendered.Stdout, "earlier lines hidden"))

	jsonOut := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(jsonOut.ExitCode, 0))
	// --json is never truncated: both ends of the log are present.
	assert.Check(t, cmp.Contains(jsonOut.Stdout, "log line 0"))
	assert.Check(t, cmp.Contains(jsonOut.Stdout, "log line 599"))

	all := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID, "--tail", "0"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(all.ExitCode, 0))
	// --tail 0 shows everything, with no truncation notice.
	assert.Check(t, cmp.Contains(all.Stdout, "log line 0"))
	assert.Check(t, cmp.Contains(all.Stdout, "log line 599"))
	assert.Check(t, !strings.Contains(all.Stdout, "earlier lines hidden"))
}

func TestJobOutputList_Execution(t *testing.T) {
	fake, env := setupJobGetFake(t)
	fake.AddJobStdout(testJobID, 1, 103, []byte("second executor output\n"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID, "--execution", "1", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputList_ExecutionNotFound(t *testing.T) {
	_, env := setupJobGetFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", testJobID, "--execution", "5"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5)) // ExitNotFound
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
