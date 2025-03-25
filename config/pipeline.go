package config

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/git"
)

// CircleCI provides various `<< pipeline.x >>` values to be used in your config, but sometimes we need to fabricate those values when validating config.
type Values map[string]interface{}

// Static typing is bypassed using an empty interface here due to pipeline parameters supporting multiple types.
type Parameters map[string]interface{}

// LocalPipelineValues returns a map of pipeline values that can be used for local validation.
// The given parameters will be prefixed with "pipeline.parameters." and accessible via << pipeline.parameters.foo >>.
func LocalPipelineValues(parameters Parameters) Values {
	revision := git.Revision()
	gitUrl := "https://github.com/CircleCI-Public/circleci-cli"
	projectType := "github"

	// If we encounter an error infering project, skip this and use defaults.
	if remote, err := git.InferProjectFromGitRemotes(); err == nil {
		switch remote.VcsType {
		case git.GitHub:
			gitUrl = fmt.Sprintf("https://github.com/%s/%s", remote.Organization, remote.Project)
			projectType = "github"
		case git.Bitbucket:
			gitUrl = fmt.Sprintf("https://bitbucket.org/%s/%s", remote.Organization, remote.Project)
			projectType = "bitbucket"
		}
	}

	vals := map[string]interface{}{
		"pipeline.id":                                                "00000000-0000-0000-0000-000000000001",
		"pipeline.number":                                            1,
		"pipeline.name":                                              "",
		"pipeline.project.git_url":                                   gitUrl,
		"pipeline.project.type":                                      projectType,
		"pipeline.git.tag":                                           git.Tag(),
		"pipeline.git.branch":                                        git.Branch(),
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
		"pipeline.git.repo_url":                                      gitUrl,
		"pipeline.git.ssh_checkout_url":                              "",
		"pipeline.trigger_parameters.circleci.event_time":            "2020-01-01T00:00:00Z",
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
		"pipeline.event.name":                                        "",
		"pipeline.event.action":                                      "",
	}

	for k, v := range parameters {
		vals[fmt.Sprintf("pipeline.parameters.%s", k)] = v
	}

	return vals
}
