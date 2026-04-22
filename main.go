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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/root"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

var version = "dev"

func main() {
	// Ignore SIGPIPE so that piping to an early-exiting command (e.g. `head -5`)
	// surfaces as a normal EPIPE write error rather than terminating the process
	// with exit code 141. Go's I/O layer handles EPIPE silently on stdout/stderr.
	signal.Ignore(syscall.SIGPIPE)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd := root.NewRootCmd(version)
	rootCmd.SetContext(ctx)
	if err := rootCmd.Execute(); err != nil {
		var cliErr *clierrors.CLIError
		if errors.As(err, &cliErr) {
			if jsonFlagPresent() {
				fmt.Fprint(os.Stderr, cliErr.FormatJSON())
			} else {
				fmt.Fprint(os.Stderr, cliErr.Format())
			}
			os.Exit(cliErr.ExitCode)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(clierrors.ExitGeneralError)
	}
}

// jsonFlagPresent reports whether --json appears anywhere in the raw argument
// list. This is intentionally a simple scan rather than full flag parsing —
// we only need it to format errors before Cobra has had a chance to run.
func jsonFlagPresent() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break // everything after -- is positional, not a flag
		}
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}
