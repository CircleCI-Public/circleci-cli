package config

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/git"
)

// CircleCI provides various `<< pipeline.x >>` values to be used in your config, but sometimes we need to fabricate those values when validating config.
type Values map[string]string

// Static typing is bypassed using an empty interface here due to pipeline parameters supporting multiple types.
type Parameters map[string]interface{}

// vars should contain any pipeline parameters that should be accessible via
// << pipeline.parameters.foo >>
func LocalPipelineValues() Values {
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

	vals := map[string]string{
		"id":                "00000000-0000-0000-0000-000000000001",
		"number":            "1",
		"project.git_url":   gitUrl,
		"project.type":      projectType,
		"git.tag":           git.Tag(),
		"git.branch":        git.Branch(),
		"git.revision":      revision,
		"git.base_revision": revision,
	}

	return vals
}
