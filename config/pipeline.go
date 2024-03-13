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
		"pipeline.id":                "00000000-0000-0000-0000-000000000001",
		"pipeline.number":            1,
		"pipeline.project.git_url":   gitUrl,
		"pipeline.project.type":      projectType,
		"pipeline.git.tag":           git.Tag(),
		"pipeline.git.branch":        git.Branch(),
		"pipeline.git.revision":      revision,
		"pipeline.git.base_revision": revision,
	}

	for k, v := range parameters {
		vals[fmt.Sprintf("pipeline.parameters.%s", k)] = v
	}

	return vals
}
