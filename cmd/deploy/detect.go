package deploy

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// deployJobKeywords lists substrings that, when found in a job name,
// strongly suggest the job performs a deployment and is a candidate
// for adding a deploy marker step.
var deployJobKeywords = []string{
	"deploy",
	"release",
	"publish",
	"ship",
}

// DetectedJob describes a job in config.yml that matched the deploy heuristics.
type DetectedJob struct {
	// Name is the job name as it appears under the top-level `jobs:` key.
	Name string
	// AlreadyInstrumented is true when at least one of the job's existing
	// steps already invokes `circleci run release log` (or a compatible
	// marker command). Such jobs are skipped during patching to keep the
	// command idempotent.
	AlreadyInstrumented bool
}

// DetectDeployJobs scans the root YAML document of a CircleCI config and
// returns every job whose name matches the deploy heuristics. It never
// modifies the node tree.
func DetectDeployJobs(root *yaml.Node) []DetectedJob {
	jobsNode := findJobsNode(root)
	if jobsNode == nil {
		return nil
	}

	var detected []DetectedJob
	// A YAML mapping node stores keys and values as alternating entries in
	// Content; we iterate in pairs and skip entries where the value is not
	// itself a mapping (malformed configs, anchors resolved elsewhere, etc.)
	for i := 0; i+1 < len(jobsNode.Content); i += 2 {
		nameNode := jobsNode.Content[i]
		valueNode := jobsNode.Content[i+1]
		if nameNode.Kind != yaml.ScalarNode {
			continue
		}
		if !isDeployJobName(nameNode.Value) {
			continue
		}
		detected = append(detected, DetectedJob{
			Name:                nameNode.Value,
			AlreadyInstrumented: jobAlreadyInstrumented(valueNode),
		})
	}
	return detected
}

// findJobsNode returns the mapping node stored under the top-level `jobs:`
// key, or nil when the config has no jobs section.
func findJobsNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	doc := root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "jobs" {
			valueNode := doc.Content[i+1]
			if valueNode.Kind == yaml.MappingNode {
				return valueNode
			}
			return nil
		}
	}
	return nil
}

// isDeployJobName reports whether the supplied job name contains any of
// the deploy-related keywords, case-insensitively.
func isDeployJobName(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range deployJobKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// jobAlreadyInstrumented walks the supplied job mapping node looking for
// a `run` step whose command text includes `circleci run release log` or
// `circleci run release plan`. Those are the commands this tool adds (or
// that a user may have added themselves), so finding either means we
// should leave the job untouched.
func jobAlreadyInstrumented(jobNode *yaml.Node) bool {
	if jobNode == nil || jobNode.Kind != yaml.MappingNode {
		return false
	}
	stepsNode := findChild(jobNode, "steps")
	if stepsNode == nil || stepsNode.Kind != yaml.SequenceNode {
		return false
	}
	for _, step := range stepsNode.Content {
		if stepContainsDeployMarker(step) {
			return true
		}
	}
	return false
}

// stepContainsDeployMarker inspects a single step node to determine
// whether it already runs the deploy marker CLI. Steps can appear as a
// bare scalar (e.g. `- checkout`) or as a mapping with a `run` key
// holding either a scalar command or a nested mapping with
// `command:` / `name:` fields.
func stepContainsDeployMarker(step *yaml.Node) bool {
	switch step.Kind {
	case yaml.ScalarNode:
		return containsDeployMarker(step.Value)
	case yaml.MappingNode:
		run := findChild(step, "run")
		if run == nil {
			return false
		}
		switch run.Kind {
		case yaml.ScalarNode:
			return containsDeployMarker(run.Value)
		case yaml.MappingNode:
			cmd := findChild(run, "command")
			if cmd != nil && cmd.Kind == yaml.ScalarNode && containsDeployMarker(cmd.Value) {
				return true
			}
		}
	}
	return false
}

// containsDeployMarker checks whether a command string invokes the
// deploy marker CLI in any of its supported forms.
func containsDeployMarker(cmd string) bool {
	if cmd == "" {
		return false
	}
	normalized := strings.Join(strings.Fields(cmd), " ")
	return strings.Contains(normalized, "circleci run release log") ||
		strings.Contains(normalized, "circleci run release plan")
}

// findChild returns the first value node associated with the given key
// in a mapping node, or nil if the mapping does not contain that key.
func findChild(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// InferEnvironmentName returns a best-effort environment name derived
// from the job name (e.g. `deploy-prod` → `production`). The boolean
// return reports whether the heuristic matched; callers should prompt
// the user when it did not.
func InferEnvironmentName(jobName string) (string, bool) {
	lower := strings.ToLower(jobName)
	switch {
	case strings.Contains(lower, "production"), strings.Contains(lower, "prod"):
		return "production", true
	case strings.Contains(lower, "staging"), strings.Contains(lower, "stage"):
		return "staging", true
	case strings.Contains(lower, "development"), strings.Contains(lower, "dev"):
		return "development", true
	case strings.Contains(lower, "test"):
		return "test", true
	}
	return "", false
}
