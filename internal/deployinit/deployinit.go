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

// Package deployinit implements scanning and patching of .circleci/config.yml
// to add deploy marker steps for circleci deploy init.
package deployinit

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// JobInfo describes a deploy-like job detected in the config.
type JobInfo struct {
	Name        string
	Branch      string // from workflow branch filter; "" means all/unknown
	InferredEnv string // environment guessed from job name; "" if unknown
}

// DetectResult holds the jobs found in the config.
type DetectResult struct {
	Jobs       []JobInfo
	HasDeploys bool // config already uses circleci/deploys orb
}

var deployPattern = regexp.MustCompile(`(?i)(^|[-_])(deploy|release|publish|ship)([-_]|$)`)

var envPatterns = []struct {
	re  *regexp.Regexp
	env string
}{
	{regexp.MustCompile(`(?i)(^|[-_])(prod|production)([-_]|$)`), "production"},
	{regexp.MustCompile(`(?i)(^|[-_])(staging|stage)([-_]|$)`), "staging"},
	{regexp.MustCompile(`(?i)(^|[-_])(dev|development)([-_]|$)`), "development"},
	{regexp.MustCompile(`(?i)(^|[-_])(qa|test|testing)([-_]|$)`), "testing"},
	{regexp.MustCompile(`(?i)(^|[-_])(demo)([-_]|$)`), "demo"},
}

// InferEnvironment returns an environment name from a job name, or "".
func InferEnvironment(jobName string) string {
	for _, p := range envPatterns {
		if p.re.MatchString(jobName) {
			return p.env
		}
	}
	return ""
}

// IsDeployJob returns true if jobName looks like a deploy job.
func IsDeployJob(name string) bool {
	return deployPattern.MatchString(name)
}

// Detect reads configPath and returns detected deploy jobs.
func Detect(configPath string) (*DetectResult, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	if len(root.Content) == 0 {
		return &DetectResult{}, nil
	}
	doc := root.Content[0]

	jobNames := extractJobNames(doc)
	branchForJob := extractBranchFilters(doc)

	result := &DetectResult{
		HasDeploys: hasDeploysOrb(doc),
	}
	for _, name := range jobNames {
		if !IsDeployJob(name) {
			continue
		}
		result.Jobs = append(result.Jobs, JobInfo{
			Name:        name,
			Branch:      branchForJob[name],
			InferredEnv: InferEnvironment(name),
		})
	}
	return result, nil
}

// hasDeploysOrb returns true if the config already uses circleci/deploys.
func hasDeploysOrb(doc *yaml.Node) bool {
	orbsNode := findMappingValue(doc, "orbs")
	if orbsNode == nil {
		return false
	}
	for i := 1; i < len(orbsNode.Content); i += 2 {
		if strings.HasPrefix(orbsNode.Content[i].Value, "circleci/deploys") {
			return true
		}
	}
	return false
}

func extractJobNames(doc *yaml.Node) []string {
	jobsNode := findMappingValue(doc, "jobs")
	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return nil
	}
	var names []string
	for i := 0; i+1 < len(jobsNode.Content); i += 2 {
		names = append(names, jobsNode.Content[i].Value)
	}
	return names
}

func extractBranchFilters(doc *yaml.Node) map[string]string {
	result := make(map[string]string)
	workflowsNode := findMappingValue(doc, "workflows")
	if workflowsNode == nil || workflowsNode.Kind != yaml.MappingNode {
		return result
	}
	for i := 0; i+1 < len(workflowsNode.Content); i += 2 {
		key := workflowsNode.Content[i].Value
		if key == "version" {
			continue
		}
		wfNode := resolve(workflowsNode.Content[i+1])
		jobsSeq := findMappingValue(wfNode, "jobs")
		if jobsSeq == nil || jobsSeq.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range jobsSeq.Content {
			name, branch := jobFilterFromItem(resolve(item))
			if name != "" && branch != "" {
				result[name] = branch
			}
		}
	}
	return result
}

