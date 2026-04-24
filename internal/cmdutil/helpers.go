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
	"encoding/json"
	"io"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// WriteJSON encodes v as indented JSON to w. Use streams.Out as the writer.
// Returns the encoder error, if any.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// GitDetectErr wraps a gitremote.Detect error into a structured CLIError.
//
// The standard "run from inside a git repository" suggestion is always included
// as the first suggestion. Pass additional command-specific suggestions as
// variadic args (e.g. "Or specify the project with --project gh/org/repo").
func GitDetectErr(err error, suggestions ...string) *clierrors.CLIError {
	all := append(
		[]string{"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote"},
		suggestions...,
	)
	return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
		WithSuggestions(all...).
		WithExitCode(clierrors.ExitBadArguments)
}

// ConfirmOrForce requires user confirmation of a destructive operation.
//
//   - If force is true, returns nil immediately (scripting / non-interactive path).
//   - In a TTY, shows prompt and returns abortErr if the user declines.
//   - Outside a TTY, returns requireForceErr with the standard --force suggestion
//     appended so callers don't have to repeat it.
//
// Construct abortErr and requireForceErr with domain-specific codes and messages;
// the standard suggestion text is added automatically to requireForceErr.
func ConfirmOrForce(ctx context.Context, streams iostream.Streams, force bool, prompt string, abortErr, requireForceErr *clierrors.CLIError) error {
	if force {
		return nil
	}
	if streams.IsInteractive() {
		if !streams.Confirm(ctx, prompt) {
			return abortErr
		}
		return nil
	}
	return requireForceErr.WithSuggestions("Pass --force (-f) to skip this prompt in non-interactive mode")
}
