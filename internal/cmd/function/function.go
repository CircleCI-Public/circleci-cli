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

// Package cmdfunction implements the "circleci function" command group.
package cmdfunction

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// NewFunctionCmd returns the parent command for the function command group.
func NewFunctionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "function <command>",
		Short:   "Discover and declare CircleCI functions",
		GroupID: "ci",
		Long: heredoc.Docf(`
			Discover published CircleCI functions and declare them in your config.

			A function is a versioned binary invoked as a step. Run
			%[1]scircleci help functions%[1]s for the config syntax.
		`, "`"),
		Example: heredoc.Doc(`
			# List published functions
			$ circleci function list

			# Show a function's versions and arguments
			$ circleci function get setup-go

			# Declare one in .circleci/config.yml
			$ circleci function add setup-go
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "Commands", newListCmd(), newGetCmd(), newAddCmd(), newUpdateCmd())
	return cmd
}
