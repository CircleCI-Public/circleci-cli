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

package orb

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/pack"
)

func newPackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack a multi-file orb directory into a single YAML",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<path>%[1]s: path to an orb source directory or a single orb YAML file.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Pack an orb source directory into a single YAML file.

			If the path is a directory, the '@orb.yml' (or 'orb.yml') file at the
			root is merged with any 'commands/', 'jobs/', 'executors/', and 'examples/'
			subdirectories. Each .yml file in those directories is added as a named
			entry under the corresponding top-level key.

			If the path is a single file it is parsed and written to stdout.

			The merged YAML is written to stdout.
		`),
		Example: heredoc.Doc(`
			# Pack a single orb file
			$ circleci orb pack orb.yml

			# Pack a multi-file orb directory
			$ circleci orb pack ./src

			# Pack and save to a file
			$ circleci orb pack ./src > orb.yml

			# Pack and immediately validate
			$ circleci orb pack ./src | circleci orb validate -
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := cmdutil.RequireArgs(args, "path"); err != nil {
				return err
			}
			packed, err := pack.Pack(args[0])
			if err != nil {
				return clierrors.New("orb.pack_failed", "Orb pack failed",
					fmt.Sprintf("Could not pack %q: %s", args[0], err)).
					WithExitCode(clierrors.ExitBadArguments)
			}
			_, _ = fmt.Fprint(iostream.Out(ctx), packed)
			return nil
		},
	}
}
