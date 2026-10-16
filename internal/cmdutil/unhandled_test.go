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
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

func TestMarkUnhandledErrors(t *testing.T) {
	plain := errors.New("boom")
	cliErr := clierrors.New("thing.failed", "Thing failed", "The thing failed.")

	runWith := func(t *testing.T, runErr error) error {
		t.Helper()
		cmd := &cobra.Command{RunE: func(*cobra.Command, []string) error { return runErr }}
		MarkUnhandledErrors(cmd)
		return cmd.RunE(cmd, nil)
	}

	t.Run("plain error is marked", func(t *testing.T) {
		err := runWith(t, plain)
		u, ok := errors.AsType[*UnhandledError](err)
		assert.Assert(t, ok, "got %T", err)
		assert.Check(t, cmp.Equal(u.Unwrap(), plain))
		assert.Check(t, cmp.Equal(err.Error(), "boom"))
	})

	t.Run("wrapped error is marked and still matches", func(t *testing.T) {
		err := runWith(t, fmt.Errorf("doing thing: %w", plain))
		_, ok := errors.AsType[*UnhandledError](err)
		assert.Check(t, ok, "got %T", err)
		assert.Check(t, cmp.ErrorIs(err, plain))
	})

	for _, tc := range []struct {
		name   string
		runErr error
	}{
		{name: "CLIError is unchanged", runErr: cliErr},
		{name: "wrapped CLIError is unchanged", runErr: fmt.Errorf("context: %w", cliErr)},
		{name: "cancellation is unchanged", runErr: context.Canceled},
		{name: "timeout is unchanged", runErr: fmt.Errorf("waiting: %w", context.DeadlineExceeded)},
		{name: "nil is unchanged", runErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(runWith(t, tc.runErr), tc.runErr))
		})
	}

	t.Run("command without RunE is left alone", func(t *testing.T) {
		cmd := &cobra.Command{}
		MarkUnhandledErrors(cmd)
		assert.Check(t, cmp.Nil(cmd.RunE))
	})
}

func TestMarkUnhandledErrorsForSubcommands(t *testing.T) {
	leaf := &cobra.Command{Use: "leaf", RunE: func(*cobra.Command, []string) error { return errors.New("boom") }}
	group := &cobra.Command{Use: "group"}
	group.AddCommand(leaf)
	root := &cobra.Command{Use: "root"}
	root.AddCommand(group)

	MarkUnhandledErrorsForSubcommands(root)

	_, ok := errors.AsType[*UnhandledError](leaf.RunE(leaf, nil))
	assert.Check(t, ok, "nested command's RunE was not wrapped")
}

func TestUnhandledErrorCLIError(t *testing.T) {
	err := (&UnhandledError{Err: errors.New("boom")}).CLIError()

	assert.Check(t, cmp.Equal(err.Code, "unclassified"))
	assert.Check(t, cmp.Equal(err.Message, "boom"))
	assert.Check(t, cmp.DeepEqual(err.Suggestions, []string{FeedbackSuggestion}))
	assert.Check(t, cmp.Equal(err.ExitCode, clierrors.ExitGeneralError))
}
