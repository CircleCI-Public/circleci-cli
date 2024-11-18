package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

func TestLocalPipelineValues(t *testing.T) {
	tests := []struct {
		name       string
		parameters Parameters
		wantKeys   []string
	}{
		{
			name:       "standard values given nil parameters",
			parameters: nil,
			wantKeys: []string{
				"pipeline.id",
				"pipeline.number",
				"pipeline.project.git_url",
				"pipeline.project.type",
				"pipeline.git.tag",
				"pipeline.git.branch",
				"pipeline.git.revision",
				"pipeline.git.base_revision",
				"pipeline.git.branch.is_default",
				"pipeline.trigger_parameters.circleci.event_time",
				"pipeline.trigger_parameters.webhook.body",
				"pipeline.trigger_parameters.github_app.branch",
				"pipeline.trigger_parameters.github_app.checkout_sha",
				"pipeline.trigger_parameters.github_app.commit_sha",
				"pipeline.trigger_parameters.github_app.commit_title",
				"pipeline.trigger_parameters.github_app.commit_message",
				"pipeline.trigger_parameters.github_app.commit_timestamp",
				"pipeline.trigger_parameters.github_app.commit_author_name",
				"pipeline.trigger_parameters.github_app.ref",
				"pipeline.trigger_parameters.github_app.repo_name",
				"pipeline.trigger_parameters.github_app.repo_url",
				"pipeline.trigger_parameters.github_app.total_commits_count",
				"pipeline.trigger_parameters.github_app.user_avatar",
				"pipeline.trigger_parameters.github_app.user_id",
				"pipeline.trigger_parameters.github_app.user_name",
				"pipeline.trigger_parameters.github_app.user_username",
				"pipeline.trigger_parameters.github_app.web_url",
				"pipeline.trigger_parameters.gitlab.commit_sha",
				"pipeline.trigger_parameters.gitlab.default_branch",
				"pipeline.trigger_parameters.gitlab.x_gitlab_event_id",
				"pipeline.trigger_parameters.gitlab.is_fork_merge_request",
			},
		},
		{
			name:       "standard and prefixed parameters given map of parameters",
			parameters: Parameters{"foo": "bar", "baz": "buzz"},
			wantKeys: []string{
				"pipeline.id",
				"pipeline.number",
				"pipeline.project.git_url",
				"pipeline.project.type",
				"pipeline.git.tag",
				"pipeline.git.branch",
				"pipeline.git.revision",
				"pipeline.git.base_revision",
				"pipeline.parameters.foo",
				"pipeline.parameters.baz",
				"pipeline.git.branch.is_default",
				"pipeline.trigger_parameters.circleci.event_time",
				"pipeline.trigger_parameters.webhook.body",
				"pipeline.trigger_parameters.github_app.branch",
				"pipeline.trigger_parameters.github_app.checkout_sha",
				"pipeline.trigger_parameters.github_app.commit_sha",
				"pipeline.trigger_parameters.github_app.commit_title",
				"pipeline.trigger_parameters.github_app.commit_message",
				"pipeline.trigger_parameters.github_app.commit_timestamp",
				"pipeline.trigger_parameters.github_app.commit_author_name",
				"pipeline.trigger_parameters.github_app.ref",
				"pipeline.trigger_parameters.github_app.repo_name",
				"pipeline.trigger_parameters.github_app.repo_url",
				"pipeline.trigger_parameters.github_app.total_commits_count",
				"pipeline.trigger_parameters.github_app.user_avatar",
				"pipeline.trigger_parameters.github_app.user_id",
				"pipeline.trigger_parameters.github_app.user_name",
				"pipeline.trigger_parameters.github_app.user_username",
				"pipeline.trigger_parameters.github_app.web_url",
				"pipeline.trigger_parameters.gitlab.commit_sha",
				"pipeline.trigger_parameters.gitlab.default_branch",
				"pipeline.trigger_parameters.gitlab.x_gitlab_event_id",
				"pipeline.trigger_parameters.gitlab.is_fork_merge_request",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, tt.wantKeys, maps.Keys(LocalPipelineValues(tt.parameters)), "LocalPipelineValues(%v)", tt.parameters)
		})
	}
}
