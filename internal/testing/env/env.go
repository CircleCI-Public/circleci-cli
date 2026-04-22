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

// Package env provides a test environment builder for acceptance tests.
package env

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestEnv holds the environment configuration for a single test run.
type TestEnv struct {
	// HomeDir is the temp home directory for this test.
	HomeDir string
	// CircleCIURL overrides the CircleCI API base URL (for fake servers).
	CircleCIURL string
	// Token is the CircleCI API token injected via environment variable.
	Token string
	// Extra holds additional environment variables.
	Extra map[string]string
}

// New creates a TestEnv with an isolated temp home directory.
func New(t *testing.T) *TestEnv {
	t.Helper()
	home := t.TempDir()
	return &TestEnv{
		HomeDir: home,
		Extra:   map[string]string{},
	}
}

// Environ returns the environment slice suitable for exec.Cmd.Env.
// It includes a minimal safe environment (PATH, no inherited HOME).
func (e *TestEnv) Environ() []string {
	env := []string{
		"HOME=" + e.HomeDir,
		"XDG_CONFIG_HOME=" + filepath.Join(e.HomeDir, ".config"),
		"PATH=" + os.Getenv("PATH"),
		"NO_COLOR=1", // deterministic output in tests
	}
	if e.Token != "" {
		env = append(env, "CIRCLECI_TOKEN="+e.Token)
	}
	if e.CircleCIURL != "" {
		// The fake server URL is injected via a dedicated env var that
		// the API client reads in test builds.
		env = append(env, "CIRCLECI_HOST="+e.CircleCIURL)
	}
	for k, v := range e.Extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
