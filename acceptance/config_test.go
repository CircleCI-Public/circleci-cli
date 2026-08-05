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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	// language=yaml
	testConfigYAML = `version: "2.1"
jobs:
  build:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
`
	// language=yaml
	testCompiledYAML = `# compiled output
version: "2.1"
`
)

// TestConfigGroup_UnknownSubcommand exercises the group's reaction to a
// subcommand that doesn't exist. The contract — exit 2 plus a structured
// "Unknown command" stderr — is provided by cmdutil.GroupRunE; this test
// pins the user-visible shape from outside the binary.
func TestConfigGroup_UnknownSubcommand(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "bogus"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- config validate ---

func TestConfigValidate(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigValidate_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml", "--json"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["valid"], true))
	_, hasYAML := out["compiled_yaml"]
	assert.Check(t, hasYAML, "expected compiled_yaml field in JSON output")
}

func TestConfigValidate_Invalid(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(false, "", "unknown key 'foo'", "job 'build' is missing required fields")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, "version: \"2.1\"\nfoo: bar\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigValidate_FileNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", "nonexistent.yml"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigValidate_PositionalPath is the regression test for the first half of
// https://github.com/CircleCI-Public/circleci-cli/issues/1636: `circleci config
// validate <path>` — the form 0.1.x documented — must read the file it is given.
func TestConfigValidate_PositionalPath(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "other.yml"), testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "other.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigValidate_PositionalPathNotFound pins the failure mode reported in
// issue 1636: a positional path that does not exist used to be ignored in favour
// of .circleci/config.yml, so validation passed for a file nobody had read. The
// working directory holds a valid default config precisely so that silent
// fallback would show up as a success here.
func TestConfigValidate_PositionalPathNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "does-not-exist.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigValidate_PositionalPathBeatsConfigFlag pins the 0.1.x precedence:
// when a caller passes both, the positional path is the one that is read.
func TestConfigValidate_PositionalPathBeatsConfigFlag(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wins.yml"), testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "wins.yml", "--config", "loses.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stdout, "wins.yml"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigValidate_TooManyArgs keeps the argument contract explicit: a second
// positional is a usage error rather than a silently ignored argument. The exit
// code is Cobra's own for arg-count violations, as it is for config pack and
// config process.
func TestConfigValidate_TooManyArgs(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "a.yml", "b.yml"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

const testOrgUUID = "00000000-0000-0000-0000-0000000000aa"

func TestConfigValidate_WithOrgSlug(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddOrg(testOrgUUID, "gh/myorg", "My Org", "github")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stdout, ".circleci/config.yml"))
	// The slug was resolved to its org UUID before the compile call.
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOrgUUID))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigValidate_WithOrgUUID is the UUID counterpart of
// TestConfigValidate_WithOrgSlug: passing the org UUID directly to --org must
// reach the compile endpoint as the same owner_id, with no slug lookup needed
// (no AddOrg is registered).
func TestConfigValidate_WithOrgUUID(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml", "--org", testOrgUUID},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stdout, ".circleci/config.yml"))
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOrgUUID))
}

// TestConfigValidate_InfersOrgFromGitRemote is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/1061: with no --org,
// the org is inferred from the git remote so private and namespaced orbs
// resolve. The remote https://github.com/myorg/myrepo maps to project slug
// gh/myorg/myrepo, whose org UUID must reach the compile endpoint as owner_id.
func TestConfigValidate_InfersOrgFromGitRemote(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)
	fake.AddProjectBySlug("gh/myorg/myrepo", "00000000-0000-0000-0000-0000000000b1", "myrepo", testOrgUUID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/myrepo")
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, ".circleci/config.yml"))
	// The org inferred from the git remote reached the compile call as owner_id.
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOrgUUID))
}

// TestConfigValidate_ProjectLinkOverridesGitRemote pins that org inference
// honours a `circleci project link` binding (.circleci/info.yml) over the git
// remote — the case where the remote is not the right answer (repository
// renames, forks, standalone projects). The remote points at one org; the link
// binds a different one, and the linked org must reach the compile endpoint.
//
// The linked project is deliberately NOT registered with the fake: `project
// link` records the org UUID in info.yml, so inference uses it directly with no
// project lookup. The linked org resolving without a matching fake project
// proves that direct-ID path (which also works offline / with a restricted
// token).
func TestConfigValidate_ProjectLinkOverridesGitRemote(t *testing.T) {
	const linkedOrgUUID = "00000000-0000-0000-0000-0000000000cc"
	const linkedProjUUID = "00000000-0000-0000-0000-0000000000dd"

	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)
	// The git remote resolves to a *different* org. If the link were ignored,
	// this org would be used and the assertion below would catch it.
	fake.AddProjectBySlug("gh/otherorg/otherrepo", "00000000-0000-0000-0000-0000000000b3", "otherrepo", testOrgUUID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	initGitRepoWithRemote(t, dir, "https://github.com/otherorg/otherrepo")
	writeConfig(t, dir, testConfigYAML)
	writeProjectLink(t, dir, linkedOrgUUID, linkedProjUUID)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	// The linked project's org — not the git remote's — reached compile.
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), linkedOrgUUID))
}

