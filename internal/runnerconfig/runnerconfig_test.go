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

package runnerconfig

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

func testOptions() Options {
	return Options{
		ResourceClass: "my-org/my-runner",
		Token:         "fake-runner-token-value",
		Name:          "prod-server-1",
	}
}

func TestRenderMachine(t *testing.T) {
	got, err := Render(Machine, testOptions())
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(got), t.Name()+".txt"))
}

func TestRenderContainer(t *testing.T) {
	got, err := Render(Container, Options{
		ResourceClass: "my-org/my-runner",
		Token:         "fake-runner-token-value",
	})
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(got), t.Name()+".txt"))
}

func TestRenderProvisioner(t *testing.T) {
	got, err := Render(Provisioner, Options{
		ResourceClass: "my-org/my-runner",
		Token:         "fake-runner-token-value",
	})
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(got), t.Name()+".txt"))
}

// TestRenderMachineAgentKeys decodes the output using the agent's own yaml tags.
// A renamed or mistyped tag here is invisible in a golden file but stops the
// agent from starting, which is the bug this command originally shipped with.
func TestRenderMachineAgentKeys(t *testing.T) {
	opts := testOptions()
	opts.WorkingDirectory = "/srv/circleci/workdir"

	got, err := Render(Machine, opts)
	assert.NilError(t, err)

	var agent struct {
		API struct {
			AuthToken string `yaml:"auth_token"`
		} `yaml:"api"`
		Runner struct {
			Name           string `yaml:"name"`
			WorkDir        string `yaml:"working_directory"`
			CleanupWorkDir bool   `yaml:"cleanup_working_directory"`
		} `yaml:"runner"`
		Logging struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
			File   string `yaml:"file"`
		} `yaml:"logging"`
	}
	assert.NilError(t, yaml.Unmarshal(got, &agent))

	assert.Check(t, cmp.Equal(agent.API.AuthToken, "fake-runner-token-value"))
	assert.Check(t, cmp.Equal(agent.Runner.Name, "prod-server-1"))
	assert.Check(t, cmp.Equal(agent.Runner.WorkDir, "/srv/circleci/workdir"))
	assert.Check(t, cmp.Equal(agent.Runner.CleanupWorkDir, true))
	assert.Check(t, cmp.Equal(agent.Logging.Level, "info"))
	assert.Check(t, cmp.Equal(agent.Logging.Format, "text"))
	assert.Check(t, cmp.Equal(agent.Logging.File, "-"))
}

func TestRenderMachineDefaultsWorkingDirectory(t *testing.T) {
	got, err := Render(Machine, testOptions())
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(string(got), "working_directory: "+DefaultWorkingDirectory))
}

// TestRenderHelmProductsKeyByResourceClass guards the chart contract: the map
// key must be the resource class verbatim, including the slash. A mismatch
// leaves the agent running but never claiming a task.
func TestRenderHelmProductsKeyByResourceClass(t *testing.T) {
	for _, product := range []Product{Container, Provisioner} {
		got, err := Render(product, Options{
			ResourceClass: "my-org/my-runner",
			Token:         "fake-runner-token-value",
		})
		assert.NilError(t, err)
		assert.Check(t, strings.Contains(string(got), "my-org/my-runner:"), "product %s", product)
	}
}

func TestRenderRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		code string
	}{
		{"empty token", Options{ResourceClass: "my-org/my-runner", Name: "runner"}, "runner.invalid_token"},
		{"placeholder token", Options{ResourceClass: "my-org/my-runner", Token: "<< AUTH_TOKEN >>", Name: "runner"}, "runner.invalid_token"},
		{"no namespace", Options{ResourceClass: "my-runner", Token: "tok", Name: "runner"}, "runner.invalid_resource_class"},
		{"empty namespace", Options{ResourceClass: "/my-runner", Token: "tok", Name: "runner"}, "runner.invalid_resource_class"},
		{"empty name part", Options{ResourceClass: "my-org/", Token: "tok", Name: "runner"}, "runner.invalid_resource_class"},
		{"too many parts", Options{ResourceClass: "a/b/c", Token: "tok", Name: "runner"}, "runner.invalid_resource_class"},
		{"empty runner name", Options{ResourceClass: "my-org/my-runner", Token: "tok"}, "runner.invalid_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(Machine, tt.opts)
			assert.Assert(t, err != nil)

			var cliErr *clierrors.CLIError
			assert.Assert(t, errors.As(err, &cliErr))
			assert.Check(t, cmp.Equal(cliErr.Code, tt.code))
		})
	}
}

func TestParseProduct(t *testing.T) {
	for _, valid := range Products {
		got, err := ParseProduct(valid)
		assert.Check(t, err == nil, "expected %q to be valid", valid)
		assert.Check(t, cmp.Equal(string(got), valid))
	}

	// "kubernetes" is the plausible wrong guess, since that is what the agent
	// calls its container-runner subcommand.
	for _, invalid := range []string{"", "kubernetes", "Machine", "machine runner"} {
		_, err := ParseProduct(invalid)
		assert.Assert(t, err != nil, "expected %q to be invalid", invalid)
		assert.Check(t, cmp.Equal(err.Code, "runner.invalid_product"))
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean hostname is untouched", "Akils-MacBook-Pro.local", "Akils-MacBook-Pro.local"},
		{"fqdn is untouched", "runner-01.eu-west-1.example.com", "runner-01.eu-west-1.example.com"},
		{"slash becomes a dash", "my-org/runner", "my-org-runner"},
		{"colon becomes a dash", "host:1", "host-1"},
		{"non-ascii becomes a dash", "wörker", "w-rker"},
		{"parens and underscores survive", "runner_(eu)", "runner_(eu)"},
		{"surrounding punctuation is dropped", "  -host-  ", "host"},
		{"nothing usable leaves empty", "///", ""},
		{"empty stays empty", "", ""},
		{
			"over-long names are truncated to the agent limit",
			strings.Repeat("a", 100),
			strings.Repeat("a", 64),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeName(tt.in)
			assert.Check(t, cmp.Equal(got, tt.want))

			// Whatever survives must be a name the agent accepts.
			if got != "" {
				assert.Check(t, ValidateName(got) == nil, "sanitized %q is still invalid", got)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"runner", "prod-server-1", "My Runner (eu)", "a", strings.Repeat("a", 64)}
	for _, name := range valid {
		assert.Check(t, ValidateName(name) == nil, "expected %q to be valid", name)
	}

	invalid := []string{
		"",
		"<< AGENT_NAME >>",
		"runner_name",
		"RUNNER_NAME",
		"bad/name",
		"bad@name",
		strings.Repeat("a", 65),
	}
	for _, name := range invalid {
		err := ValidateName(name)
		assert.Assert(t, err != nil, "expected %q to be invalid", name)
		assert.Check(t, cmp.Equal(err.Code, "runner.invalid_name"))
	}
}
