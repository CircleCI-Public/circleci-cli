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
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aymanbagabas/go-pty"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

// runLoginCLI starts `circleci auth login --no-browser` with CIRCLECI_HOST
// set to baseURL, streams stderr until it sees the authorize URL printed by
// the command, then returns the URL alongside the running command. The caller
// is responsible for delivering a callback to redirect_uri (or cancelling the
// context) and then calling finishLoginCLI to collect output and the exit code.
func runLoginCLI(t *testing.T, baseURL string, extraEnv ...string) (cmd *exec.Cmd, authorizeURL string, stdout, stderr *bytes.Buffer, stderrPipe io.ReadCloser) {
	t.Helper()

	binPath, cleanup, err := binary.BuildBinary()
	assert.NilError(t, err)
	t.Cleanup(cleanup)

	env := testenv.New(t)
	env.CircleCIURL = baseURL

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	cmd = exec.CommandContext(ctx, binPath, //nolint:gosec // test binary path
		"--insecure-storage",
		"auth", "login", "--no-browser",
	)
	cmd.Env = append(env.Environ(), extraEnv...)
	cmd.Dir = t.TempDir()

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.Stdout = stdout

	stderrPipe, err = cmd.StderrPipe()
	assert.NilError(t, err)

	assert.NilError(t, cmd.Start())

	// Tee stderr into a buffer for assertions while we scan it for the URL.
	tee := io.TeeReader(stderrPipe, stderr)
	scanner := bufio.NewScanner(tee)

	deadline := time.After(10 * time.Second)
	urlCh := make(chan string, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if u := extractAuthorizeURL(line); u != "" {
				urlCh <- u
				return
			}
		}
		urlCh <- ""
	}()

	select {
	case authorizeURL = <-urlCh:
	case <-deadline:
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for authorize URL on stderr")
	}
	assert.Assert(t, authorizeURL != "", "expected authorize URL on stderr; got: %s", stderr.String())

	return cmd, authorizeURL, stdout, stderr, stderrPipe
}

// extractAuthorizeURL returns the authorize URL on a line if present, else "".
// Matches any http(s) URL that contains "/oauth/authorize?".
func extractAuthorizeURL(line string) string {
	for _, scheme := range []string{"https://", "http://"} {
		i := strings.Index(line, scheme)
		if i < 0 {
			continue
		}
		rest := line[i:]
		if !strings.Contains(rest, "/oauth/authorize?") {
			continue
		}
		if end := strings.IndexAny(rest, " \t\r\n"); end > 0 {
			return rest[:end]
		}
		return rest
	}
	return ""
}

// finishLoginCLI drains remaining stderr and waits for the command to exit.
func finishLoginCLI(t *testing.T, cmd *exec.Cmd, stderr *bytes.Buffer, stderrPipe io.ReadCloser) int {
	t.Helper()
	_, _ = io.Copy(stderr, stderrPipe)
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	assert.NilError(t, err)
	return 0
}

// deliverCallback issues the OAuth provider's redirect to the CLI's loopback
// server with the given code/state.
func deliverCallback(t *testing.T, authorizeURL, code, state string) {
	t.Helper()
	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)

	cb, err := url.Parse(u.Query().Get("redirect_uri"))
	assert.NilError(t, err)
	q := cb.Query()
	if code != "" {
		q.Set("code", code)
	}
	if state != "" {
		q.Set("state", state)
	}
	cb.RawQuery = q.Encode()

	resp, err := http.Get(cb.String()) //nolint:gosec,noctx // test loopback URL
	assert.NilError(t, err)
	_ = resp.Body.Close()
	assert.Check(t, cmp.Equal(resp.StatusCode, http.StatusOK))
}

func TestAuthLogin(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token":  "test-access-token",
		"token_type":    "Bearer",
		"expires_in":    int64(3600),
		"refresh_token": "test-refresh-token",
	})

	cmd, authorizeURL, stdout, stderr, stderrPipe := runLoginCLI(t, fake.URL())

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	state := u.Query().Get("state")

	deliverCallback(t, authorizeURL, "test-auth-code", state)

	exit := finishLoginCLI(t, cmd, stderr, stderrPipe)
	assert.Equal(t, exit, 0, "stderr: %s", stderr.String())
	assert.Equal(t, stdout.String(), "")
	assert.Check(t, cmp.Contains(stderr.String(), "OK: Saved token to"))
	assert.Check(t, !strings.Contains(stderr.String(), "Received authorization code"),
		"stderr should not leak internal protocol step; got: %s", stderr.String())
}

