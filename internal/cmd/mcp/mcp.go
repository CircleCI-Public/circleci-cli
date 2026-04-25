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

package mcp

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// NewMCPCmd returns the "circleci mcp" command group.
func NewMCPCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp <command>",
		Short: "Expose CircleCI CLI as an MCP server",
		Long: heredoc.Doc(`
			Expose CircleCI CLI functionality as Model Context Protocol (MCP) tools,
			allowing AI agents such as Claude to interact with CircleCI on your behalf.

			Configure your MCP client to run 'circleci mcp serve'. The server reads
			your existing CircleCI credentials and inherits your working directory,
			so git-based project detection works the same as in the CLI.
		`),
	}

	cmd.AddCommand(newServeCmd(version))
	return cmd
}
