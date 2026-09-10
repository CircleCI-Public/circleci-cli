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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// TestAuthLogin_AgentOpensBrowser covers the one case where the
// non-interactive login flow opens a browser itself: an AI coding agent is
// driving the CLI, so there is no TTY to show the login TUI, but a human is at
// this machine and the authorize URL alone would strand them in tool output
// they may never read.
//
// The platform's browser opener is shadowed on PATH by a script that records
// the URL it was handed, which is why this cannot run on Windows — there
// cli/browser calls a syscall rather than an executable.
func TestAuthLogin_AgentOpensBrowser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so it cannot be shadowed on PATH")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(fakes.User{
		ID:    "e4a72497-7c55-400d-a72d-dadc4b92255d",
		Name:  "Test User",
		Login: "testuser",
	})

	openedPath := filepath.Join(t.TempDir(), "opened.txt")
	binDir := installFakeBrowserOpener(t, openedPath)

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["CIRCLE_LOGIN_TIMEOUT"] = "20s"
	// Make the CLI believe an AI coding agent is driving it — this is the only
	// thing that opts the non-interactive path into opening a browser.
	env.Extra["AI_AGENT"] = "acceptance-test-agent"

	// RunCLI blocks until the CLI exits and the CLI does not exit until the
	// OAuth callback lands, so the callback has to come from another
	// goroutine. Nothing in there asserts — gotest.tools' fatal assertions are
	// only safe on the test goroutine — so the error comes back over a channel.
	cbErr := make(chan error, 1)
	go func() { cbErr <- completeOAuthCallbackAfterBrowserOpens(ctx, fake, openedPath) }()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "login"},
		Env:     withPathPrefix(env.Environ(), binDir),
		WorkDir: t.TempDir(),
	})

	assert.NilError(t, <-cbErr)

	assert.Assert(t, t.Run("browser was opened with the authorize url", func(t *testing.T) {
		recorded, err := os.ReadFile(openedPath)
		assert.NilError(t, err)

		opened := strings.TrimSpace(string(recorded))
		assert.Check(t, cmp.Contains(opened, "/oauth/authorize"))
		assert.Check(t, cmp.Contains(opened, "request_uri="))
		// The opened URL must be the very one the CLI printed, not a second
		// flow with different PKCE state.
		assert.Check(t, cmp.Contains(result.Stderr, opened))
	}))

	assert.Assert(t, t.Run("tells the user to approve, and still prints the url", func(t *testing.T) {
		assert.Check(t, cmp.Contains(result.Stderr, "Opening this URL in your browser"))
		assert.Check(t, cmp.Contains(result.Stderr, "approve the request there to continue"))
		assert.Check(t, terminalURLRe.MatchString(result.Stderr),
			"the URL must still be printed as a fallback in case the browser never opens")
	}))

	assert.Assert(t, t.Run("logged in", func(t *testing.T) {
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Contains(result.Stderr, "Logged in as testuser"))
	}))
}

// TestAuthLogin_NoAgentOnlyPrintsURL is the counterpart: with no agent driving
// it, a non-interactive login must behave exactly as it always has and only
// print the URL. A browser opened here would surprise scripts and CI.
func TestAuthLogin_NoAgentOnlyPrintsURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cli/browser opens the browser via a Windows syscall, so it cannot be shadowed on PATH")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(fakes.User{
		ID:    "e4a72497-7c55-400d-a72d-dadc4b92255d",
		Name:  "Test User",
		Login: "testuser",
	})

	openedPath := filepath.Join(t.TempDir(), "opened.txt")
	binDir := installFakeBrowserOpener(t, openedPath)

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["CIRCLE_LOGIN_TIMEOUT"] = "20s"

	cbErr := make(chan error, 1)
	go func() { cbErr <- completeOAuthCallback(ctx, fake) }()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "login"},
		Env:     withPathPrefix(env.Environ(), binDir),
		WorkDir: t.TempDir(),
	})

	assert.NilError(t, <-cbErr)

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Contains(result.Stderr, "Open this URL in your browser to continue"))
	assert.Check(t, cmp.Contains(result.Stderr, "Logged in as testuser"))

	_, err := os.Stat(openedPath)
	assert.Check(t, os.IsNotExist(err), "no browser may be opened without an agent driving the CLI")
}

// installFakeBrowserOpener shadows the platform's browser opener on PATH with a
// script that appends the URL it was given to recordPath, and returns the
// directory to prepend to PATH. cli/browser runs "open" on darwin and the first
// of xdg-open/x-www-browser/www-browser/wslview it finds on linux, so
// shadowing xdg-open is enough there.
func installFakeBrowserOpener(t *testing.T, recordPath string) string {
	t.Helper()

	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}

	binDir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$1\" >> %q\n", recordPath)
	assert.NilError(t, os.WriteFile(filepath.Join(binDir, opener), []byte(script), 0o755)) //#nosec:G306 // must be executable

	return binDir
}

// withPathPrefix returns environ with dir prepended to its PATH entry. Go
// resolves duplicate environment entries by taking the first, so PATH has to
// be rewritten in place rather than appended to the slice.
func withPathPrefix(environ []string, dir string) []string {
	out := make([]string, 0, len(environ)+1)
	found := false
	for _, e := range environ {
		if rest, ok := strings.CutPrefix(e, "PATH="); ok {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+rest)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

// completeOAuthCallbackAfterBrowserOpens waits for the CLI to launch the
// browser before delivering the callback. Without that ordering the CLI could
// finish and exit before the fire-and-forget opener goroutine is scheduled,
// leaving nothing recorded and the test flaky.
func completeOAuthCallbackAfterBrowserOpens(ctx context.Context, fake *fakes.CircleCI, openedPath string) error {
	if err := waitFor(ctx, func() bool {
		_, err := os.Stat(openedPath)
		return err == nil
	}); err != nil {
		return fmt.Errorf("browser was never opened: %w", err)
	}
	return completeOAuthCallback(ctx, fake)
}

// completeOAuthCallback plays the part of the browser: it recovers the loopback
// redirect_uri and state from the pushed authorization request the CLI sent to
// /oauth/par, then hits the redirect_uri the way a browser would once the user
// approves the consent screen.
func completeOAuthCallback(ctx context.Context, fake *fakes.CircleCI) error {
	var par url.Values
	if err := waitFor(ctx, func() bool {
		par = fake.LastPARRequest()
		return par != nil
	}); err != nil {
		return fmt.Errorf("no pushed authorization request recorded: %w", err)
	}

	redirectURI, state := par.Get("redirect_uri"), par.Get("state")
	if redirectURI == "" || state == "" {
		return fmt.Errorf("pushed authorization request missing redirect_uri or state: %v", par)
	}

	callbackURL := redirectURI + "?code=fake-auth-code&state=" + url.QueryEscape(state)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loopback callback returned %d, want %d", resp.StatusCode, http.StatusOK)
	}
	return nil
}

// waitFor polls done until it returns true or ctx expires.
func waitFor(ctx context.Context, done func() bool) error {
	for {
		if done() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
