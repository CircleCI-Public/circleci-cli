package deploy

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MarkerStep describes the deploy marker step we want to add to a single job.
type MarkerStep struct {
	JobName         string
	ComponentName   string
	EnvironmentName string
}

// PatchResult summarises what PatchConfig did so the caller can report
// useful feedback to the user.
type PatchResult struct {
	// Modified lists the names of jobs that had a new deploy marker step
	// appended to them in this invocation.
	Modified []string
	// Skipped lists jobs that were already instrumented and therefore
	// left untouched.
	Skipped []string
}

// ReadConfig reads the config file at path and parses it into a YAML
// Document node that preserves formatting, comments and ordering for a
// subsequent round-trip.
func ReadConfig(path string) ([]byte, *yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return data, &root, nil
}

// WriteConfig marshals the document node back to YAML and writes it to
// path. It uses two-space indentation to match the style used across
// CircleCI config examples.
func WriteConfig(path string, root *yaml.Node) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing encoder: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// PatchConfig mutates root so that each requested job gains a new step
// that records a deploy marker. Jobs that already contain such a step
// are left untouched so the command is safely re-runnable.
func PatchConfig(root *yaml.Node, steps []MarkerStep) (PatchResult, error) {
	jobsNode := findJobsNode(root)
	if jobsNode == nil {
		return PatchResult{}, fmt.Errorf("config has no top-level `jobs:` section")
	}

	var result PatchResult
	for _, step := range steps {
		jobNode := findChild(jobsNode, step.JobName)
		if jobNode == nil {
			return result, fmt.Errorf("job %q not found in config", step.JobName)
		}
		if jobAlreadyInstrumented(jobNode) {
			result.Skipped = append(result.Skipped, step.JobName)
			continue
		}
		if err := appendMarkerStep(jobNode, step); err != nil {
			return result, fmt.Errorf("patching job %q: %w", step.JobName, err)
		}
		result.Modified = append(result.Modified, step.JobName)
	}
	return result, nil
}

// appendMarkerStep adds a single `run` step to the `steps:` sequence of
// the supplied job. If the job has no `steps:` key the function returns
// an error rather than guessing where to insert one, since a job
// without steps is almost certainly a config error the user should see.
func appendMarkerStep(jobNode *yaml.Node, step MarkerStep) error {
	if jobNode.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node for job")
	}
	stepsNode := findChild(jobNode, "steps")
	if stepsNode == nil {
		return fmt.Errorf("job has no `steps:` section")
	}
	if stepsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("job `steps:` is not a sequence")
	}

	stepsNode.Content = append(stepsNode.Content, newMarkerStepNode(step))
	return nil
}

// newMarkerStepNode constructs the YAML node tree representing a single
// `run` step that invokes `circleci run release log`. Using an explicit
// mapping (rather than parsing a template string) lets us control
// indentation and style without relying on round-trip quirks.
//
// The generated step looks like:
//
//   - run:
//     name: Log deploy marker
//     command: |
//     circleci run release log \
//     --component-name=<name> \
//     --environment-name=<env> \
//     --target-version=$CIRCLE_SHA1
func newMarkerStepNode(step MarkerStep) *yaml.Node {
	commandLiteral := fmt.Sprintf(
		"circleci run release log \\\n  --component-name=%s \\\n  --environment-name=%s \\\n  --target-version=$CIRCLE_SHA1\n",
		step.ComponentName,
		step.EnvironmentName,
	)

	runMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			scalar("name"),
			scalar("Log deploy marker"),
			scalar("command"),
			literalBlock(commandLiteral),
		},
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			scalar("run"),
			runMapping,
		},
	}
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func literalBlock(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Style: yaml.LiteralStyle,
		Value: value,
	}
}
