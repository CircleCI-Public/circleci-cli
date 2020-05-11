package pipeline

import "sort"

// CircleCI provides various `<< pipeline.x >>` values to be used in your config, but sometimes we need to fabricate those values when validating config.
type Values map[string]string


func FabricatedValues() Values {
	return map[string]string{
		"id":     "00000000-0000-0000-0000-000000000001",
		"number": "1",
		// TODO: Could these be grabbed from git?
		"project.git_url":   "https://test.vcs/test/test",
		"project.type":      "vcs_type",
		"git.tag":           "test_git_tag",
		"git.branch":        "test_git_branch",
		"git.revision":      "0123456789abcdef0123456789abcdef0123",
		"git.base_revision": "0123456789abcdef0123456789abcdef0123",
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
