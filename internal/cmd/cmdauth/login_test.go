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

package cmdauth

import (
	"testing"

	"gotest.tools/v3/assert"
)

// TestShouldOpenBrowser covers the gate that decides whether the
// non-interactive login/signup flow opens the authorize URL itself. The
// interactive TUI does not consult it at all — see shouldOpenBrowser.
func TestShouldOpenBrowser(t *testing.T) {
	// EnvAllowsInteractive reads these, and both are "set to anything
	// non-empty" checks, so an empty value is equivalent to unset. Clearing
	// them keeps the table independent of the environment the test runs in —
	// notably CI, where CI=true would otherwise fail every positive case.
	clearInteractivityEnv := func(t *testing.T) {
		t.Helper()
		t.Setenv("CI", "")
		t.Setenv("CIRCLE_NO_INTERACTIVE", "")
	}

	t.Run("opens for a detected agent", func(t *testing.T) {
		clearInteractivityEnv(t)
		assert.Check(t, shouldOpenBrowser(false, "claude-code"))
	})

	t.Run("does not open with no agent", func(t *testing.T) {
		clearInteractivityEnv(t)
		assert.Check(t, !shouldOpenBrowser(false, ""),
			"a plain non-TTY invocation (script, pipe) must only print the URL")
	})

	t.Run("--no-browser wins over agent detection", func(t *testing.T) {
		clearInteractivityEnv(t)
		assert.Check(t, !shouldOpenBrowser(true, "claude-code"),
			"--no-browser is an explicit request not to open a browser")
	})

	t.Run("CI suppresses opening", func(t *testing.T) {
		clearInteractivityEnv(t)
		t.Setenv("CI", "true")
		assert.Check(t, !shouldOpenBrowser(false, "claude-code"),
			"an agent inside a CI job must not spawn a browser on a headless runner")
	})

	t.Run("CIRCLE_NO_INTERACTIVE suppresses opening", func(t *testing.T) {
		clearInteractivityEnv(t)
		t.Setenv("CIRCLE_NO_INTERACTIVE", "1")
		assert.Check(t, !shouldOpenBrowser(false, "claude-code"))
	})

	t.Run("agent name is passed through from detection", func(t *testing.T) {
		clearInteractivityEnv(t)
		// Any non-empty name counts; the gate does not allowlist agents.
		for _, name := range []string{"claude-code", "codex", "mcp/gemini-cli", "amp"} {
			assert.Check(t, shouldOpenBrowser(false, name), "agent %q", name)
		}
	})
}
