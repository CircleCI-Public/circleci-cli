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

package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// startFlow returns a Flow against the given host with a Cleanup that closes it.
func startFlow(t *testing.T, host string) *Flow {
	t.Helper()
	flow, err := Start(context.Background(), host, "test-device-id", "test-os")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = flow.Close() })
	return flow
}

// callback issues a GET to the loopback redirect_uri with the given query params.
func callback(t *testing.T, redirectURI string, params map[string]string) {
	t.Helper()
	cb, err := url.Parse(redirectURI)
	assert.NilError(t, err)
	q := cb.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	cb.RawQuery = q.Encode()

	resp, err := http.Get(cb.String())
	assert.NilError(t, err)
	_ = resp.Body.Close()
}

func TestStart_AuthorizeURL(t *testing.T) {
	flow := startFlow(t, "https://example.com")

	u, err := url.Parse(flow.AuthorizeURL)
	assert.NilError(t, err)

	t.Run("base URL", func(t *testing.T) {
		assert.Check(t, is.Equal(u.Scheme, "https"))
		assert.Check(t, is.Equal(u.Host, "example.com"))
		assert.Check(t, is.Equal(u.Path, "/oauth/authorize"))
	})

	t.Run("required PKCE and OAuth params", func(t *testing.T) {
		q := u.Query()
		assert.Check(t, is.Equal(q.Get("client_id"), ClientID))
		assert.Check(t, is.Equal(q.Get("response_type"), "code"))
		assert.Check(t, is.Equal(q.Get("code_challenge_method"), "S256"))
		assert.Check(t, q.Get("code_challenge") != "")
		assert.Check(t, q.Get("state") != "")
	})

	t.Run("device_id and os params", func(t *testing.T) {
		q := u.Query()
		assert.Check(t, is.Equal(q.Get("device_id"), "test-device-id"))
		assert.Check(t, is.Equal(q.Get("os"), "test-os"))
	})

	t.Run("code_challenge is SHA256(verifier)", func(t *testing.T) {
		h := sha256.Sum256([]byte(flow.verifier))
		want := base64.RawURLEncoding.EncodeToString(h[:])
		assert.Check(t, is.Equal(u.Query().Get("code_challenge"), want))
	})

	t.Run("state is 32 hex chars", func(t *testing.T) {
		s := u.Query().Get("state")
		assert.Check(t, is.Len(s, 32))
	})

	t.Run("redirect_uri is loopback", func(t *testing.T) {
		r := u.Query().Get("redirect_uri")
		assert.Check(t, strings.HasPrefix(r, "http://127.0.0.1:"))
		assert.Check(t, strings.HasSuffix(r, "/callback"))
	})
}

func TestStart_TrimsTrailingSlashFromHost(t *testing.T) {
	flow := startFlow(t, "https://example.com/")
	assert.Check(t, strings.HasPrefix(flow.AuthorizeURL, "https://example.com/oauth/authorize?"))
}

func TestStart_FlowsAreUnique(t *testing.T) {
	a := startFlow(t, "https://example.com")
	b := startFlow(t, "https://example.com")

	assert.Check(t, a.verifier != b.verifier, "verifier must be per-flow")
	assert.Check(t, a.state != b.state, "state must be per-flow")
}

func TestFlow_Wait_Success(t *testing.T) {
	flow := startFlow(t, "https://example.com")
	u, _ := url.Parse(flow.AuthorizeURL)

	go callback(t, u.Query().Get("redirect_uri"), map[string]string{
		"code":  "test-code",
		"state": u.Query().Get("state"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := flow.Wait(ctx)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.Code, "test-code"))
	assert.Check(t, is.Equal(res.Verifier, flow.verifier))
}

func TestFlow_Wait_StateMismatch(t *testing.T) {
	flow := startFlow(t, "https://example.com")
	u, _ := url.Parse(flow.AuthorizeURL)

	go callback(t, u.Query().Get("redirect_uri"), map[string]string{
		"code":  "test-code",
		"state": "wrong-state",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := flow.Wait(ctx)
	assert.ErrorContains(t, err, "state")
}

func TestFlow_Wait_OAuthError(t *testing.T) {
	flow := startFlow(t, "https://example.com")
	u, _ := url.Parse(flow.AuthorizeURL)

	go callback(t, u.Query().Get("redirect_uri"), map[string]string{
		"error":             "access_denied",
		"error_description": "User declined the request",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := flow.Wait(ctx)
	assert.ErrorContains(t, err, "access_denied")
	assert.ErrorContains(t, err, "User declined the request")
}

func TestFlow_Wait_MissingCode(t *testing.T) {
	flow := startFlow(t, "https://example.com")
	u, _ := url.Parse(flow.AuthorizeURL)

	go callback(t, u.Query().Get("redirect_uri"), map[string]string{
		"state": u.Query().Get("state"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := flow.Wait(ctx)
	assert.ErrorContains(t, err, "code")
}

func TestFlow_Wait_ContextCancelled(t *testing.T) {
	flow := startFlow(t, "https://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := flow.Wait(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFlow_Wait_ContextDeadline(t *testing.T) {
	flow := startFlow(t, "https://example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := flow.Wait(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestFlow_Callback_RejectsUnknownPath(t *testing.T) {
	flow := startFlow(t, "https://example.com")
	u, _ := url.Parse(flow.AuthorizeURL)

	r, _ := url.Parse(u.Query().Get("redirect_uri"))
	r.Path = "/not-callback"

	resp, err := http.Get(r.String())
	assert.NilError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Check(t, is.Equal(resp.StatusCode, http.StatusNotFound))
}

func TestFlow_Exchange_Success(t *testing.T) {
	var gotForm url.Values
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "exchanged-token",
			"token_type":    "Bearer",
			"expires_in":    int64(7200),
			"refresh_token": "exchanged-refresh",
		})
	}))
	t.Cleanup(srv.Close)

	flow := startFlow(t, srv.URL)
	tok, err := flow.Exchange(context.Background(), "test-code")
	assert.NilError(t, err)

	assert.Check(t, is.Equal(tok.AccessToken, "exchanged-token"))
	assert.Check(t, is.Equal(tok.TokenType, "Bearer"))
	assert.Check(t, is.Equal(tok.ExpiresIn, int64(7200)))
	assert.Check(t, is.Equal(tok.RefreshToken, "exchanged-refresh"))

	assert.Check(t, is.Equal(gotPath, "/oauth/token"))
	assert.Check(t, is.Equal(gotForm.Get("grant_type"), "authorization_code"))
	assert.Check(t, is.Equal(gotForm.Get("code"), "test-code"))
	assert.Check(t, is.Equal(gotForm.Get("code_verifier"), flow.verifier))
}

func TestFlow_Exchange_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "code already redeemed",
		})
	}))
	t.Cleanup(srv.Close)

	flow := startFlow(t, srv.URL)
	_, err := flow.Exchange(context.Background(), "bad-code")
	assert.ErrorContains(t, err, "invalid_grant")
}
