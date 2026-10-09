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

package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

func TestShouldAutoUpdate(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		version string
		method  string
		want    bool
	}{
		{name: "opted in, Homebrew, released build", env: map[string]string{config.AutoUpdateEnv: "on"}, version: "1.4.0", method: "homebrew", want: true},
		{name: "off unless opted in", version: "1.4.0", method: "homebrew", want: false},
		{name: "other install methods never", env: map[string]string{config.AutoUpdateEnv: "on"}, version: "1.4.0", method: "deb", want: false},
		{name: "dev builds never", env: map[string]string{config.AutoUpdateEnv: "on"}, version: "dev", method: "homebrew", want: false},
		{name: "never in CI", env: map[string]string{config.AutoUpdateEnv: "on", "CI": "true"}, version: "1.4.0", method: "homebrew", want: false},
		{name: "never with update checks off", env: map[string]string{config.AutoUpdateEnv: "on", "CIRCLE_NO_UPDATE_CHECK": "1"}, version: "1.4.0", method: "homebrew", want: false},
		{name: "agents included", env: map[string]string{config.AutoUpdateEnv: "on", "AI_AGENT": "codex"}, version: "1.4.0", method: "homebrew", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{config.AutoUpdateEnv, "CI", "CIRCLE_NO_UPDATE_CHECK", ForceEnv, "AI_AGENT"} {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			cfg, err := config.Load(testCtx(), filepath.Join(t.TempDir(), "config.yml"), false)
			assert.NilError(t, err)

			assert.Check(t, cmp.Equal(ShouldAutoUpdate(cfg, tt.version, tt.method), tt.want))
		})
	}
}

func TestStartAutoUpdate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew does not run on Windows")
	}
	// A harmless command that exists on every Unix and exits straight away.
	succeed := []string{"true"}

	t.Run("starts at most once a day", func(t *testing.T) {
		path := statePath(t)

		assert.Check(t, StartAutoUpdate(testCtx(), path, succeed), "first call should start")
		assert.Check(t, !StartAutoUpdate(testCtx(), path, succeed), "second call within the window should not")
		assert.Check(t, time.Since(loadState(t, path).AutoUpdateStartedAt()) < time.Minute)
	})

	t.Run("writes output to the log next to the state file", func(t *testing.T) {
		path := statePath(t)

		assert.Assert(t, StartAutoUpdate(testCtx(), path, succeed))
		assert.Check(t, cmp.Equal(AutoUpdateLogPath(path), filepath.Join(filepath.Dir(path), "auto-update.log")))
		_, err := os.Stat(AutoUpdateLogPath(path))
		assert.Check(t, err, "the log file should exist once the upgrade has started")
	})

	t.Run("starts again once the window has passed", func(t *testing.T) {
		path := statePath(t)
		assert.NilError(t, config.SaveState(testCtx(), path, func(s *config.State) error {
			s.SetAutoUpdateStartedAt(time.Now().Add(-25 * time.Hour))
			return nil
		}))

		assert.Check(t, StartAutoUpdate(testCtx(), path, succeed))
	})

	t.Run("nothing to run", func(t *testing.T) {
		assert.Check(t, !StartAutoUpdate(testCtx(), statePath(t), nil))
	})

	t.Run("a command that cannot start is reported as not started", func(t *testing.T) {
		missing := []string{filepath.Join(t.TempDir(), "no-such-brew"), "upgrade", "circleci"}
		assert.Check(t, !StartAutoUpdate(testCtx(), statePath(t), missing))
	})
}
