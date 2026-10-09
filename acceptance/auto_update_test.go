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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/poll"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/installmethod"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// setupAutoUpdate builds on setupUpdateFake (a 1.3.0 release, this build treated
// as 1.2.0) with the install method pinned to Homebrew and a fake brew first on
// PATH (see withFakeBrew).
func setupAutoUpdate(t *testing.T) (*testenv.TestEnv, string) {
	t.Helper()
	_, env := setupUpdateFake(t)
	return withFakeBrew(t, env)
}

// withFakeBrew pins the install method to Homebrew and puts a fake brew first on
// PATH. The fake appends each invocation's arguments to the returned file, so a
// test can see whether, and how, the CLI started an upgrade.
func withFakeBrew(t *testing.T, env *testenv.TestEnv) (*testenv.TestEnv, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew does not run on Windows")
	}
	env.Extra[installmethod.ForceEnv] = "homebrew"

	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "brew-args.txt")
	script := "#!/bin/sh\necho \"$@\" >> \"$FAKE_BREW_ARGS\"\n"
	assert.NilError(t, os.WriteFile(filepath.Join(binDir, "brew"), []byte(script), 0o755)) //#nosec:G306 // test helper must be executable
	env.Extra["PATH"] = binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env.Extra["FAKE_BREW_ARGS"] = argsFile
	return env, argsFile
}

// brewCalls returns each recorded brew invocation, or nil when brew never ran.
func brewCalls(t *testing.T, argsFile string) []string {
	t.Helper()
	b, err := os.ReadFile(argsFile) //#nosec:G304 // test-owned temp file
	if os.IsNotExist(err) {
		return nil
	}
	assert.NilError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// waitForBrew waits for the detached upgrade to record its first invocation.
func waitForBrew(t *testing.T, argsFile string) {
	t.Helper()
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if len(brewCalls(t, argsFile)) > 0 {
			return poll.Success()
		}
		return poll.Continue("brew has not run yet")
	}, poll.WithTimeout(10*time.Second))
}

// settleBackground gives a detached upgrade time to run, for tests asserting
// that one never started.
func settleBackground() { time.Sleep(time.Second) }

func TestAutoUpdate_RunsBrewInBackground(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)
	env.Extra[config.AutoUpdateEnv] = "on"

	result := runSettingList(t, env)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	waitForBrew(t, argsFile)
	assert.Check(t, cmp.DeepEqual(brewCalls(t, argsFile), []string{"upgrade circleci"}))
	// The notice says the upgrade has started instead of asking for it.
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAutoUpdate_StartsEvenWhenTheCommandFails(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)
	env.Extra[config.AutoUpdateEnv] = "on"

	// An unknown setting key fails in the command itself, after it has started.
	result := runSetting(t, env, "set", "no-such-key", "value")

	assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
	waitForBrew(t, argsFile)
	assert.Check(t, cmp.DeepEqual(brewCalls(t, argsFile), []string{"upgrade circleci"}))
}

func TestAutoUpdate_DoesNotNeedTheReleaseCheck(t *testing.T) {
	fake, env := setupUpdateFake(t)
	env, argsFile := withFakeBrew(t, env)
	env.Extra[config.AutoUpdateEnv] = "on"
	// Homebrew decides whether there is anything newer, so the upgrade starts
	// even when the CLI can't find out about releases itself.
	fake.SetReleaseStatus(http.StatusServiceUnavailable)

	result := runSettingList(t, env)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	waitForBrew(t, argsFile)
	assert.Check(t, cmp.DeepEqual(brewCalls(t, argsFile), []string{"upgrade circleci"}))
}

func TestAutoUpdate_OffByDefault(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)

	result := runSettingList(t, env)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	settleBackground()
	assert.Check(t, cmp.Nil(brewCalls(t, argsFile)))
	assert.Check(t, cmp.Contains(result.Stderr, "To upgrade, please run: brew upgrade circleci"))
}

func TestAutoUpdate_RunsForAgentsUnderJSON(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)
	env.Extra[config.AutoUpdateEnv] = "on"
	env.Extra["AI_AGENT"] = "codex"

	result := runSettingList(t, env, "--json")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	waitForBrew(t, argsFile)
	assert.Check(t, cmp.DeepEqual(brewCalls(t, argsFile), []string{"upgrade circleci"}))
	// Agents and --json never get the notice, only the upgrade.
	assert.Check(t, !strings.Contains(result.Stderr, "A new version of circleci"))
}

func TestAutoUpdate_AtMostOncePerDay(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)
	env.Extra[config.AutoUpdateEnv] = "on"

	first := runSettingList(t, env)
	assert.Check(t, cmp.Equal(first.ExitCode, 0), "stderr: %s", first.Stderr)
	waitForBrew(t, argsFile)

	second := runSettingList(t, env)
	assert.Check(t, cmp.Equal(second.ExitCode, 0), "stderr: %s", second.Stderr)
	settleBackground()

	assert.Check(t, cmp.Len(brewCalls(t, argsFile), 1))
	// The second run is inside the window, so it asks the user to upgrade again.
	assert.Check(t, cmp.Contains(second.Stderr, "To upgrade, please run: brew upgrade circleci"))
}

func TestAutoUpdate_NeverInCI(t *testing.T) {
	env, argsFile := setupAutoUpdate(t)
	env.Extra[config.AutoUpdateEnv] = "on"
	env.Extra["CI"] = "true"

	result := runSettingList(t, env)

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	settleBackground()
	assert.Check(t, cmp.Nil(brewCalls(t, argsFile)))
}

func runSetting(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"setting"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestSettingAutoUpdate(t *testing.T) {
	env := testenv.New(t)
	env.Extra[installmethod.ForceEnv] = "homebrew"

	autoUpdate := func(t *testing.T) any {
		t.Helper()
		result := runSetting(t, env, "list", "--json")
		assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		return out["auto_update"]
	}

	assert.Assert(t, t.Run("off by default", func(t *testing.T) {
		assert.Check(t, cmp.Equal(autoUpdate(t), false))
	}))

	assert.Assert(t, t.Run("set on", func(t *testing.T) {
		result := runSetting(t, env, "set", "auto-update", "on")
		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, "Auto-update enabled."))
		// A Homebrew install gets no caveat.
		assert.Check(t, !strings.Contains(result.Stderr, "Note:"))
		assert.Check(t, cmp.Equal(autoUpdate(t), true))
	}))

	assert.Assert(t, t.Run("unset reverts to off", func(t *testing.T) {
		result := runSetting(t, env, "unset", "auto-update")
		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.Contains(result.Stderr, "Reverted auto-update to the default (off)"))
		assert.Check(t, cmp.Equal(autoUpdate(t), false))
	}))
}

func TestSettingAutoUpdate_NotHomebrew(t *testing.T) {
	// The test binary runs from a temp dir, so it is no known install method.
	env := testenv.New(t)

	result := runSetting(t, env, "set", "auto-update", "on")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "auto-update only upgrades Homebrew installs for now"))
}

func TestSettingAutoUpdate_InvalidValue(t *testing.T) {
	env := testenv.New(t)

	result := runSetting(t, env, "set", "auto-update", "sometimes")

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "Invalid value for auto-update: sometimes"))
}