func TestAuthLogin_StateMismatch(t *testing.T) {
	cmd, authorizeURL, _, stderr, stderrPipe := runLoginCLI(t, "https://example.com")

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	wrongState := u.Query().Get("state") + "tampered"

	// We can't use deliverCallback here because the loopback server returns
	// 400 on state mismatch and we want to assert that without failing.
	cb, _ := url.Parse(u.Query().Get("redirect_uri"))
	q := cb.Query()
	q.Set("code", "test-auth-code")
	q.Set("state", wrongState)
	cb.RawQuery = q.Encode()
	resp, err := http.Get(cb.String()) //nolint:gosec,noctx // test loopback URL
	assert.NilError(t, err)
	_ = resp.Body.Close()
	assert.Check(t, cmp.Equal(resp.StatusCode, http.StatusBadRequest))

	exit := finishLoginCLI(t, cmd, stderr, stderrPipe)
	assert.Equal(t, exit, 3, "expected ExitAuthError; stderr: %s", stderr.String())
	assert.Check(t, cmp.Contains(stderr.String(), "error: state parameter does not match the CLI's expected value"))
}

func TestAuthLogin_OAuthError(t *testing.T) {
	cmd, authorizeURL, _, stderr, stderrPipe := runLoginCLI(t, "https://example.com")

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	cb, _ := url.Parse(u.Query().Get("redirect_uri"))
	q := cb.Query()
	q.Set("error", "access_denied")
	q.Set("error_description", "User denied authorization")
	cb.RawQuery = q.Encode()
	resp, err := http.Get(cb.String()) //nolint:gosec,noctx // test loopback URL
	assert.NilError(t, err)
	_ = resp.Body.Close()

	exit := finishLoginCLI(t, cmd, stderr, stderrPipe)
	assert.Equal(t, exit, 3, "expected ExitAuthError; stderr: %s", stderr.String())
	assert.Check(t, cmp.Contains(stderr.String(), "error: authorization failed: access_denied: User denied authorization"))
}

// runLoginCLIPTY starts `circleci auth login` (no --no-browser) under a PTY so
// the binary's IsInteractive check returns true and the new prompt-before-browser
// flow is exercised. BROWSER is set to /bin/true so OpenBrowser is a no-op.
// Returns the running command, the captured authorize URL, the PTY (for writing
// the Enter keystroke), and a function that blocks until the binary exits and
// returns the combined PTY output plus the exit code.
func runLoginCLIPTY(t *testing.T, baseURL, browserCmd string) (cmd *pty.Cmd, p pty.Pty, authorizeURL string, finish func() (string, int)) {
	t.Helper()
	skip.If(t, runtime.GOOS == "windows", "PTY-based interactive prompt test not supported on Windows")

	binPath, cleanup, err := binary.BuildBinary()
	assert.NilError(t, err)
	t.Cleanup(cleanup)

	env := testenv.New(t)
	env.CircleCIURL = baseURL
	envSlice := append(env.Environ(), "BROWSER="+browserCmd)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	p, err = pty.New()
	assert.NilError(t, err)

	cmd = p.CommandContext(ctx, binPath,
		"--insecure-storage",
		"auth", "login",
	)
	cmd.Env = envSlice
	cmd.Dir = t.TempDir()
	assert.NilError(t, cmd.Start())

	output := &bytes.Buffer{}
	urlCh := make(chan string, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
				if u := extractAuthorizeURL(output.String()); u != "" {
					select {
					case urlCh <- u:
					default:
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case authorizeURL = <-urlCh:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timed out waiting for authorize URL on PTY output\noutput: %s", output.String())
	}
	assert.Assert(t, authorizeURL != "")

	finish = func() (string, int) {
		err := cmd.Wait()
		_ = p.Close()
		<-drained
		exit := 0
		if exitErr, ok := errAs[*exec.ExitError](err); ok {
			exit = exitErr.ExitCode()
		} else if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) && !errors.Is(err, os.ErrClosed) {
			t.Fatalf("cmd.Wait failed: %v\noutput: %s", err, output.String())
		}
		return output.String(), exit
	}
	return cmd, p, authorizeURL, finish
}

// errAs is a small helper around errors.As for use with type-asserted unwrapping.
func errAs[T error](err error) (T, bool) {
	var t T
	if errors.As(err, &t) {
		return t, true
	}
	return t, false
}

func TestAuthLogin_InteractivePrompt(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token": "interactive-access-token",
		"token_type":   "Bearer",
		"expires_in":   int64(3600),
	})

	_, p, authorizeURL, finish := runLoginCLIPTY(t, fake.URL(), "/bin/true")

	// Send Enter to dismiss the "Press Enter to continue..." prompt.
	_, err := p.Write([]byte("\n"))
	assert.NilError(t, err)

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	deliverCallback(t, authorizeURL, "test-auth-code", u.Query().Get("state"))

	out, exit := finish()
	assert.Equal(t, exit, 0, "output: %s", out)

	assert.Check(t, cmp.Contains(out, "The following URL will be opened in your browser to authorize the CLI:"))
	assert.Check(t, cmp.Contains(out, "Press Enter to continue (Ctrl+C to abort)"))
	assert.Check(t, cmp.Contains(out, "Saved token to"))
	assert.Check(t, !strings.Contains(out, "Received authorization code"),
		"PTY output should not leak internal protocol step; got: %s", out)
}

