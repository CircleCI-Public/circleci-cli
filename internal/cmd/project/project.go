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

// Package project implements the "circleci project" command group.
package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// NewProjectCmd returns the "circleci project" parent command.
func NewProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Manage CircleCI projects",
		Long: heredoc.Doc(`
			List, follow, and manage settings for CircleCI projects.

			A project corresponds to a version-control repository connected
			to CircleCI. Use 'circleci project list' to see all followed projects,
			'circleci project follow' to start following a new project, and
			'circleci project env' to manage environment variables.

			To manage environment variables directly, use the top-level alias:
			  circleci envvar list --project gh/org/repo
		`),
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newFollowCmd())
	cmd.AddCommand(newEnvCmd())

	return cmd
}
