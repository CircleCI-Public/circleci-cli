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

// Command release keeps the release PR up to date, and prints the release notes for the version
// being published. See RELEASE.md.
//
// The newest section of CHANGELOG.md is the latest release. Once it's tagged, every push to main
// opens or updates the "Release vX.Y.Z" PR from the release/next branch, which adds a section for
// the next version with the PRs merged since. Merging that PR leaves the newest section untagged,
// which is what tells the deploy job to publish it.
//
// Usage:
//
//	GITHUB_TOKEN=... CIRCLE_SHA1=... go run ./cmd/ci/release pr
//	go run ./cmd/ci/release notes
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/iostreamcobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "release <command>",
		Short:        "Update the release PR, or print a release's notes",
		SilenceUsage: true,
	}
	cmd.PersistentFlags().Bool("debug", false, "log HTTP requests to stderr")
	cmd.AddCommand(prCmd(), notesCmd())
	return cmd
}

type config struct {
	API  string
	Repo string
	Base string
	SHA  string
	now  time.Time
}

func prCmd() *cobra.Command {
	cfg := config{SHA: os.Getenv("CIRCLE_SHA1")}
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Open, update or close the release PR for the PRs merged since the last release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostreamcobra.FromCmd(cmd.Context(), cmd, "")
			if cfg.SHA == "" {
				return errors.New("--sha or CIRCLE_SHA1 must be set")
			}
			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return errors.New("GITHUB_TOKEN must be set")
			}
			cfg.now = time.Now()
			return runPR(ctx, cfg, newGitHub(cfg.API, cfg.Repo, token))
		},
	}
	cmd.Flags().StringVar(&cfg.API, "api", "https://api.github.com", "GitHub API URL")
	cmd.Flags().StringVar(&cfg.Repo, "repo", "CircleCI-Public/circleci-cli", "repository to release")
	cmd.Flags().StringVar(&cfg.Base, "base", "main", "branch releases are made from")
	cmd.Flags().StringVar(&cfg.SHA, "sha", cfg.SHA, "commit being built (default $CIRCLE_SHA1)")
	return cmd
}

func notesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notes",
		Short: "Print the newest CHANGELOG.md section: the release notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostreamcobra.FromCmd(cmd.Context(), cmd, "")
			v, changelog, err := readChangelog()
			if err != nil {
				return err
			}
			notes, err := releaseNotes(changelog, v)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(iostream.Out(ctx), notes)
			return err
		},
	}
}

func readChangelog() (version, string, error) {
	b, err := os.ReadFile(changelogFile)
	if err != nil {
		return version{}, "", err
	}
	v, err := latestVersion(string(b))
	return v, string(b), err
}

// runPR updates the release PR, unless the newest version in the changelog isn't tagged yet: then
// its release PR has just been merged, and the deploy job is publishing it.
func runPR(ctx context.Context, cfg config, gh *gitHub) error {
	v, changelog, err := readChangelog()
	if err != nil {
		return err
	}
	tagged, err := gh.tagExists(ctx, v.tag())
	if err != nil {
		return err
	}
	if !tagged {
		iostream.InfoContext(ctx, "not yet released; the deploy job publishes it", "version", v.tag())
		return nil
	}
	return updateReleasePR(ctx, cfg, gh, v, changelog)
}
