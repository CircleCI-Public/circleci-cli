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
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func rootUsage(command *cobra.Command) error {
	ctx := command.Context()

	iostream.ErrPrintf(ctx, "Usage:  %s", command.UseLine())

	var subcommands []*cobra.Command
	for _, c := range command.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		subcommands = append(subcommands, c)
	}

	if len(subcommands) > 0 {
		iostream.ErrPrintf(ctx, "\n\nAvailable commands:\n")
		for _, c := range subcommands {
			iostream.ErrPrintf(ctx, "  %s\n", c.Name())
		}
		return nil
	}

	flagUsages := command.LocalFlags().FlagUsages()
	if flagUsages != "" {
		iostream.ErrPrintln(ctx, "\n\nFlags:")
		iostream.ErrPrint(ctx, iostream.Indent(dedent(flagUsages), "  "))
	}
	return nil
}

func nestedSuggest(command *cobra.Command, arg string) {
	ctx := command.Context()
	iostream.ErrPrintf(ctx, "unknown command %q for %q\n", arg, command.CommandPath())

	var candidates []string
	if arg == "help" {
		candidates = []string{"--help"}
	} else {
		if command.SuggestionsMinimumDistance <= 0 {
			command.SuggestionsMinimumDistance = 2
		}
		candidates = command.SuggestionsFor(arg)
	}

	if len(candidates) > 0 {
		iostream.ErrPrintln(ctx, "\nDid you mean this?")
		for _, c := range candidates {
			iostream.ErrPrintf(ctx, "\t%s\n", c)
		}
	}

	iostream.ErrPrintln(ctx)
	_ = rootUsage(command)
}

func isRootCmd(command *cobra.Command) bool {
	return command != nil && !command.HasParent()
}

func rootHelp(command *cobra.Command, _ []string) {
	ctx := command.Context()

	flags := command.Flags()

	if isRootCmd(command) {
		if versionVal, err := flags.GetBool("version"); err == nil && versionVal {
			_, _ = fmt.Fprint(iostream.Out(ctx), command.Annotations["versionInfo"])
			return
		} else if err != nil {
			iostream.ErrPrintln(ctx, err)
			return
		}
	}

	if help, _ := flags.GetBool("help"); !help && !command.Runnable() && len(flags.Args()) > 0 {
		nestedSuggest(command, flags.Args()[0])
		return
	}

	var md strings.Builder
	md.WriteString("# CircleCI CLI\n\n")

	section := func(title, body string) {
		body = strings.Trim(body, "\r\n")
		if body == "" {
			return
		}
		if title != "" {
			_, _ = fmt.Fprintf(&md, "## %s\n\n", title)
		}
		_, _ = fmt.Fprintf(&md, "%s\n\n", body)
	}

	longText := command.Long
	if longText == "" {
		longText = command.Short
	}
	if longText != "" && command.LocalFlags().Lookup("jq") != nil {
		longText = strings.TrimRight(longText, "\n") +
			"\n\nFor more information about output formatting flags, see `circleci help formatting`."
	}
	section("", longText)

	section("Usage", "`"+command.UseLine()+"`")

	if len(command.Aliases) > 0 {
		aliases := buildAliasList(command, command.Aliases)
		for i, a := range aliases {
			aliases[i] = "`" + a + "`"
		}
		section("Aliases", strings.Join(aliases, ", "))
	}

	for _, g := range groupedCommands(command) {
		var rows [][2]string
		for _, c := range g.Commands {
			rows = append(rows, [2]string{"`" + c.Name() + "`", c.Short})
		}
		section(titleCase(g.Title), mdTable("Command", "Description", rows))
	}

	if isRootCmd(command) {
		section("Help Topics", topicsMarkdown(helpTopics))
	}

	section("Flags", mdTable("Flag", "Description", flagRows(command.LocalFlags())))
	section("Inherited Flags", mdTable("Flag", "Description", flagRows(command.InheritedFlags())))

	section("Arguments", command.Annotations["help:arguments"])
	if command.Example != "" {
		section("Examples", exampleMarkdown(command.Example))
	}
	section("Environment Variables", command.Annotations["help:environment"])

	section("Learn More", heredoc.Docf(`
		- Use %[1]scircleci <command> <subcommand> --help%[1]s for more information about a command.
		- Read the manual at <https://circleci-public.github.io/circleci-cli>
		- Support at <https://github.com/CircleCI-Public/circleci-cli/issues>
	`, "`"))

	iostream.PrintMarkdown(ctx, md.String())
}

// mdTable renders rows as a GitHub-flavored markdown table. Returns "" when
// there are no rows so the caller can skip the section entirely.
//
//nolint:unparam
func mdTable(col1, col2 string, rows [][2]string) string {
	if len(rows) == 0 {
		return ""
	}
	t := mdtable.New(col1, col2)
	for _, r := range rows {
		t.Row(escapeTableCell(r[0]), escapeTableCell(r[1]))
	}
	return t.Render()
}

// titleCase capitalizes the first letter of each space-separated word, e.g.
// "Available commands" -> "Available Commands".
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// escapeTableCell flattens a value to a single table cell: newlines become
// spaces and pipes are escaped so they are not read as column separators.
func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.TrimSpace(s)
}

