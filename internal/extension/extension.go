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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
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
// The current process is replaced by the extension via syscall exec on Unix;
// on Windows the extension is run as a child process and its exit code is
// propagated.
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

	cmd := exec.CommandContext(ctx, path, args...) //#nosec:G204,G702 // path comes from FindAllOnPATH, args are user-supplied CLI args for the extension
	cmd.Stdin = iostream.In(ctx)
	cmd.Stdout = iostream.Out(ctx)
	cmd.Stderr = iostream.Err(ctx)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil {
			return &ErrExited{Code: cmd.ProcessState.ExitCode()}
		}

		return err
	}
	return nil
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
