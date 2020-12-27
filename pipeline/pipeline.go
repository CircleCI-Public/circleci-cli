package pipeline

import (
	"fmt"
	"sort"

	"github.com/CircleCI-Public/circleci-cli/git"
)

// CircleCI provides various `<< pipeline.x >>` values to be used in your config, but sometimes we need to fabricate those values when validating config.
type Values map[string]string

func FabricatedValues() Values {

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

	return map[string]string{
		"id":                "00000000-0000-0000-0000-000000000001",
		"number":            "1",
		"project.git_url":   gitUrl,
		"project.type":      projectType,
		"git.tag":           git.Tag(),
		"git.branch":        git.Branch(),
		"git.revision":      revision,
		"git.base_revision": revision,
	}
}

// TODO: type Parameters map[string]string

// KeyVal is a data structure specifically for passing pipeline data to GraphQL which doesn't support free-form maps.
type KeyVal struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

// PrepareForGraphQL takes a golang homogenous map, and transforms it into a list of keyval pairs, since GraphQL does not support homogenous maps.
func PrepareForGraphQL(kvMap Values) []KeyVal {
	// we need to create the slice of KeyVals in a deterministic order for testing purposes
	keys := make([]string, 0, len(kvMap))
	for k := range kvMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	kvs := make([]KeyVal, 0, len(kvMap))
	for _, k := range keys {
		kvs = append(kvs, KeyVal{Key: k, Val: kvMap[k]})
	}
	return kvs
}
