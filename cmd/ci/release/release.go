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

package main

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

const releaseBranch = "release/next"

func updateReleasePR(ctx context.Context, cfg config, gh *gitHub, current version, changelog string) error {
	prs, err := gh.mergedPRs(ctx, current.tag(), cfg.SHA, cfg.Base)
	if err != nil {
		return err
	}
	open, err := gh.openPR(ctx, releaseBranch)
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		iostream.InfoContext(ctx, "nothing merged since the last release", "version", current.tag())
		if open == 0 {
			return nil
		}
		if err := gh.closePR(ctx, open); err != nil {
			return err
		}
		return gh.deleteBranch(ctx, releaseBranch)
	}

	next := current.bump(bumpFor(prs))
	iostream.InfoContext(ctx, "updating the release PR", "prs", len(prs), "version", next.tag())
	notes, err := gh.generateNotes(ctx, next.tag(), current.tag(), cfg.SHA)
	if err != nil {
		return err
	}

	if err := pushReleaseBranch(ctx, cfg, gh, next, map[string]string{
		changelogFile: addRelease(changelog, next, cfg.now.UTC().Format("2006-01-02"), notes),
	}); err != nil {
		return err
	}

	pr := map[string]string{
		"title": "Release " + next.tag(),
		"body": fmt.Sprintf("Merging this releases %s to GitHub and every package manager.\n\n"+
			"A merged PR's `release:major` or `release:patch` label changes the version; "+
			"it's otherwise a minor release.\n\n"+
			"---\n\n%s", next.tag(), notes),
	}
	if open != 0 {
		return gh.updatePR(ctx, open, pr)
	}
	pr["head"], pr["base"] = releaseBranch, cfg.Base
	return gh.createPR(ctx, pr)
}

// pushReleaseBranch points the release branch at a commit on sha with the files changed. It's
// left alone if it already has them, so CI doesn't run again for the same change.
func pushReleaseBranch(ctx context.Context, cfg config, gh *gitHub, next version, files map[string]string) error {
	base, err := gh.commitTree(ctx, cfg.SHA)
	if err != nil {
		return err
	}
	tree, err := gh.createTree(ctx, base, files)
	if err != nil {
		return err
	}
	existing, err := gh.branch(ctx, releaseBranch)
	if err != nil {
		return err
	}
	if existing != "" {
		existingTree, err := gh.commitTree(ctx, existing)
		if err != nil {
			return err
		}
		if existingTree == tree {
			iostream.InfoContext(ctx, "release branch is up to date", "branch", releaseBranch)
			return nil
		}
	}
	commit, err := gh.createCommit(ctx, "Release "+next.tag(), tree, cfg.SHA)
	if err != nil {
		return err
	}
	return gh.setBranch(ctx, releaseBranch, commit, existing != "")
}