// TestConfigValidate_NoOrgOutsideGitRemote guards the other half of the #1061
// contract: inference is best-effort. Outside a git checkout (and with no
// --org) validation still succeeds, compiling with an empty owner_id rather
// than failing — public configs must validate anywhere.
func TestConfigValidate_NoOrgOutsideGitRemote(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "validate", "--config", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, ".circleci/config.yml"))
	// No git remote to infer from: the compile call carries no owner.
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), ""))
}

// TestConfigValidate_RemovedOrgFlags pins the clean break from the legacy org
// flag names: --org-id and --org-slug were collapsed into --org and must no
// longer be accepted. Cobra reports unknown flags with a non-zero exit and an
// "unknown flag" message on stderr.
func TestConfigValidate_RemovedOrgFlags(t *testing.T) {
	for _, flag := range []string{"--org-id", "--org-slug"} {
		t.Run(flag, func(t *testing.T) {
			env := testenv.New(t)
			env.Token = testToken

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    []string{"config", "validate", flag, "gh/myorg"},
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, result.ExitCode != 0)
			assert.Check(t, cmp.Contains(result.Stderr, "unknown flag: "+flag))
		})
	}
}

// --- config process ---

func TestConfigProcess(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "process", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigProcess_Invalid(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(false, "", "unknown orb 'myorg/unknown@1.0.0'")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "process", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigProcess_WithParams(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "process", ".circleci/config.yml", "--pipeline-parameters", "env: staging"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stdout, "version"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigProcess_InfersOrgFromGitRemote is the config process counterpart of
// TestConfigValidate_InfersOrgFromGitRemote: process shares the same org
// resolution, so with no --org the org is inferred from the git remote and
// reaches the compile endpoint as owner_id.
func TestConfigProcess_InfersOrgFromGitRemote(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, testCompiledYAML)
	fake.AddProjectBySlug("gh/myorg/myrepo", "00000000-0000-0000-0000-0000000000b2", "myrepo", testOrgUUID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/myrepo")
	writeConfig(t, dir, testConfigYAML)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "process", ".circleci/config.yml"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "version"))
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOrgUUID))
}

// --- config pack ---

func TestConfigPack(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci", "jobs"), 0o755))
	writeFile(t, filepath.Join(dir, ".circleci", "config.yml"), "version: \"2.1\"\n")
	writeFile(t, filepath.Join(dir, ".circleci", "jobs", "build.yml"),
		"build:\n  docker:\n    - image: cimg/base:stable\n  steps:\n    - checkout\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", ".circleci"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigPack_ParseErrorIsClickable is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/issues/519: a YAML problem in
// one file of a packed tree has to name that file and line, leading the message,
// so the terminal turns it into a link.
//
// Asserted with Contains rather than a golden file because the message carries an
// absolute path: a golden would embed this machine's temp directory, and would
// need a second copy for Windows' separators.
func TestConfigPack_ParseErrorIsClickable(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci", "executors"), 0o755))
	writeFile(t, filepath.Join(dir, ".circleci", "@config.yml"), "version: \"2.1\"\n")
	writeFile(t, filepath.Join(dir, ".circleci", "executors", "executorA.yml"),
		"docker:\n  - image: not-relevant\n    auth:\n"+
			"      username: $DOCKERHUB_USER\n      password: $DOCKERHUB_ACCESSTOKEN\n    auth:\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", ".circleci"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	// The packed document is not emitted when packing fails.
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	// The path is resolved by the CLI, so compare on the tail: the leading
	// directory is the test's temp dir, which varies per run and per platform.
	assert.Check(t, cmp.Contains(result.Stderr,
		filepath.Join("executors", "executorA.yml")+`:6: mapping key "auth" already defined at line 3`))
}

// TestConfigPack_CrossFileAnchors is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/issues/341, using the layout
// and expected output from the report: an anchor defined in anchors/@anchors.yml
// aliased from commands/my-command.yml.
func TestConfigPack_CrossFileAnchors(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "src", "anchors"), 0o755))
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "src", "commands"), 0o755))
	writeFile(t, filepath.Join(dir, "src", "anchors", "@anchors.yml"), "my-anchor: &my-anchor my-value\n")
	writeFile(t, filepath.Join(dir, "src", "commands", "my-command.yml"), "description: *my-anchor\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestConfigPack_UnknownAnchor checks that resolving anchors across files does
// not turn a mistyped alias into silence: with no definition anywhere in the
// tree it is still an error, and still names the anchor.
//
// Asserted with Contains rather than a golden file because the message carries an
// absolute path: a golden would embed this machine's temp directory, and would
// need a second copy for Windows' separators.
func TestConfigPack_UnknownAnchor(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "src", "anchors"), 0o755))
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "src", "commands"), 0o755))
	writeFile(t, filepath.Join(dir, "src", "anchors", "@anchors.yml"), "my-anchor: &my-anchor my-value\n")
	writeFile(t, filepath.Join(dir, "src", "commands", "c.yml"), "description: *my-anchr\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Contains(result.Stderr, "unknown anchor 'my-anchr' referenced"))
	// Just the filename, no directory: the path is %q-quoted in the message, and
	// %q escapes the separator on Windows, so "commands\c.yml" would not be found
	// in a message reading "commands\\c.yml". c.yml is unambiguous in this tree.
	assert.Check(t, cmp.Contains(result.Stderr, "c.yml"))
}

