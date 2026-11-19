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
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/update"
)

// validReferenceName is the shape a subcommand or flag name read from a
// reference must have to become a command or flag. The reference comes from
// the registry or a manifest on disk.
var validReferenceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// PrepareForMCP readies the extension commands under root for the MCP server.
// It refreshes the cached reference of each official extension that is not
// installed, then gives every extension command with a reference one child per
// documented subcommand, so the server exposes each as its own tool with typed
// flags rather than one tool taking opaque args.
func PrepareForMCP(ctx context.Context, root *cobra.Command) {
	for _, cmd := range root.Commands() {
		binaryName, ok := cmd.Annotations[extension.ReferenceAnnotation]
		// A command that already has children was prepared by an earlier call.
		if !ok || cmd.HasSubCommands() {
			continue
		}

		notInstalled := cmd.Annotations[extension.NotInstalledAnnotation] == "true"
		if notInstalled {
			refreshReference(ctx, binaryName)
		}

		ref, err := extension.LookupReference(binaryName)
		if err != nil || ref == nil {
			continue
		}

		// The placeholder's Long was built before the refresh above.
		if notInstalled {
			cmd.Long = promptLong(cmd.Name(), ref)
		}

		addReferenceCmds(cmd, ref)
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

// refreshReference fetches the registry's reference for binaryName, an official
// extension that is not installed, when the cached one is missing or older than
// update.CacheWindow. It is best-effort: a failure keeps the cached reference
// and holds off the next attempt for failedFetchRetry.
func refreshReference(ctx context.Context, binaryName string) {
	cfg := cmdutil.GetConfig(ctx)
	if !update.ShouldFetchReference(cfg) {
		return
	}

	refsDir, err := config.ExtensionReferencesDir()
	if err != nil {
		return
	}

	cache := extension.NewReferenceCache(refsDir)
	cached, err := cache.Get(binaryName)
	if err == nil && (time.Since(cached.FetchedAt) < update.CacheWindow || time.Since(cached.FailedAt) < failedFetchRetry) {
		return
	}

	m := extension.NewManager(extension.Config{
		Version: cmdutil.GetVersion(ctx),
		Agent:   cmdutil.GetAgentName(ctx),
		BaseURL: cfg.EffectiveExtensionHost(),
	})

	fetchCtx, cancel := context.WithTimeout(ctx, updateFetchTimeout)
	latest, err := m.Get(fetchCtx, binaryName)
	cancel()
	if err != nil {
		iostream.DebugContext(ctx, "extension reference: fetch failed", "extension", binaryName, "err", err)
		// Keep any reference already cached; only the retry is held off.
		cached.FailedAt = time.Now()
		if err := cache.Put(binaryName, cached); err != nil {
			iostream.DebugContext(ctx, "extension reference: could not cache", "extension", binaryName, "err", err)
		}
		return
	}

	err = cache.Put(binaryName, extension.CachedReference{
		Version:   latest.Version,
		FetchedAt: time.Now(),
		Ref:       latest.Ref,
	})
	if err != nil {
		iostream.DebugContext(ctx, "extension reference: could not cache", "extension", binaryName, "err", err)
	}
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
