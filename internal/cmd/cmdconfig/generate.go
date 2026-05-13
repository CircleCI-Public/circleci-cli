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
	"errors"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate .circleci/config.yml from a repository scan",
		Long: heredoc.Doc(`
			Detect the language stack, container image, and setup commands for a
			repository, then write a starter pipeline to <path>/.circleci/config.yml.

			If a config file already exists at that path, generate does not overwrite
			it; it prints a confirmation and exits successfully. The .circleci/
			directory is created if needed, and the file is written atomically.
		`),
		Example: heredoc.Doc(`
			# Generate a config for the current directory
			$ circleci config generate

			# Generate a config for a specific project path
			$ circleci config generate ./my-app

			# Re-run is a no-op when a config already exists
			$ circleci config generate
			✓ Using existing config at .circleci/config.yml
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented")
		},
	}
	return cmd
}
