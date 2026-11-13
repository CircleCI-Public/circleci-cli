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

package extension

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

// startScript starts a shell script child and a goroutine supervising it, and
// returns the channel to send signals on plus a function that waits for the
// child and reports its exit code and how long the wait took.
func startScript(t *testing.T, script string, grace time.Duration) (chan<- os.Signal, func() (int, time.Duration)) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "child.sh")
	assert.NilError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755))

	cmd := exec.Command(path)
	assert.NilError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	sigs := make(chan os.Signal, 2)
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		superviseShutdown(cmd.Process, sigs, done, grace)
	}()

	return sigs, func() (int, time.Duration) {
		start := time.Now()
		_ = cmd.Wait()
		elapsed := time.Since(start)
		close(done)
		<-finished
		return cmd.ProcessState.ExitCode(), elapsed
	}
}

// ready gives the shell time to install its traps before we signal it.
func ready() { time.Sleep(300 * time.Millisecond) }

func TestSuperviseShutdown(t *testing.T) {
	t.Run("forwards the signal and lets the child exit on its own", func(t *testing.T) {
		sigs, wait := startScript(t, `trap 'exit 42' TERM INT; sleep 30 & wait`, 10*time.Second)
		ready()
		sigs <- syscall.SIGTERM

		code, elapsed := wait()
		assert.Check(t, cmp.Equal(code, 42))
		assert.Check(t, elapsed < 5*time.Second, "child should exit promptly, took %s", elapsed)
	})

	t.Run("does not forward SIGINT", func(t *testing.T) {
		// The terminal already delivers Ctrl+C to the extension; a forwarded
		// copy would arrive as a second one. If SIGINT were forwarded the
		// child would exit 42; instead it is killed after the grace period.
		const grace = 200 * time.Millisecond
		sigs, wait := startScript(t, `trap 'exit 42' INT; while :; do sleep 1; done`, grace)
		ready()
		sigs <- syscall.SIGINT

		code, elapsed := wait()
		assert.Check(t, cmp.Equal(code, -1))
		assert.Check(t, elapsed < 5*time.Second, "took %s", elapsed)
	})

	t.Run("kills a child that ignores the signal after the grace period", func(t *testing.T) {
		const grace = 200 * time.Millisecond
		sigs, wait := startScript(t, `trap '' TERM INT; while :; do sleep 1; done`, grace)
		ready()
		sigs <- syscall.SIGTERM

		code, elapsed := wait()
		assert.Check(t, cmp.Equal(code, -1))
		assert.Check(t, elapsed >= grace/2, "killed before grace elapsed: %s", elapsed)
		assert.Check(t, elapsed < 5*time.Second, "took %s", elapsed)
	})

	t.Run("kills immediately on a second signal", func(t *testing.T) {
		sigs, wait := startScript(t, `trap '' TERM INT; while :; do sleep 1; done`, time.Minute)
		ready()
		sigs <- syscall.SIGINT
		sigs <- syscall.SIGINT

		code, elapsed := wait()
		assert.Check(t, cmp.Equal(code, -1))
		assert.Check(t, elapsed < 5*time.Second, "took %s", elapsed)
	})
}

func TestRunPropagatesExitCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "circleci-fake")
	assert.NilError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 7\n"), 0o755))

	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	ctx := context.Background()
	cfg, err := config.Load(ctx, filepath.Join(dir, "config.yml"), false)
	assert.NilError(t, err)
	ctx = cmdutil.WithConfig(ctx, cfg)

	ext := &Unmanaged{Name: "fake", BinaryName: "circleci-fake", Path: path}
	err = ext.Run(ctx, nil, nil)

	var exited *ErrExited
	assert.Assert(t, errors.As(err, &exited), "got %v", err)
	assert.Check(t, cmp.Equal(exited.Code, 7))
}
