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

//go:build !windows

// This test drives a real terminal-backed PTY via go-expect to exercise the
// interactive viewport pager. The expect console's Tty() is Unix-only (Windows
// uses ConPTY), matching the acceptance harness which skips TTY snapshots on
// Windows, so this file is constrained to non-Windows builds.

package acceptance_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/pete-woods/go-expect"
	"gotest.tools/v3/assert"
)

// TestNoColorFlag asserts that --no-color strips ANSI color exactly like the
// NO_COLOR env var, including through the built-in viewport pager — the path
// that renders styled markdown independently of our ColorEnabled() gate and
// where a naive flag implementation still leaks color.
//
// The table is made tall and the PTY short so the output exceeds the screen and
// the interactive viewport engages.
func TestNoColorFlag(t *testing.T) {
	run := func(t *testing.T, flagArgs, extraEnv []string) string {
		t.Helper()
		fake := fakes.NewCircleCI(t)
		for i := 0; i < 12; i++ {
			fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
		}

		home := t.TempDir()
		env := append([]string{
			"HOME=" + home,
			"XDG_CONFIG_HOME=" + home + "/.config",
			"PATH=" + os.Getenv("PATH"),
			"TERM=xterm-256color",
			"CIRCLE_NO_TELEMETRY=true",
			"CIRCLE_TOKEN=testtoken",
			"CIRCLE_HOST=" + fake.URL(),
		}, extraEnv...)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := append([]string{"context", "list", "--org", testOrgSlug}, flagArgs...)
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Env = env

		var out bytes.Buffer
		c, err := expect.NewConsole(expect.WithStdout(&out), expect.WithTermSize(120, 6))
		assert.NilError(t, err)
		defer func() { _ = c.Close() }()

		assert.NilError(t, c.Start(cmd))
		// Let the viewport render, then quit it so the process exits.
		time.Sleep(500 * time.Millisecond)
		_, _ = c.Send("q")
		_ = expect.WaitProcess(ctx, cmd)
		_ = c.Close()
		_, _ = c.ExpectEOF()
		return out.String()
	}

	const colorSeq = "38;5;" // 256-color SGR sequence emitted by the themed renderer

	colored := run(t, nil, nil)
	flag := run(t, []string{"--no-color"}, nil)
	envVar := run(t, nil, []string{"NO_COLOR=1"})

	assert.Check(t, strings.Contains(colored, colorSeq), "default interactive output should contain ANSI color")
	assert.Check(t, !strings.Contains(envVar, colorSeq), "NO_COLOR=1 output should contain no ANSI color")
	assert.Check(t, !strings.Contains(flag, colorSeq), "--no-color output should contain no ANSI color")
}
