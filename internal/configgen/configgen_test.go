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

package configgen

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

const jobName = "build-and-test"

type renderedStep struct {
	Run struct {
		Name    string `yaml:"name"`
		Command string `yaml:"command"`
	} `yaml:"run"`
}

type renderedConfig struct {
	Jobs map[string]struct {
		Docker []struct {
			Image string `yaml:"image"`
		} `yaml:"docker"`
		Steps []yaml.Node `yaml:"steps"`
	} `yaml:"jobs"`
	Workflows map[string]struct {
		Jobs []string `yaml:"jobs"`
	} `yaml:"workflows"`
}

// render runs renderConfig and returns the parsed config plus the named run steps
func render(t *testing.T, r *reposcan.Result, suite string) (renderedConfig, []renderedStep) {
	t.Helper()

	body, err := renderConfig(r, suite)
	assert.NilError(t, err)

	var cfg renderedConfig
	assert.NilError(t, yaml.Unmarshal(body, &cfg))

	job, ok := cfg.Jobs[jobName]
	assert.Assert(t, ok, "job %s missing from:\n%s", jobName, body)

	var steps []renderedStep
	for _, n := range job.Steps {
		if n.Kind != yaml.MappingNode {
			continue
		}
		var s renderedStep
		assert.NilError(t, n.Decode(&s))
		steps = append(steps, s)
	}
	return cfg, steps
}

func stepCommands(steps []renderedStep) map[string]string {
	m := make(map[string]string, len(steps))
	for _, s := range steps {
		m[s.Run.Name] = s.Run.Command
	}
	return m
}

func TestRenderConfig_TestStep(t *testing.T) {
	const want = `circleci testsuite run "ci tests"`

	tests := []struct {
		name   string
		result *reposcan.Result
		image  string
		steps  map[string]string
	}{
		{
			name:   "nil result falls back to the generic template",
			result: nil,
			image:  "cimg/base:stable",
			steps: map[string]string{
				"build": `echo "Add your build steps here"`,
				"test":  want,
			},
		},
		{
			name:   "unknown stack falls back to the generic template",
			result: &reposcan.Result{Stack: reposcan.StackUnknown},
			image:  "cimg/base:stable",
			steps: map[string]string{
				"build": `echo "Add your build steps here"`,
				"test":  want,
			},
		},
		{
			name: "detected test step is replaced and other steps are kept",
			result: &reposcan.Result{
				Stack:        "go",
				Image:        "cimg/go",
				ImageVersion: "1.23",
				Setup: []reposcan.SetupStep{
					{Name: "install", Command: "go mod download"},
					{Name: "test", Command: "go test ./..."},
				},
			},
			image: "cimg/go:1.23",
			steps: map[string]string{
				"install": "go mod download",
				"test":    want,
			},
		},
		{
			name: "test step is appended when the scan found none",
			result: &reposcan.Result{
				Stack: "node",
				Image: "cimg/node",
				Setup: []reposcan.SetupStep{
					{Name: "install", Command: "npm ci"},
				},
			},
			image: "cimg/node",
			steps: map[string]string{
				"install": "npm ci",
				"test":    want,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, steps := render(t, tc.result, defaultTestSuiteName)

			t.Run("job uses the expected image", func(t *testing.T) {
				assert.Assert(t, cmp.Len(cfg.Jobs[jobName].Docker, 1))
				assert.Check(t, cmp.Equal(cfg.Jobs[jobName].Docker[0].Image, tc.image))
			})

			t.Run("steps match", func(t *testing.T) {
				assert.Check(t, cmp.DeepEqual(stepCommands(steps), tc.steps))
			})

			t.Run("original test command is gone", func(t *testing.T) {
				for _, s := range steps {
					assert.Check(t, !strings.Contains(s.Run.Command, "go test"), "step %q", s.Run.Name)
				}
			})
		})
	}
}

func TestRenderConfig_StepOrderIsPreserved(t *testing.T) {
	_, steps := render(t, &reposcan.Result{
		Stack: "node",
		Image: "cimg/node",
		Setup: []reposcan.SetupStep{
			{Name: "system", Command: "sudo apt-get update"},
			{Name: "install", Command: "npm ci"},
			{Name: "test", Command: "npm test"},
		},
	}, defaultTestSuiteName)

	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Run.Name
	}
	assert.Check(t, cmp.DeepEqual(names, []string{"system", "install", "test"}))
}

func TestRenderConfig_AppendedTestStepComesLast(t *testing.T) {
	_, steps := render(t, &reposcan.Result{
		Stack: "node",
		Image: "cimg/node",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "npm ci"},
			{Name: "playwright-install", Command: "npx playwright install"},
		},
	}, defaultTestSuiteName)

	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Run.Name
	}
	assert.Check(t, cmp.DeepEqual(names, []string{"install", "playwright-install", "test"}))
}

func TestRenderConfig_SuiteNameIsQuoted(t *testing.T) {
	_, steps := render(t, nil, `my "quoted" suite`)

	assert.Check(t, cmp.Equal(stepCommands(steps)["test"], `circleci testsuite run "my \"quoted\" suite"`))
}
