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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

// TestAuthSignup_Timeout exercises the wait-for-callback path. The fake
// server is started but never delivers a callback — the CLI should print
// the signup URL (with signup=true and a loopback redirect_uri) and then
// time out cleanly with ExitTimeout.
func TestAuthSignup_Timeout(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Extra["CIRCLECI_LOGIN_TIMEOUT"] = "200ms"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "signup", "--insecure-storage"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 8, "stderr: %s", result.Stderr) // ExitTimeout

	// Targeted URL semantics: lock down the params we care about while the
	// rest of the URL (host, ports, PKCE values, state, device_id) is random.
	assert.Check(t, cmp.Contains(result.Stderr, "signup=true"))
	assert.Check(t, strings.Contains(result.Stderr, "127.0.0.1%3A") ||
		strings.Contains(result.Stderr, "127.0.0.1:"),
		"authorize URL must include a loopback redirect_uri:\n%s", result.Stderr)

	// Golden-compare the surrounding stderr after scrubbing the random URL.
	scrubbed := regexp.MustCompile(`https?://\S+`).ReplaceAllString(result.Stderr, "<authorize-url>")
	assert.Check(t, golden.String(scrubbed, t.Name()+".stderr.txt"))
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

// TestAuthSignup_HappyPath drives the full signup flow against the fake
// server, mirroring TestAuthLogin_Browser. The CLI prints the authorize
// URL on a PTY; the test scrapes it, hits the loopback redirect_uri with a
// fake code, and asserts the CLI exchanges the code, prints "Signed up as
// …", and persists the token to YAML (--insecure-storage path).
func TestAuthSignup_HappyPath(t *testing.T) {
	ctx := t.Context()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":    "user-uuid-1234",
		"name":  "New User",
		"login": "newuser",
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

	var authURL string
	assert.Assert(t, t.Run("read authorize url", func(t *testing.T) {
		// "Waiting for signup to complete" is printed AFTER the URL,
		// so the buffer at this point contains the URL.
		out, err := console.ExpectString("Waiting for signup")
		assert.NilError(t, err)

		authURL = regexp.MustCompile(`https?://\S+`).FindString(out)
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

	assert.Assert(t, t.Run("signed up", func(t *testing.T) {
		_, err := console.ExpectString("Signed up as newuser")
		assert.NilError(t, err)
		_, err = console.ExpectString("Saved host to")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("token persisted to YAML", func(t *testing.T) {
		// --insecure-storage is injected by RunCLIInteractive; token lands in
		// the YAML config under HOME/.config/circleci/config.yml.
		cfgPath := filepath.Join(env.HomeDir, ".config", "circleci", "config.yml")
		body, err := os.ReadFile(cfgPath)
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(string(body), "test-signup-token"))
	}))
}
