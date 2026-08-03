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

package root

import (
	"strings"

	"github.com/spf13/cobra"
)

// argumentsHeading labels the argument documentation folded into Long for MCP.
const argumentsHeading = "Arguments:\n"

// inlineArgumentDocs copies each command's "help:arguments" annotation into its
// Long description, recursively, for the MCP command tree only.
//
// ophis builds a tool's description from Long (falling back to Short) plus
// Example, and derives the args schema from the Use line alone — it never reads
// our annotation. So a command that documents its accepted values under
// `## Arguments` rather than in prose (the org/project setting commands
// enumerate every valid `<setting>` there) would expose nothing to an MCP client
// but "Usage pattern: <setting>", leaving it to guess a value and recover from
// the error.
//
// Only the MCP entry points call this, and they build their tool tree inside
// RunE, so `circleci <cmd> --help` still renders the annotation as its own
// `## Arguments` section and stays inside the help line budget. Keeping the
// duplication out of the authored Long is the whole point: the help page shows
// it once, and MCP gets it without a second copy in the source.
func inlineArgumentDocs(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		inlineArgumentDocs(sub)
	}

	args := strings.TrimSpace(cmd.Annotations["help:arguments"])
	if args == "" {
		return
	}
	// Idempotent: `mcp tools` and the servers each call this once per process,
	// but appending twice would double the text if that ever changed.
	if strings.Contains(cmd.Long, argumentsHeading+args) {
		return
	}

	desc := strings.TrimSpace(cmd.Long)
	if desc == "" {
		// ophis falls back to Short only when Long is empty; seeding it here
		// keeps the one-line summary that would otherwise be dropped.
		desc = strings.TrimSpace(cmd.Short)
	}
	if desc != "" {
		desc += "\n\n"
	}
	// Trailing newline so ophis's "\n"-joined Examples block keeps the blank-line
	// separation an authored heredoc Long would have given it.
	cmd.Long = desc + argumentsHeading + args + "\n"
}