// TestAuthLogin_InteractivePromptCancel verifies that Ctrl+C at the
// "Press Enter to continue..." prompt aborts promptly. The previous
// implementation called bufio.ReadString on stdin without selecting on the
// context, so SIGINT only cancelled ctx — the read kept blocking until the
// user actually pressed Enter, making the CLI appear hung.
func TestAuthLogin_InteractivePromptCancel(t *testing.T) {
	cmd, _, _, finish := runLoginCLIPTY(t, "https://example.com", "/bin/true")

	// We're parked at the "Press Enter to continue..." prompt. Send SIGINT
	// instead of a newline.
	assert.NilError(t, cmd.Process.Signal(os.Interrupt))

	// Bound the wait so a regression (read that ignores ctx) fails this test
	// instead of hanging until the 30s context timeout.
	done := make(chan struct{})
	var out string
	var exit int
	go func() {
		out, exit = finish()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("CLI did not exit within 5s of SIGINT at prompt; output: %s", out)
	}

	assert.Equal(t, exit, 6, "expected ExitCancelled; output: %s", out)
	assert.Check(t, cmp.Contains(out, "Interrupted before authorization started."))
}

// TestAuthLogin_Timeout covers the auth.login.timeout branch by setting
// CIRCLECI_LOGIN_TIMEOUT to a very short duration and never delivering the
// OAuth callback. The CLI should give up with ExitTimeout and surface the
// re-run suggestion.
func TestAuthLogin_Timeout(t *testing.T) {
	cmd, _, _, stderr, stderrPipe := runLoginCLI(t, "https://example.com", "CIRCLECI_LOGIN_TIMEOUT=100ms")

	exit := finishLoginCLI(t, cmd, stderr, stderrPipe)
	assert.Equal(t, exit, 8, "expected ExitTimeout; stderr: %s", stderr.String())
	assert.Check(t, cmp.Contains(stderr.String(), "error: No callback received within the timeout window."))
	assert.Check(t, cmp.Contains(stderr.String(), "Re-run 'circleci auth login'"))
}

// TestAuthLogin_BrowserOpenFails covers the fallback message shown when
// OpenBrowser returns an error. We force failure by pointing BROWSER at a
// non-existent binary so cmd.Start() returns ENOENT. The flow should not
// abort — the user is told to open the URL manually and the OAuth callback
// still completes the login.
//
// Linux-only: pkg/browser dispatches via xdg-open on Linux (which honors the
// BROWSER env var), but uses `open` on macOS and `rundll32` on Windows —
// neither returns an error for a missing handler, so the failure path can't
// be exercised there.
func TestAuthLogin_BrowserOpenFails(t *testing.T) {
	skip.If(t, runtime.GOOS != "linux", "browser-open failure can only be forced on Linux via BROWSER env var")
	fake := fakes.NewCircleCI(t)
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token": "fallback-access-token",
		"token_type":   "Bearer",
	})

	_, p, authorizeURL, finish := runLoginCLIPTY(t, fake.URL(), "circleci-test-nonexistent-browser-xyz")

	_, err := p.Write([]byte("\n"))
	assert.NilError(t, err)

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	deliverCallback(t, authorizeURL, "test-auth-code", u.Query().Get("state"))

	out, exit := finish()
	assert.Equal(t, exit, 0, "output: %s", out)

	assert.Check(t, cmp.Contains(out, "Could not open browser automatically. Open this URL manually:"))
	assert.Check(t, cmp.Contains(out, "Saved token to"))
}

// TestAuthLogin_TokenExchangeFails covers the auth.login.token_exchange_failed
// branch: callback succeeds, but POST /oauth/token returns a 4xx error. The
// CLI should exit with ExitAuthError and surface the OAuth error.
func TestAuthLogin_TokenExchangeFails(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetOAuthTokenError(http.StatusBadRequest, map[string]any{
		"error":             "invalid_grant",
		"error_description": "code already redeemed",
	})

	cmd, authorizeURL, _, stderr, stderrPipe := runLoginCLI(t, fake.URL())

	u, err := url.Parse(authorizeURL)
	assert.NilError(t, err)
	deliverCallback(t, authorizeURL, "test-auth-code", u.Query().Get("state"))

	exit := finishLoginCLI(t, cmd, stderr, stderrPipe)
	assert.Equal(t, exit, 3, "expected ExitAuthError; stderr: %s", stderr.String())
	assert.Check(t, cmp.Contains(stderr.String(), "error: "))
	assert.Check(t, cmp.Contains(stderr.String(), "invalid_grant"))
}

// TestAuthLogin_ConfigLoadFailure covers the config.load_failed branch by
// pointing --config at a file containing malformed YAML. The CLI should exit
// with ExitGeneralError and surface the parse error in the structured format.
func TestAuthLogin_ConfigLoadFailure(t *testing.T) {
	badConfig := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(badConfig, []byte("token: [unclosed\n"), 0o600)
	assert.NilError(t, err)

	env := testenv.New(t)
	env.CircleCIURL = "https://example.com"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--config", badConfig, "auth", "login", "--no-browser"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "expected ExitGeneralError; stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "error: parsing config file"))
	assert.Check(t, cmp.Contains(result.Stderr, badConfig))
}
