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
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
)

func stringifyReference(cmd *cobra.Command) string {
	var buf = bytes.Buffer{}

	// Lead with the root command's intro blurb so the reference reads like the
	// top-level help rather than starting abruptly at the first command.
	intro := cmd.Long
	if intro == "" {
		intro = cmd.Short
	}
	if intro != "" {
		_, _ = fmt.Fprintf(&buf, "%s\n\n", strings.TrimSpace(intro))
	}

	// Global flags apply to every command, so document them once up front.
	if flags := cmd.PersistentFlags(); flags.HasAvailableFlags() {
		_, _ = fmt.Fprintf(&buf, "## Global Flags\n\n")
		_, _ = fmt.Fprintf(&buf, "%s\n\n", mdTable("Flag", "Description", flagRows(flags)))
	}

	// Group top-level commands under the same headings cobra uses for the root
	// help (CI / Management / User / Additional Commands), in the same order, so
	// the reference mirrors `circleci --help`.
	for _, g := range groupedCommands(cmd) {
		_, _ = fmt.Fprintf(&buf, "## %s\n\n", titleCase(g.Title))
		for _, c := range g.Commands {
			cmdRef(&buf, c, 3)
		}
	}
	return buf.String()
}

func cmdRef(w io.Writer, cmd *cobra.Command, depth int) {
	// Managed extensions can supply their own documentation.
	// If extension.ReferenceAnnotation is set, the manifest contains
	// a reference field, use that to build the reference instead.
	if binaryName, ok := cmd.Annotations[extension.ReferenceAnnotation]; ok {
		extDir, err := config.ExtensionsDir()
		if err == nil {
			store := extension.NewStore(extDir)
			if manifest, err := store.Get(binaryName); err == nil {
				extensionCmdRef(w, cmd.CommandPath(), manifest.Ref, depth)
				return
			}
		}
	}

	_, _ = fmt.Fprintf(w, "%s `%s`\n\n", strings.Repeat("#", depth), cmd.UseLine())
	_, _ = fmt.Fprintf(w, "%s\n\n", cmd.Short)

	flags := cmd.Flags()
	if flags.HasAvailableFlags() {
		_, _ = fmt.Fprintf(w, "%s\n\n", mdTable("Flag", "Description", flagRows(flags)))
	}

	// Render the arguments annotation as a bold label rather than a heading so
	// it doesn't show up as a section in the generated site's heading nav.
	if args := cmd.Annotations["help:arguments"]; strings.Trim(args, "\n") != "" {
		_, _ = fmt.Fprintf(w, "**Arguments:**\n\n%s\n\n", strings.Trim(args, "\n"))
	}

	if len(cmd.Aliases) > 0 {
		_, _ = fmt.Fprintf(w, "%s\n\n", "**Aliases:**")
		aliasList := buildAliasList(cmd, cmd.Aliases)
		for i, a := range aliasList {
			aliasList[i] = "`" + a + "`"
		}
		_, _ = fmt.Fprintf(w, "\n%s\n\n", dedent(strings.Join(aliasList, ", ")))
	}

	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}
		cmdRef(w, c, depth+1)
	}
}

// extensionCmdRef renders a documented extension command from its extension.Reference.
func extensionCmdRef(w io.Writer, path string, ref *extension.Reference, depth int) {
	useLine := path
	if ref.Use != "" {
		useLine += " " + ref.Use
	}
	_, _ = fmt.Fprintf(w, "%s `%s`\n\n", strings.Repeat("#", depth), useLine)

	if ref.Short != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", ref.Short)
	}
	if long := strings.TrimSpace(ref.Long); long != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", long)
	}

	if rows := docFlagRows(ref.Flags); len(rows) > 0 {
		_, _ = fmt.Fprintf(w, "%s\n\n", mdTable("Flag", "Description", rows))
	}

	if args := ref.Args; len(args) > 0 {
		_, _ = fmt.Fprint(w, "**Arguments:**\n\n")

		for _, arg := range args {
			_, _ = fmt.Fprintf(w, "`%s` %s\n", arg.Name, arg.Help)
		}

		_, _ = fmt.Fprint(w, "\n")
	}

	for i := range ref.Subcommands {
		sub := &ref.Subcommands[i]
		subPath := path
		if sub.Name != "" {
			subPath = path + " " + sub.Name
		}
		extensionCmdRef(w, subPath, &sub.Reference, depth+1)
	}
}

// docFlagRows turns documented flags into (name, description) table rows,
// mirroring the "-s, --name TYPE usage (default …)" formatting that flagRows
// produces for cobra pflag.FlagSet.
func docFlagRows(flags []extension.ReferenceFlag) [][2]string {
	var rows [][2]string
	for _, f := range flags {
		if f.Name == "" {
			continue
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		if f.Type != "" {
			name += " " + f.Type
		}
		usage := f.Usage
		if f.Default != "" {
			if f.Type == "string" || f.Type == "" {
				usage += fmt.Sprintf(" (default %q)", f.Default)
			} else {
				usage += fmt.Sprintf(" (default %s)", f.Default)
			}
		}
		rows = append(rows, [2]string{"`" + name + "`", usage})
	}
	return rows
}
