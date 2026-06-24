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

// Package configcmd contains the business logic for circleci config subcommands.
package configcmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

// LocalPipelineValues fabricates the CircleCI pipeline values used when validating
// or processing a config locally. Git state is read best-effort; failures are silent.
// params are merged as pipeline.parameters.<key> and also returned separately so
// callers can pass them as PipelineParameters.
func LocalPipelineValues(params map[string]any) map[string]any {
	revision := gitHead()
	gitURL := ""
	projectType := "github"
	branch := ""

	if info, err := gitremote.DetectFromRemote(); err == nil {
		branch = info.Branch
		parts := strings.SplitN(info.Slug, "/", 3)
		if len(parts) == 3 {
			vcs, org, repo := parts[0], parts[1], parts[2]
			switch vcs {
			case "gh":
				gitURL = fmt.Sprintf("https://github.com/%s/%s", org, repo)
				projectType = "github"
			case "bb":
				gitURL = fmt.Sprintf("https://bitbucket.org/%s/%s", org, repo)
				projectType = "bitbucket"
			case "gl":
				gitURL = fmt.Sprintf("https://gitlab.com/%s/%s", org, repo)
				projectType = "gitlab"
			}
		}
	}

	tag := gitCurrentTag()

	vals := map[string]any{
		"pipeline.id":                                                "00000000-0000-0000-0000-000000000001",
		"pipeline.number":                                            1,
		"pipeline.name":                                              "",
		"pipeline.project.git_url":                                   gitURL,
		"pipeline.project.type":                                      projectType,
		"pipeline.git.tag":                                           tag,
		"pipeline.git.branch":                                        branch,
		"pipeline.git.revision":                                      revision,
		"pipeline.git.base_revision":                                 revision,
		"pipeline.git.branch.is_default":                             false,
		"pipeline.git.commit.author_avatar_url":                      "",
		"pipeline.git.commit.author_email":                           "",
		"pipeline.git.commit.author_login":                           "",
		"pipeline.git.commit.author_name":                            "",
		"pipeline.git.commit.body":                                   "",
		"pipeline.git.commit.subject":                                "",
		"pipeline.git.commit.url":                                    "",
		"pipeline.git.repo_id":                                       "",
		"pipeline.git.repo_name":                                     "",
		"pipeline.git.repo_owner":                                    "",
		"pipeline.git.repo_url":                                      gitURL,
		"pipeline.trigger_parameters.circleci.event_time":            "2020-01-01T00:00:00Z",
		"pipeline.trigger_parameters.circleci.trigger_type":          "github_app",
		"pipeline.trigger_parameters.circleci.event_type":            "push",
		"pipeline.trigger_parameters.webhook.body":                   "",
		"pipeline.trigger_parameters.github_app.branch":              "main",
		"pipeline.trigger_parameters.github_app.checkout_sha":        revision,
		"pipeline.trigger_parameters.github_app.commit_sha":          revision,
		"pipeline.trigger_parameters.github_app.commit_title":        "",
		"pipeline.trigger_parameters.github_app.commit_message":      "",
		"pipeline.trigger_parameters.github_app.commit_timestamp":    "2020-01-01T00:00:00Z",
		"pipeline.trigger_parameters.github_app.commit_author_name":  "",
		"pipeline.trigger_parameters.github_app.ref":                 "refs/heads/master",
		"pipeline.trigger_parameters.github_app.repo_name":           "",
		"pipeline.trigger_parameters.github_app.repo_url":            "",
		"pipeline.trigger_parameters.github_app.total_commits_count": 1,
		"pipeline.trigger_parameters.github_app.user_avatar":         "",
		"pipeline.trigger_parameters.github_app.user_id":             "00000000-0000-0000-0000-000000000001",
		"pipeline.trigger_parameters.github_app.user_name":           "",
		"pipeline.trigger_parameters.github_app.user_username":       "",
		"pipeline.trigger_parameters.github_app.web_url":             "",
		"pipeline.trigger_parameters.gitlab.commit_sha":              revision,
		"pipeline.trigger_parameters.gitlab.default_branch":          "main",
		"pipeline.trigger_parameters.gitlab.x_gitlab_event_id":       "00000000-0000-0000-0000-000000000001",
		"pipeline.trigger_parameters.gitlab.is_fork_merge_request":   false,
		"pipeline.trigger.type":                                      "",
		"pipeline.trigger.id":                                        "00000000-0000-0000-0000-000000000001",
		"pipeline.trigger.name":                                      "",
		"pipeline.trigger.received_at":                               "2020-01-01T00:00:00Z",
		"pipeline.trigger_source":                                    "webhook",
		"pipeline.schedule.name":                                     "",
		"pipeline.schedule.id":                                       "",
		"pipeline.event.name":                                        "",
		"pipeline.event.action":                                      "",
		"pipeline.event.context.github.pr_url":                       gitURL,
		"pipeline.event.github.repository.url":                       gitURL,
		"pipeline.event.github.repository.name":                      "repo-name",
		"pipeline.event.github.repository.owner.login":               "repo-owner",
		"pipeline.event.github.sender.avatar_url":                    "",
		"pipeline.event.github.sender.login":                         "user-login",
		"pipeline.event.github.ref":                                  "branch-name",
		"pipeline.event.github.after":                                revision,
		"pipeline.event.github.head_commit.url":                      "",
		"pipeline.event.github.pull_request.title":                   "",
		"pipeline.event.github.pull_request.url":                     gitURL,
		"pipeline.event.github.pull_request.head.sha":                revision,
		"pipeline.event.github.pull_request.base.sha":                revision,
		"pipeline.event.github.pull_request.base.ref":                "base-branch",
		"pipeline.event.github.pull_request.head.ref":                "head-branch",
		"pipeline.event.github.pull_request.merged":                  false,
		"pipeline.event.github.pull_request.number":                  111,
		"pipeline.event.github.pull_request.draft":                   false,
		"pipeline.event.github.label":                                "",
		"pipeline.config.file_path":                                  "",
		"pipeline.config.repository.url":                             gitURL,
		"pipeline.config.repository.name":                            "",
		"pipeline.config.sha":                                        revision,
		"pipeline.config.ref":                                        "refs/head/master",
		"pipeline.deploy.component_name":                             "",
		"pipeline.deploy.environment_name":                           "",
		"pipeline.deploy.target_version":                             "",
		"pipeline.deploy.current_version":                            "",
		"pipeline.deploy.namespace":                                  "",
		"pipeline.deploy.reason":                                     "",
		"pipeline.event.github.comment.body":                         "",
		"pipeline.event.github.sender.type":                          "",
	}

	for k, v := range params {
		vals["pipeline.parameters."+k] = v
	}

	return vals
}

// gitHead returns the full SHA of HEAD, or "" when git state is unreadable.
// Equivalent to `git rev-parse HEAD`.
func gitHead() string {
	repo, err := openGitRepo()
	if err != nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	return head.Hash().String()
}

// gitCurrentTag returns the first tag (in lexicographic order, as `git tag`
// sorts) pointing at HEAD, or "" when there is none or git state is unreadable.
// Equivalent to the first line of `git tag --points-at HEAD`.
func gitCurrentTag() string {
	repo, err := openGitRepo()
	if err != nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	iter, err := repo.Tags()
	if err != nil {
		return ""
	}

	var matches []string
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		// Lightweight tags point straight at the commit; annotated tags point at
		// a tag object that must be dereferenced to its target commit.
		target := ref.Hash()
		if tagObj, err := repo.TagObject(ref.Hash()); err == nil {
			target = tagObj.Target
		}
		if target == head.Hash() {
			matches = append(matches, ref.Name().Short())
		}
		return nil
	})
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// openGitRepo opens the repository containing the current working directory,
// walking up to find the .git dir (like the git CLI does).
func openGitRepo() (*git.Repository, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{DetectDotGit: true})
}
