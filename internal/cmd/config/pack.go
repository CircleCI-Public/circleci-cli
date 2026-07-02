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

package cmdconfig

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/pack"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack <path>",
		Short: "Bundle split config files into a single YAML document",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				path is the path to a split config directory to pack,
				e.g. ".circleci" or "src/ci". The directory structure is
				mapped to YAML keys in the merged document.
			`),
		},
		Long: heredoc.Doc(`
			Bundle a split CircleCI config directory into a single YAML document.

			When a config is split across multiple files, pack merges them back
			into the single-file format that CircleCI accepts. The directory
			structure maps to YAML keys:

			  .circleci/
			    config.yml          → merged at the top level
			    jobs/
			      build.yml         → jobs.build
			      test.yml          → jobs.test

			Files whose names begin with "@" are merged at the current level
			rather than nested under a key.

			The merged YAML is printed to stdout.
		`),
		Example: heredoc.Doc(`
			# Pack the default config directory
			$ circleci config pack .circleci

			# Pack and pipe to validate
			$ circleci config pack .circleci | circleci config validate --config -

			# Pack a custom directory
			$ circleci config pack src/ci
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			packed, err := pack.Pack(args[0])
			if err != nil {
				return clierrors.New("config.pack_failed", "Config pack failed",
					fmt.Sprintf("Could not pack %q: %s", args[0], err)).
					WithExitCode(clierrors.ExitBadArguments)
			}
			_, _ = fmt.Fprint(iostream.Out(ctx), packed)
			return nil
		},
	}

	return cmd
}
