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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// updateNoticeText is the exact two-line message the notifier prints to stderr
// when __CIRCLE_UPDATE_FORCE reports the current version as 1.2.0.
const updateNoticeText = "A new version of circleci is available: 1.2.0 → 1.3.0\n" +
	"https://github.com/CircleCI-Public/circleci-cli/releases/tag/v1.3.0"

// setupUpdateFake registers a fake serving a 1.3.0 release published 48h ago
// (past the 24h notify delay) and a test env with a token pointed at it.
func setupUpdateFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.SetLatestRelease("circleci-cli", "1.3.0", time.Now().Add(-48*time.Hour))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	env.Extra["__CIRCLE_UPDATE_FORCE"] = "1.2.0"
	return fake, env
}

func runSettingList(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"setting", "list"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestUpdateNotice_AppearsOnStderrNotStdout(t *testing.T) {
	_, env := setupUpdateFake(t)

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// The full two-line message, on stderr...
	assert.Check(t, cmp.Contains(result.Stderr, updateNoticeText))
	// ...and never on stdout, which must stay clean for pipelines.
	assert.Check(t, !strings.Contains(result.Stdout, "A new version of circleci"))
}

func TestUpdateNotice_AbsentUnderJSON(t *testing.T) {
	_, env := setupUpdateFake(t)

	result := runSettingList(t, env, "--json")

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AbsentInCI(t *testing.T) {
	_, env := setupUpdateFake(t)
	env.Extra["CI"] = "true"

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AbsentWithNoUpdateCheckEnv(t *testing.T) {
	_, env := setupUpdateFake(t)
	env.Extra["CIRCLE_NO_UPDATE_CHECK"] = "1"

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AbsentWithSettingDisabled(t *testing.T) {
	_, env := setupUpdateFake(t)

	// Persist update_check: false in the config file the CLI will read.
	cfgPath := filepath.Join(env.ConfigDir(), "circleci", "config.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o700))
	assert.NilError(t, os.WriteFile(cfgPath, []byte("update_check: false\n"), 0o600))

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AppearsWithoutToken(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetLatestRelease("circleci-cli", "1.3.0", time.Now().Add(-48*time.Hour))

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["__CIRCLE_UPDATE_FORCE"] = "1.2.0"
	// No token: GET /api/v3/tool/releases serves anonymously, so the check still
	// runs and unauthenticated users get the notice.

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, updateNoticeText))
}

func TestUpdateNotice_AbsentWithoutForceNonTTY(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetLatestRelease("circleci-cli", "1.3.0", time.Now().Add(-48*time.Hour))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	// No __CIRCLE_UPDATE_FORCE: acceptance runs non-TTY and the binary version is
	// "dev", so the notice must never appear. This is what keeps existing
	// stderr goldens free of churn.

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AbsentOnCommandFailure(t *testing.T) {
	_, env := setupUpdateFake(t)

	// An unknown setting key exits non-zero; PersistentPostRunE (and thus the
	// notice) never runs on a failed command.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"setting", "set", "bogus-key", "value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestUpdateNotice_AbsentWhenServerUnavailable(t *testing.T) {
	fake, env := setupUpdateFake(t)
	fake.SetReleaseStatus(503) // transient failure → no notice, no state written

	result := runSettingList(t, env)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}
