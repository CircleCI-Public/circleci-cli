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

package cmdutil

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// FeedbackSuggestion points at `circleci feedback` from errors that look like a
// CircleCI-side fault: a 5xx from the API, or an error no handler classified.
const FeedbackSuggestion = `If this looks like a CircleCI problem, report it: circleci feedback "<what you ran and what went wrong>"`

// UnhandledError marks a non-CLIError returned from a command's RunE. Cobra's
// argument and flag errors never pass through RunE, so they are never marked.
type UnhandledError struct {
	Err error
}

func (e *UnhandledError) Error() string { return e.Err.Error() }

func (e *UnhandledError) Unwrap() error { return e.Err }

// CLIError renders the error as a general error that suggests reporting it.
func (e *UnhandledError) CLIError() *clierrors.CLIError {
	return clierrors.New(unclassifiedErrorCode, "Unexpected error", e.Err.Error()).
		WithSuggestions(FeedbackSuggestion)
}

// MarkUnhandledErrors wraps cmd's RunE to mark its unclassified errors.
// Cancellation and timeouts are expected, not a fault worth reporting.
func MarkUnhandledErrors(cmd *cobra.Command) {
	if cmd.RunE == nil {
		return
	}

	currentRunE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := currentRunE(cmd, args)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if _, ok := errors.AsType[*clierrors.CLIError](err); ok {
			return err
		}
		return &UnhandledError{Err: err}
	}
}

func MarkUnhandledErrorsForSubcommands(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		MarkUnhandledErrors(c)
		MarkUnhandledErrorsForSubcommands(c)
	}
}
