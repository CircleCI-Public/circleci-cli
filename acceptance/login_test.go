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
	"regexp"
	"testing"

	"github.com/Netflix/go-expect"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const keyDown = "\x1b[B"

// navigateToMethodPicker drives the TUI through the host-picker stages common
// to all login flows: selects "Other", types the given base URL, and returns
// once the method picker ("Login with a web browser" / "Paste an
// authentication token") is visible.
func navigateToMethodPicker(t *testing.T, console *expect.Console, baseURL string) {
	t.Helper()

	_, err := console.ExpectString("Where do you use CircleCI?")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // move to "Other", confirm
	assert.NilError(t, err)

	_, err = console.ExpectString("Base URL:")
	assert.NilError(t, err)
	_, err = console.Send(baseURL + "\r")
	assert.NilError(t, err)
}

// TestAuthLogin_Browser drives the interactive login TUI through the full
// browser OAuth flow without opening an actual browser. It:
//
//  1. Selects "Other" in the host picker and types the fake server URL.
//  2. Picks "Login with a web browser".
//  3. Reads the authorize URL from the prompt output.
//  4. Simulates the browser redirect by hitting the embedded redirect_uri
//     directly, passing back the state from the authorize URL.
func TestAuthLogin_Browser(t *testing.T) {
	ctx := t.Context()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":    "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"name":  "Test User",
		"login": "testuser",
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	// Short timeout so the test fails fast if the callback never arrives.
	env.Extra["CIRCLECI_LOGIN_TIMEOUT"] = "20s"

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "login"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("pick host", func(t *testing.T) {
		navigateToMethodPicker(t, console, fake.URL())
	}))

	assert.Assert(t, t.Run("pick method", func(t *testing.T) {
		_, err := console.ExpectString("Login with a web browser")
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}))

	var authURL string
	assert.Assert(t, t.Run("read authorize url", func(t *testing.T) {
		out, err := console.ExpectString("Press Enter to open")
		assert.NilError(t, err)

		// The URL is printed as plain text (no ANSI codes) so the terminal
		// detects it as a clickable link. It is the only https?:// token here.
		authURL = regexp.MustCompile(`https?://\S+`).FindString(out)
		assert.Assert(t, authURL != "", "authorize URL not found in output: %q", out)
	}))

	assert.Assert(t, t.Run("browser callback", func(t *testing.T) {
		parsed, err := url.Parse(authURL)
		assert.NilError(t, err)
		q := parsed.Query()
		state := q.Get("state")
		redirectURI := q.Get("redirect_uri")
		assert.Assert(t, state != "", "state param missing from authorize URL")
		assert.Assert(t, redirectURI != "", "redirect_uri param missing from authorize URL")

		// Hit the loopback callback server directly — the same request a browser
		// would make after the user approves the OAuth consent screen.
		callbackURL := redirectURI + "?code=fake-auth-code&state=" + url.QueryEscape(state)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
		assert.NilError(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.NilError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusOK)
	}))

	assert.Assert(t, t.Run("logged in", func(t *testing.T) {
		_, err := console.ExpectString("Logged in as testuser")
		assert.NilError(t, err)
		_, err = console.ExpectString("Saved host to")
		assert.NilError(t, err)
	}))
}

// TestAuthLogin_Token drives the interactive login TUI through the
// personal-access-token flow: selects "Other", types the fake server URL,
// picks "Paste an authentication token", types a token, and asserts the CLI
// exits 0 having validated the token against /api/v2/me.
func TestAuthLogin_Token(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":    "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"name":  "Test User",
		"login": "testuser",
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"auth", "login"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("pick host", func(t *testing.T) {
		navigateToMethodPicker(t, console, fake.URL())
	}))

	assert.Assert(t, t.Run("pick method", func(t *testing.T) {
		_, err := console.ExpectString("Paste an authentication token")
		assert.NilError(t, err)
		_, err = console.Send(keyDown + "\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("enter token", func(t *testing.T) {
		_, err := console.ExpectString("Enter CircleCI personal access token")
		assert.NilError(t, err)
		_, err = console.Send("fake-token\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("logged in", func(t *testing.T) {
		_, err := console.ExpectString("Logged in as testuser")
		assert.NilError(t, err)
		_, err = console.ExpectString("Saved host to")
		assert.NilError(t, err)
	}))
}
