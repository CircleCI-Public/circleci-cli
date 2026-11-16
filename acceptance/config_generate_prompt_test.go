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

package acceptance_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

var configPromptURL = url.URL{Path: "/api/v3/configs/generate-prompt"}

func runConfigGeneratePrompt(t *testing.T, fake *fakes.CircleCI, dir string, args ...string) binary.CLIResult {
	t.Helper()
	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"config", "generate", dir}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func configPromptBody(t *testing.T, fake *fakes.CircleCI, dir string) map[string]any {
	t.Helper()
	reqs := fake.FindRequests(http.MethodPost, configPromptURL)
	assert.Assert(t, cmp.Len(reqs, 1))
	var body map[string]any
	assert.NilError(t, reqs[0].Decode(&body))
	if p, ok := body["config_path"].(string); ok {
		body["config_path"] = strings.ReplaceAll(filepath.FromSlash(p), dir, "<DIR>")
	}
	return body
}

// TestConfigGenerate_Prompt prints the server's prompt for a detected stack and
// writes nothing. No token is needed.
func TestConfigGenerate_Prompt(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	dir := t.TempDir()
	copyFixture(t, "testdata/config-generate/dotnet", dir)

	result := runConfigGeneratePrompt(t, fake, dir)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("writes nothing", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(dir, ".circleci"))
		assert.Check(t, os.IsNotExist(err))
	})

	t.Run("sends the detected stack", func(t *testing.T) {
		body := configPromptBody(t, fake, dir)
		stack, _ := body["stack"].(map[string]any)
		assert.Check(t, cmp.Equal(stack["stack"], "dotnet"))
		assert.Check(t, cmp.Equal(body["config_path"], filepath.Join("<DIR>", ".circleci", "config.yml")))
		assert.Check(t, cmp.Equal(body["has_existing_config"], false))
	})
}

func TestConfigGenerate_PromptJSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	result := runConfigGeneratePrompt(t, fake, t.TempDir(), "--json")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestConfigGenerate_PromptExistingConfig asks for an improvement prompt and
// leaves the existing config alone.
func TestConfigGenerate_PromptExistingConfig(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	original := []byte("version: 2.1\n")
	assert.NilError(t, os.WriteFile(configPath, original, 0o644))

	result := runConfigGeneratePrompt(t, fake, dir)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	body := configPromptBody(t, fake, dir)
	assert.Check(t, cmp.Equal(body["has_existing_config"], true))

	got, err := os.ReadFile(configPath)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(got, original))
}

// TestConfigGenerate_PromptServerError fails with an API error that points at
// --template rather than falling back to a prompt of its own.
func TestConfigGenerate_PromptServerError(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetConfigPromptStatus(http.StatusInternalServerError)

	result := runConfigGeneratePrompt(t, fake, t.TempDir())

	assert.Check(t, cmp.Equal(result.ExitCode, 4), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