// TestConfigPack_WarnsOnDetachedListKey is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/issues/512: `requires`
// indented level with the job name is, in YAML, a sibling of it, so the job gets
// no value and CircleCI rejects the packed result at compile time.
//
// The pack still succeeds and still emits what the YAML says — the warning is
// advisory. Two things matter here that the unit tests cannot show: the warning
// goes to stderr, so stdout stays pipeable into validate, and the exit code
// stays 0.
func TestConfigPack_WarnsOnDetachedListKey(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci", "workflows"), 0o755))
	writeFile(t, filepath.Join(dir, ".circleci", "@config.yml"), "version: \"2.1\"\n")
	writeFile(t, filepath.Join(dir, ".circleci", "workflows", "build_and_test.yml"),
		"jobs:\n"+
			"  - specs\n"+
			"  - specs_feature_failures_only:\n"+
			"    requires:\n"+
			"      - specs\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", ".circleci"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	// Advisory: the document is well-formed YAML, so packing succeeds.
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	// Asserted with Contains rather than a golden because the warning carries an
	// absolute path: a golden would embed this machine's temp directory, and would
	// need a second copy for Windows' separators.
	assert.Check(t, cmp.Contains(result.Stderr,
		`"specs_feature_failures_only" has no value but shares a list item with "requires"`))
	assert.Check(t, cmp.Contains(result.Stderr, `indent "requires" one level further`))
	// The location leads, and points at the line the job name is on.
	assert.Check(t, cmp.Contains(result.Stderr, filepath.Join("workflows", "build_and_test.yml")+":3: "))
}

// TestConfigPack_NoSpuriousWarning is the other half: a correctly indented
// workflow job must pack silently. A warning on valid config is worse than a
// missing one, so this pins stderr as empty.
func TestConfigPack_NoSpuriousWarning(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci", "workflows"), 0o755))
	writeFile(t, filepath.Join(dir, ".circleci", "@config.yml"), "version: \"2.1\"\n")
	writeFile(t, filepath.Join(dir, ".circleci", "workflows", "build_and_test.yml"),
		"jobs:\n"+
			"  - specs\n"+
			"  - specs_feature_failures_only:\n"+
			"      requires:\n"+
			"        - specs\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", ".circleci"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

func TestConfigPack_NotFound(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "pack", "nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- helpers ---

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	writeFile(t, filepath.Join(dir, ".circleci", "config.yml"), content)
}

// writeProjectLink writes the .circleci/info.yml that `circleci project link`
// produces, binding the checkout to a CircleCI project by its stable UUIDs.
func writeProjectLink(t *testing.T, dir, orgID, projectID string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	content := fmt.Sprintf(
		"organization:\n  id: %s\nproject:\n  id: %s\n  slug: circleci/%s/%s\n",
		orgID, projectID, orgID, projectID,
	)
	writeFile(t, filepath.Join(dir, ".circleci", "info.yml"), content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
}
