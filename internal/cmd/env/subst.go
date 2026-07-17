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

package cmdenv

import (
	"io"

	"github.com/MakeNowJust/heredoc"
	"github.com/a8m/envsubst"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newSubstCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "subst [string]",
		Short: "Substitute environment variables in a string",
		Long: heredoc.Doc(`
			Substitute environment variables in a string, similar to the POSIX envsubst utility.

			Pass the string as an argument, or pipe it through stdin. The command writes
			the substituted result to stdout with no trailing newline added.

			Supports $VAR and ${VAR} syntax. References to unset variables are replaced
			with an empty string.
		`),
		Example: heredoc.Doc(`
			# Substitute a single variable
			$ export API_URL=https://circleci.com
			$ circleci env subst "Base URL: $API_URL"

			# Substitute variables in a JSON payload via stdin
			$ export TOKEN=abc123
			$ echo '{"token": "$TOKEN"}' | circleci env subst

			# Substitute into a config file before uploading
			$ circleci env subst < .circleci/config.template.yml > .circleci/config.yml
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: substRunE,
	}
}

func substRunE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	streams := iostream.Get(ctx)

	var input string
	if len(args) > 0 {
		input = args[0]
	} else {
		b, err := io.ReadAll(streams.In)
		if err != nil {
			return clierrors.New("env.subst.read_failed", "Could not read stdin",
				"Reading from stdin: "+err.Error()).
				WithExitCode(clierrors.ExitBadArguments)
		}
		input = string(b)
	}

	if input == "" {
		return nil
	}

	result, err := envsubst.String(input)
	if err != nil {
		return clierrors.New("env.subst.failed", "Substitution failed",
			"Expanding environment variables: "+err.Error()).
			WithExitCode(clierrors.ExitBadArguments)
	}

	_, err = io.WriteString(streams.Out, result)
	return err
}
