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
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/root"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/jq"
)

var version = "dev"

func main() {
	// Ignore SIGPIPE so that piping to an early-exiting command (e.g. `head -5`)
	// surfaces as a normal EPIPE write error rather than terminating the process
	// with exit code 141. Go's I/O layer handles EPIPE silently on stdout/stderr.
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd := root.NewRootCmd(version)
	rootCmd.SetContext(ctx)
	if err := rootCmd.Execute(); err != nil {
		// Extension binary disappeared between startup scan and exec — show a
		// clean error rather than leaking the ErrNotFound message.
		if notFound, ok := errors.AsType[*extension.ErrExtensionBinaryNotFound](err); ok {
			err = clierrors.New("extension.binary_unavailable", "Extension binary missing", notFound.Error()).
				WithSuggestions("Reinstall the extension",
					fmt.Sprintf("circleci extension install %s", notFound.Name),
				).
				WithExitCode(clierrors.ExitNotFound)
		}
		// Extension exit codes are not errors — propagate them directly.
		if exited, ok := errors.AsType[*extension.ErrExited](err); ok {
			if exited.Code > 0 {
				return exited.Code
			}
			if exited.Code < 0 {
				_, _ = fmt.Fprintln(os.Stderr, "extension terminated by signal")
			}
			return clierrors.ExitGeneralError
		}
		// A failed --jq expression is a user input error, not an API/IO failure.
		// Convert it to a structured CLIError so it renders like other argument
		// errors — and so command paths that stream JSON can't mislabel it as an
		// API error. This covers every --jq command, since the error type comes
		// straight from the jq package.
		if jqErr, ok := errors.AsType[*jq.Error](err); ok {
			msg := jqErr.Error()
			if jqErr.Expr != "" {
				msg += "\nexpression: " + jqErr.Expr
			}
			err = clierrors.New("jq.eval_failed", "Invalid --jq expression", msg).
				WithSuggestions(
					"Check the --jq expression for syntax or type errors",
					"See the jq manual: https://jqlang.github.io/jq/manual/",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		// A 410 anywhere means the server dropped this API version — override
		// before display so it renders as a "CLI out of date" error.
		if httpcl.HasStatusCode(err, http.StatusGone) {
			he, _ := errors.AsType[*httpcl.HTTPError](err)
			detail := "This version of circleci CLI is out of date."
			if he != nil {
				if msg := apiclient.ParseServerMessage(he.Body); msg != "" {
					detail = "This version of circleci CLI is out of date. " + msg
				}
			}
			err = clierrors.New("api.gone", "CLI out of date", detail).
				WithSuggestions("Upgrade to the latest version: https://circleci.com/docs/local-cli/").
				WithExitCode(clierrors.ExitAPIError)
		}
		if cliErr, ok := errors.AsType[*clierrors.CLIError](err); ok {
			if jsonFlagPresent() {
				_, _ = fmt.Fprint(os.Stderr, cliErr.FormatJSON())
			} else {
				_, _ = fmt.Fprint(os.Stderr, cliErr.Format())
			}
			return cliErr.ExitCode
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		return clierrors.ExitGeneralError
	}
	return clierrors.ExitSuccess
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
