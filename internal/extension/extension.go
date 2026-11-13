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

package extension

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// Unmanaged represents an extension not managed by the CLI.
type Unmanaged struct {
	Name       string
	BinaryName string
	Path       string
}

// Run executes the extension binary with args, injecting CircleCI environment
// variables.
//
// The extension is run as a child process and its exit code is propagated via
// *ErrExited.
//
// On SIGINT or SIGTERM the CLI keeps waiting so the extension can shut down
// cleanly. SIGTERM is forwarded to the extension (except on Windows); SIGINT
// is not, since Ctrl+C already reaches the extension via the terminal's
// process group and a duplicate would look like a second Ctrl+C. The
// extension is killed if it is still running after shutdownGrace, or
// immediately on a second signal.
//
// If the extension binary is not found, ErrExtensionBinaryNotFound is returned
// and the caller should prompt the user to reinstall the extension.
func (ext *Unmanaged) Run(ctx context.Context, client *apiclient.Client, args []string) error {
	path := ext.Path

	_, err := os.Stat(path)
	if err != nil {
		return &ErrExtensionBinaryNotFound{
			Name: ext.Name,
			Path: path,
		}
	}

	env := buildEnv(ctx, client)

	// Not CommandContext: the context is cancelled by the first signal, which
	// would SIGKILL the extension before it can clean up.
	cmd := exec.Command(path, args...) //#nosec:G204,G702 // path comes from FindAllOnPATH, args are user-supplied CLI args for the extension
	cmd.Stdin = iostream.In(ctx)
	cmd.Stdout = iostream.Out(ctx)
	cmd.Stderr = iostream.Err(ctx)
	cmd.Env = env

	// Registered before Start so no signal is missed.
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})
	supervised := make(chan struct{})
	go func() {
		defer close(supervised)
		superviseShutdown(cmd.Process, sigs, done, shutdownGrace)
	}()
	err = cmd.Wait()
	close(done)
	<-supervised

	if err != nil {
		if cmd.ProcessState != nil {
			return &ErrExited{Code: cmd.ProcessState.ExitCode()}
		}

		return err
	}
	return nil
}

// shutdownGrace is how long an extension gets to exit after the first
// SIGINT/SIGTERM before it is killed. A variable so tests can shorten it.
var shutdownGrace = 5 * time.Second

// superviseShutdown waits for a signal on sigs. The first one starts the
// grace timer and, if it is SIGTERM, is forwarded to the process. SIGINT is
// not forwarded: a terminal Ctrl+C is delivered to the whole foreground
// process group, so the extension already has it, and a second copy would
// trip "press Ctrl+C again to force quit" handling or interrupt cleanup
// (e.g. a second KeyboardInterrupt in Python). On Windows nothing is
// forwarded, as Process.Signal cannot send these signals. When the timer
// fires or a second signal arrives the process is killed. It returns when
// done is closed or after killing.
func superviseShutdown(p *os.Process, sigs <-chan os.Signal, done <-chan struct{}, grace time.Duration) {
	select {
	case sig := <-sigs:
		if sig == syscall.SIGTERM && runtime.GOOS != "windows" {
			_ = p.Signal(sig)
		}
	case <-done:
		return
	}

	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-sigs:
	case <-done:
		return
	}
	_ = p.Kill()
}

// FindAllOnPATH scans PATH for executables named "circleci-<name>" and returns the
// extension names (the part after "circleci-"). The first entry in PATH wins
// for duplicate names, matching exec.LookPath semantics.
func FindAllOnPATH() []Unmanaged {
	path := os.Getenv("PATH")

	seen := map[string]bool{}
	var exts []Unmanaged
	for _, dir := range filepath.SplitList(path) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			fileName := e.Name()

			binaryName := fileName
			if runtime.GOOS == "windows" {
				binaryName = trimExeSufix(binaryName)
			}

			extName, ok := strings.CutPrefix(binaryName, "circleci-")
			if !ok {
				continue
			}

			if extName == "" || seen[extName] {
				continue
			}
			if runtime.GOOS != "windows" {
				info, err := e.Info()
				if err != nil || info.Mode()&0o111 == 0 {
					continue
				}
			}
			seen[extName] = true
			exts = append(exts, Unmanaged{
				Name:       extName,
				BinaryName: binaryName,
				Path:       filepath.Join(dir, fileName),
			})
		}
	}
	return exts
}

var windowsExtensions = []string{
	".exe",
	".sh",
	".ps1",
}

func trimExeSufix(extName string) string {
	for _, extension := range windowsExtensions {
		ext, ok := strings.CutSuffix(extName, extension)
		if ok {
			extName = ext
			break
		}
	}
	return extName
}
