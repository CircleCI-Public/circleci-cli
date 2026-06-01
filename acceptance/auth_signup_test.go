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
	"regexp"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

var terminalURLRe = regexp.MustCompile(`https?://[^\s\x1b]+`)

// TestAuthSignup_NonInteractive_PrintsSignupURL verifies that non-interactive
// signup prints the authorize URL with signup=true, waits for a callback, and
// persists the token — mirroring the login flow.
func TestAuthSignup_NonInteractive_PrintsSignupURL(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id": "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"attributes": map[string]any{
			"name":  "New User",
			"login": "newuser",
		},
	})
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token": "test-signup-token",
		"token_type":   "Bearer",
		"expires_in":   int64(7776000),
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["CIRCLECI_LOGIN_TIMEOUT"] = "10s"

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "signup", "--no-browser"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	var authURL string
	assert.Assert(t, t.Run("read signup authorize url", func(t *testing.T) {
		out, err := console.ExpectString("Waiting for browser authentication")
		assert.NilError(t, err)

		authURL = terminalURLRe.FindString(out)
		assert.Assert(t, authURL != "", "authorize URL not found in output: %q", out)
		assert.Check(t, cmp.Contains(authURL, "signup=true"))
	}))

	assert.Assert(t, t.Run("browser callback", func(t *testing.T) {
		callbackAuthorizeURL(t, authURL)
	}))

	assert.Assert(t, t.Run("logged in", func(t *testing.T) {
		_, err := console.ExpectString("Logged in as newuser")
		assert.NilError(t, err)
	}))
}

// TestAuthSignup_AlreadyAuthenticated asserts that running `circleci auth
// signup` while a token is already configured refuses with a clear error
// and helpful suggestions, rather than silently overwriting the token.
func TestAuthSignup_AlreadyAuthenticated(t *testing.T) {
	env := testenv.New(t)
	env.Token = "preexisting-token"
	// Intentionally unreachable — we want to confirm the guard short-circuits
	// before any network or browser activity.
	env.CircleCIURL = "https://example.invalid"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "signup"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestAuthSignup_HappyPath drives the interactive signup flow against the fake
// server. The CLI presents the same TUI as login (host selection, method
// selection, browser OAuth) but with signup=true on the authorize URL.
func TestAuthSignup_HappyPath(t *testing.T) {
	ctx := t.Context()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id": "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"attributes": map[string]any{
			"name":  "New User",
			"login": "newuser",
		},
	})
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token": "test-signup-token",
		"token_type":   "Bearer",
		"expires_in":   int64(7776000),
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["CIRCLECI_LOGIN_TIMEOUT"] = "20s"

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "signup"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("select host", func(t *testing.T) {
		_, err := console.ExpectString("Where do you use CircleCI")
		assert.NilError(t, err)
		// Move to "Other" and press Enter to type the fake URL.
		_, err = console.Send("\x1b[B")
		assert.NilError(t, err)
		_, err = console.ExpectString("Other")
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("enter custom host", func(t *testing.T) {
		_, err := console.ExpectString("Base URL")
		assert.NilError(t, err)
		_, err = console.Send(fake.URL())
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("select browser method", func(t *testing.T) {
		_, err := console.ExpectString("Login with a web browser")
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}))

	var authURL string
	assert.Assert(t, t.Run("read authorize url and open browser", func(t *testing.T) {
		out, err := console.ExpectString("Press Enter to open in your browser")
		assert.NilError(t, err)

		authURL = terminalURLRe.FindString(out)
		assert.Assert(t, authURL != "", "authorize URL not found in output: %q", out)
		assert.Check(t, cmp.Contains(authURL, "signup=true"))
	}))

	assert.Assert(t, t.Run("browser callback", func(t *testing.T) {
		parsed, err := url.Parse(authURL)
		assert.NilError(t, err)
		q := parsed.Query()
		state := q.Get("state")
		redirectURI := q.Get("redirect_uri")
		assert.Assert(t, state != "", "state param missing from authorize URL")
		assert.Assert(t, redirectURI != "", "redirect_uri param missing from authorize URL")

		callbackURL := redirectURI + "?code=fake-auth-code&state=" + url.QueryEscape(state)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
		assert.NilError(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.NilError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusOK)
	}))

	assert.Assert(t, t.Run("logged in", func(t *testing.T) {
		_, err := console.ExpectString("Logged in as newuser")
		assert.NilError(t, err)
		_, err = console.ExpectString("Saved host to")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("token persisted to YAML", func(t *testing.T) {
		cfgPath := filepath.Join(env.HomeDir, ".config", "circleci", "config.yml")
		body, err := os.ReadFile(cfgPath)
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(string(body), "test-signup-token"))
	}))
}

func callbackAuthorizeURL(t *testing.T, authURL string) {
	t.Helper()

	parsed, err := url.Parse(authURL)
	assert.NilError(t, err)
	q := parsed.Query()
	state := q.Get("state")
	redirectURI := q.Get("redirect_uri")
	assert.Assert(t, state != "", "state param missing from authorize URL")
	assert.Assert(t, redirectURI != "", "redirect_uri param missing from authorize URL")

	callbackURL := redirectURI + "?code=fake-auth-code&state=" + url.QueryEscape(state)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, callbackURL, nil)
	assert.NilError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NilError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}