// flagRows turns a flag set into (name, description) table rows, mirroring
// pflag's own FlagUsages formatting (shorthand, value placeholder, default).
func flagRows(fs *pflag.FlagSet) [][2]string {
	var rows [][2]string
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" && f.ShorthandDeprecated == "" {
			name = "-" + f.Shorthand + ", " + name
		}
		varname, usage := pflag.UnquoteUsage(f)
		if varname != "" {
			name += " " + varname
		}
		if !flagDefaultIsZero(f) {
			if f.Value.Type() == "string" {
				usage += fmt.Sprintf(" (default %q)", f.DefValue)
			} else {
				usage += fmt.Sprintf(" (default %s)", f.DefValue)
			}
		}
		rows = append(rows, [2]string{"`" + name + "`", usage})
	})
	return rows
}

// flagDefaultIsZero reports whether a flag's default is its type's zero value,
// in which case pflag omits the "(default ...)" note. Replicates pflag's
// unexported defaultIsZeroValue for the value types this CLI uses.
func flagDefaultIsZero(f *pflag.Flag) bool {
	switch f.Value.Type() {
	case "bool", "boolfunc":
		return f.DefValue == "false"
	case "duration":
		return f.DefValue == "0" || f.DefValue == "0s"
	case "string":
		return f.DefValue == ""
	case "ip", "ipMask", "ipNet":
		return f.DefValue == "<nil>"
	case "intSlice", "stringSlice", "stringArray":
		return f.DefValue == "[]"
	default:
		switch f.DefValue {
		case "false", "<nil>", "", "0":
			return true
		}
		return false
	}
}

// topicsMarkdown renders the help topics as a markdown table of topic name to
// its short description, sorted by name. Each name is shown as inline code so
// it reads as the value to pass to `circleci help <topic>`.
func topicsMarkdown(topics []helpTopic) string {
	sorted := slices.Clone(topics)
	slices.SortFunc(sorted, func(a, b helpTopic) int {
		return strings.Compare(a.name, b.name)
	})

	rows := make([][2]string, 0, len(sorted))
	for _, t := range sorted {
		rows = append(rows, [2]string{"`" + t.name + "`", t.short})
	}
	return mdTable("Topic", "Description", rows)
}

// exampleMarkdown converts the conventional "# comment" / "$ command" example
// format into a markdown bullet list: each command becomes inline code,
// prefixed by its preceding comment when present.
func exampleMarkdown(example string) string {
	var b strings.Builder
	comment := ""
	for _, line := range strings.Split(example, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			// Blank line separates example blocks; nothing to emit.
		case strings.HasPrefix(t, "#"):
			comment = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		case strings.HasPrefix(t, "$"):
			cmd := strings.TrimSpace(strings.TrimPrefix(t, "$"))
			if comment != "" {
				_, _ = fmt.Fprintf(&b, "- %s: \n  `%s`\n", comment, cmd)
			} else {
				_, _ = fmt.Fprintf(&b, "- `%s`\n", cmd)
			}
			comment = ""
		default:
			// Freeform line (e.g. sample output): keep it as plain text.
			_, _ = fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	return b.String()
}

type commandGroup struct {
	Title    string
	Commands []*cobra.Command
}

func groupedCommands(cmd *cobra.Command) []commandGroup {
	var res []commandGroup

	for _, g := range cmd.Groups() {
		var cmds []*cobra.Command
		for _, c := range cmd.Commands() {
			if c.GroupID == g.ID && c.IsAvailableCommand() {
				cmds = append(cmds, c)
			}
		}
		if len(cmds) > 0 {
			res = append(res, commandGroup{
				Title:    g.Title,
				Commands: cmds,
			})
		}
	}

	var cmds []*cobra.Command
	for _, c := range cmd.Commands() {
		if c.GroupID == "" && c.IsAvailableCommand() {
			cmds = append(cmds, c)
		}
	}
	if len(cmds) > 0 {
		defaultGroupTitle := "Additional commands"
		if len(cmd.Groups()) == 0 {
			defaultGroupTitle = "Available commands"
		}
		res = append(res, commandGroup{
			Title:    defaultGroupTitle,
			Commands: cmds,
		})
	}

	return res
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		indent := len(l) - len(strings.TrimLeft(l, " "))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	var buf bytes.Buffer
	for _, l := range lines {
		_, _ = fmt.Fprintln(&buf, strings.TrimPrefix(l, strings.Repeat(" ", minIndent)))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func buildAliasList(cmd *cobra.Command, aliases []string) []string {
	if !cmd.HasParent() {
		return aliases
	}

	parentAliases := slices.Clone(cmd.Parent().Aliases)
	parentAliases = append(parentAliases, cmd.Parent().Name())
	sort.Strings(parentAliases)

	var aliasesWithParentAliases []string
	for _, alias := range aliases {
		for _, parentAlias := range parentAliases {
			aliasesWithParentAliases = append(aliasesWithParentAliases, fmt.Sprintf("%s %s", parentAlias, alias))
		}
	}

	return buildAliasList(cmd.Parent(), aliasesWithParentAliases)
}
