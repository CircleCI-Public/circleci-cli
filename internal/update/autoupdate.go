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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

// autoUpdateWindow is the least time between two background upgrades, however
// many commands run. The upgrade starts without knowing whether a newer release
// exists, leaving that to Homebrew, so the window is what keeps brew's own work
// (refreshing its package list) to once a day when there is nothing new.
const autoUpdateWindow = 24 * time.Hour

// ShouldAutoUpdate reports whether this invocation may upgrade the CLI in the
// background: the user opted in, update checks are on, it is a released build
// outside CI, and it was installed with Homebrew, the one package manager the
// CLI can drive without admin rights. Agents are deliberately included: they
// never see the update notice, and the user who opted in did so for them too.
func ShouldAutoUpdate(cfg *config.Config, version, installMethod string) bool {
	if os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv(ForceEnv) == "" && (version == "" || version == "dev") {
		return false
	}
	if cfg == nil || !cfg.IsAutoUpdate() || !cfg.IsUpdateCheck() {
		return false
	}
	return installMethod == "homebrew"
}

// AutoUpdateLogPath is where a background upgrade writes its output, next to
// the state file, so a failed upgrade can be diagnosed.
func AutoUpdateLogPath(statePath string) string {
	return filepath.Join(filepath.Dir(statePath), "auto-update.log")
}

// StartAutoUpdate runs argv as a detached background process, at most once per
// autoUpdateWindow, and reports whether it started one. It runs alongside the
// command, which is safe because the running binary is never touched, and the
// new version takes over on the next run. Like the update check, it never
// surfaces an error: failures are debug-logged and the next window tries again.
func StartAutoUpdate(ctx context.Context, statePath string, argv []string) bool {
	if len(argv) == 0 {
		return false
	}

	// Claim the window under the state lock before starting anything, so
	// commands running at the same time start one upgrade between them.
	claimed := false
	err := config.SaveState(ctx, statePath, func(s *config.State) error {
		if last := s.AutoUpdateStartedAt(); !last.IsZero() && time.Since(last) < autoUpdateWindow {
			return nil
		}
		s.SetAutoUpdateStartedAt(time.Now())
		claimed = true
		return nil
	})
	if err != nil {
		iostream.DebugContext(ctx, "auto-update: could not claim the window", "err", err)
		return false
	}
	if !claimed {
		return false
	}

	if err := startDetached(argv, AutoUpdateLogPath(statePath)); err != nil {
		iostream.DebugContext(ctx, "auto-update: could not start", "err", err)
		return false
	}
	iostream.DebugContext(ctx, "auto-update: started", "argv", argv)
	return true
}

// startDetached starts argv in its own process group, so it outlives this
// process and never receives its signals, with output going to logPath.
func startDetached(argv []string, logPath string) error {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //#nosec:G304 // a fixed file in the CLI's own state dir
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()

	//#nosec:G204 // argv comes from installmethod.HomebrewUpgrade: brew's resolved path and fixed arguments
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = os.TempDir()
	cmd.SysProcAttr = detachAttrs()
	cmd.Env = append(os.Environ(),
		// Keep the old version on disk. A long-running process started from it,
		// such as the MCP server, execs that same path again for every tool call.
		"HOMEBREW_NO_INSTALL_CLEANUP=1",
		"HOMEBREW_NO_ENV_HINTS=1",
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
