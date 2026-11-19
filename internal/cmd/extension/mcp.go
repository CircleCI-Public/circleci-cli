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

package extension

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
)

// validReferenceName is the shape a subcommand or flag name read from a
// reference must have to become a command or flag. The reference is read from
// a manifest on disk, so names are checked rather than trusted.
var validReferenceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// PrepareForMCP readies the extension commands under root for the MCP server:
// every installed extension with a reference gets one child per documented
// subcommand, so the server exposes each as its own tool with typed flags
// rather than one tool taking opaque args.
//
// The children only shape the tool schemas. The server runs every tool call as
// a fresh circleci subprocess, whose tree has no such children, so the call
// reaches the extension through the usual pass-through.
func PrepareForMCP(root *cobra.Command) {
	extsDir, err := config.ExtensionsDir()
	if err != nil {
		return
	}
	store := extension.NewStore(extsDir)

	for _, cmd := range root.Commands() {
		binaryName, ok := cmd.Annotations[extension.ReferenceAnnotation]
		// A command that already has children was prepared by an earlier call.
		if !ok || cmd.HasSubCommands() {
			continue
		}

		manifest, err := store.Get(binaryName)
		if err != nil || manifest.Ref == nil {
			continue
		}

		addReferenceCmds(cmd, manifest.Ref)
	}
}

// IsExtensionCmd reports whether cmd runs an extension: a top-level extension
// command, or a child added to one by PrepareForMCP.
func IsExtensionCmd(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.GroupID == "extension" && c.DisableFlagParsing {
			return true
		}
	}
	return false
}

// addReferenceCmds adds a child of parent for each subcommand ref documents.
func addReferenceCmds(parent *cobra.Command, ref *extension.Reference) {
	for _, sub := range ref.Subcommands {
		if !validReferenceName.MatchString(sub.Name) || hasChild(parent, sub.Name) {
			continue
		}

		child := &cobra.Command{
			Use:                strings.TrimSpace(sub.Name + " " + sub.Use),
			Short:              sub.Short,
			Long:               sub.Long,
			DisableFlagParsing: true,
			RunE:               parent.RunE,
		}

		if len(sub.Args) > 0 {
			var args strings.Builder
			for _, a := range sub.Args {
				args.WriteString("`" + a.Name + "` " + a.Help + "\n")
			}
			child.Annotations = map[string]string{"help:arguments": args.String()}
		}

		// Flags declared on the extension itself, such as testsuite's --local,
		// are accepted after any of its subcommands.
		addReferenceFlags(child, ref.Flags)
		addReferenceFlags(child, sub.Flags)

		parent.AddCommand(child)
	}
}

// addReferenceFlags defines each documented flag on cmd: a bool when the
// reference gives no value type, a string otherwise, with any non-string type
// (an enum such as "all|impacted|none") kept in the usage.
func addReferenceFlags(cmd *cobra.Command, flags []extension.ReferenceFlag) {
	for _, f := range flags {
		if !validReferenceName.MatchString(f.Name) || cmd.Flags().Lookup(f.Name) != nil {
			continue
		}

		if f.Type == "" {
			// shortcut: a bool defaulting to true cannot be turned off through MCP,
			// since the server drops false flags; no extension declares one yet —
			// pass such a flag as --name=false when one does.
			cmd.Flags().Bool(f.Name, f.Default == "true", f.Usage)
			continue
		}

		usage := f.Usage
		if f.Type != "string" {
			usage += " (" + f.Type + ")"
		}
		cmd.Flags().String(f.Name, f.Default, usage)
	}
}

func hasChild(cmd *cobra.Command, name string) bool {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}
