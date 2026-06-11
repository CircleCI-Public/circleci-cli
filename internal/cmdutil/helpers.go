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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// AddJSONFlag registers --json on cmd and binds it to out.
func AddJSONFlag(cmd *cobra.Command, out *bool) {
	cmd.Flags().BoolVar(out, "json", false, "Output as JSON")
}

// AddJQFlag registers --jq on cmd and binds it to out.
func AddJQFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("jq", "", "", "Process values from the response using jq syntax")
}

// AddOutputFlag registers --output/-o on cmd, binding it to out. The what
// argument names the content being written, e.g. "the manpage", and is used in
// the flag description: "Write <what> to this file instead of stdout".
func AddOutputFlag(cmd *cobra.Command, out *string, what string) {
	cmd.Flags().StringVarP(out, "output", "o", "", "Write "+what+" to this file instead of stdout")
}

// OpenOutput resolves the destination for a command supporting --output. When
// path is empty it returns def (the command's normal stdout) with a no-op
// closer; otherwise it creates the file (along with any missing parent
// directories) and returns it with its Close method. Callers should always
// defer the returned closer.
func OpenOutput(path string, def io.Writer) (io.Writer, func() error, error) {
	if path == "" {
		return def, func() error { return nil }, nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil { //#nosec:G301 // generated docs/completions are not sensitive
			return nil, nil, clierrors.New("output.write_failed", "Could not write output file", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
	}
	f, err := os.Create(path) //#nosec:G304 // path is user-supplied
	if err != nil {
		return nil, nil, clierrors.New("output.write_failed", "Could not write output file", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	return f, f.Close, nil
}

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

// GroupRunE is the RunE for group (parent) commands that have no action of
// their own. It shows help when invoked with no arguments and returns a
// structured error for unknown subcommands.
func GroupRunE(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return clierrors.New("command.unknown", "Unknown command",
		fmt.Sprintf("%q is not a %s command. Run '%s --help' for available commands.",
			args[0], cmd.Name(), cmd.CommandPath())).
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
