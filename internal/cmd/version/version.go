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

// Package version implements the "circleci version" command.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/update"
)

type versionInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Modified bool   `json:"modified"`
	// Latest and Outdated are pointers so that an unknown answer is JSON null,
	// which a caller can tell apart from "up to date".
	Latest   *string `json:"latest"`
	Outdated *bool   `json:"outdated"`
}

func readBuildInfo(version string) versionInfo {
	info := versionInfo{Version: version, Commit: "unknown"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}

// checkLatest reports the newest release and whether this build is behind it.
// Both are nil when that is unknown: a dev build, update checks turned off, or no
// release could be fetched (offline, or a host that does not publish releases).
func checkLatest(ctx context.Context, version string) (latest *string, outdated *bool) {
	current := update.EffectiveVersion(version)
	if current == "" || current == "dev" {
		return nil, nil
	}
	if cfg := cmdutil.GetConfig(ctx); cfg == nil || !cfg.IsUpdateCheck() {
		return nil, nil
	}
	statePath, err := config.StatePath()
	if err != nil {
		return nil, nil
	}

	rel := update.Latest(ctx, update.NewProxySource(cmdutil.LoadClientOptionalAuth(ctx)), statePath)
	if rel == nil {
		return nil, nil
	}
	behind := update.IsNewer(rel.Version, current)
	return &rel.Version, &behind
}

// NewVersionCmd returns the "circleci version" command.
func NewVersionCmd(version string) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long: heredoc.Docf(`
			Print the version and commit hash this binary was built from. With %[1]s--json%[1]s it
			also reports the newest release, asking CircleCI at most once a day. Turn that off
			with %[1]scircleci setting set update-check off%[1]s.

			JSON fields: version (release tag, or "dev" for unreleased builds), commit (full git
			hash), modified (true when built from a dirty working tree), latest (newest release),
			outdated (true when latest is newer). latest and outdated are null when unknown: a dev
			build, update checks off, or offline.
		`, "`"),
		Example: heredoc.Doc(`
			# Print version and commit hash
			$ circleci version

			# Print as JSON
			$ circleci version --json

			# Extract just the commit hash
			$ circleci version --json | jq -r .commit

			# Check whether a newer release is available
			$ circleci version --json | jq .outdated
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			info := readBuildInfo(version)

			if jsonOut {
				if skip, _ := cmd.Root().Flags().GetBool("skip-update-check"); !skip {
					info.Latest, info.Outdated = checkLatest(ctx, version)
				}
				b, _ := json.MarshalIndent(info, "", "  ")
				_, _ = fmt.Fprintln(iostream.Out(ctx), string(b))
				return nil
			}

			commit := info.Commit
			if len(commit) > 12 {
				commit = commit[:12]
			}
			if info.Modified {
				commit += " (modified)"
			}
			_, _ = fmt.Fprintf(iostream.Out(ctx), "circleci %s (%s)\n", info.Version, commit)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON (fields: version, commit, modified, latest, outdated)")
	return cmd
}
