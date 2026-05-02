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
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

type versionInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Modified bool   `json:"modified"`
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

// NewVersionCmd returns the "circleci version" command.
func NewVersionCmd(version string) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long: heredoc.Doc(`
			Print the version and commit hash this binary was built from.

			JSON fields:
			  version   release tag (e.g. "v1.2.3") or "dev" for unreleased builds
			  commit    full git commit hash
			  modified  true if the binary was built from a dirty working tree
		`),
		Example: heredoc.Doc(`
			# Print version and commit hash
			$ circleci version

			# Print as JSON
			$ circleci version --json

			# Extract just the commit hash
			$ circleci version --json | jq -r .commit
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			info := readBuildInfo(version)

			if jsonOut {
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

	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON (fields: version, commit, modified)")
	return cmd
}