func jobFilterFromItem(item *yaml.Node) (name, branch string) {
	switch item.Kind {
	case yaml.ScalarNode:
		return item.Value, ""
	case yaml.MappingNode:
		if len(item.Content) < 2 {
			return "", ""
		}
		name = item.Content[0].Value
		cfg := resolve(item.Content[1])
		if cfg.Kind != yaml.MappingNode {
			return name, ""
		}
		filtersNode := findMappingValue(cfg, "filters")
		if filtersNode == nil {
			return name, ""
		}
		branchesNode := findMappingValue(filtersNode, "branches")
		if branchesNode == nil {
			return name, ""
		}
		onlyNode := findMappingValue(branchesNode, "only")
		if onlyNode == nil {
			return name, ""
		}
		switch onlyNode.Kind {
		case yaml.ScalarNode:
			return name, onlyNode.Value
		case yaml.SequenceNode:
			if len(onlyNode.Content) > 0 {
				return name, onlyNode.Content[0].Value
			}
		}
	}
	return "", ""
}

func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func resolve(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

// PatchJob describes a job to instrument.
type PatchJob struct {
	Name        string
	Environment string
}

// PatchInput describes what to add to the config.
type PatchInput struct {
	Component string
	Jobs      []PatchJob
}

// Patch modifies configPath in-place, returning true if any changes were made.
// It is idempotent: if the deploy step is already present it is not duplicated.
func Patch(configPath string, input PatchInput, useOrb bool) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse %s: %w", configPath, err)
	}
	if len(root.Content) == 0 {
		return false, fmt.Errorf("empty config")
	}
	doc := root.Content[0]
	jobsNode := findMappingValue(doc, "jobs")
	if jobsNode == nil {
		return false, nil
	}

	changed := false
	for _, pj := range input.Jobs {
		jobNode := findMappingValue(jobsNode, pj.Name)
		if jobNode == nil {
			continue
		}
		jobNode = resolve(jobNode)
		stepsNode := findMappingValue(jobNode, "steps")
		if stepsNode == nil || stepsNode.Kind != yaml.SequenceNode {
			continue
		}
		if alreadyPatched(stepsNode) {
			continue
		}
		var step *yaml.Node
		if useOrb {
			step = makeOrbStep(input.Component, pj.Environment)
		} else {
			step = makeRunStep(input.Component, pj.Environment)
		}
		stepsNode.Content = append(stepsNode.Content, step)
		changed = true
	}

	if !changed {
		return false, nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return false, err
	}
	if err := os.WriteFile(configPath, buf.Bytes(), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// alreadyPatched returns true if stepsNode already contains a deploy marker step.
func alreadyPatched(stepsNode *yaml.Node) bool {
	for _, step := range stepsNode.Content {
		step = resolve(step)
		if step.Kind != yaml.MappingNode {
			continue
		}
		// raw run step
		runNode := findMappingValue(step, "run")
		if runNode != nil {
			runNode = resolve(runNode)
			if runNode.Kind == yaml.MappingNode {
				cmdNode := findMappingValue(runNode, "command")
				if cmdNode != nil && strings.Contains(cmdNode.Value, "circleci run release log") {
					return true
				}
			}
		}
		// orb step: deploys/log
		for i := 0; i < len(step.Content); i++ {
			if step.Content[i].Value == "deploys/log" {
				return true
			}
		}
	}
	return false
}

func makeRunStep(component, env string) *yaml.Node {
	cmd := fmt.Sprintf("circleci run release log --component %q --environment %q", component, env)
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			scalar("run"),
			{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					scalar("name"), scalar("Log deploy marker"),
					scalar("command"), scalar(cmd),
				},
			},
		},
	}
}

func makeOrbStep(component, env string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			scalar("deploys/log"),
			{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					scalar("component"), scalar(component),
					scalar("environment"), scalar(env),
				},
			},
		},
	}
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: "!!str"}
}
