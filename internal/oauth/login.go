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

// Package oauth implements the client side of the CircleCI OAuth 2.0
// Authorization Code + PKCE flow (RFC 6749 + RFC 7636 + RFC 8252) with
// Pushed Authorization Requests (RFC 9126).
//
// The flow:
//  1. Start a localhost listener on 127.0.0.1:0.
//  2. Assemble the authorization request — PKCE code challenge, state, and
//     the loopback redirect_uri — and POST it to the server's PAR endpoint
//     (/oauth/par), which returns a request_uri.
//  3. Build a short authorize URL carrying only client_id and request_uri.
//  4. The caller opens that URL in the user's browser.
//  5. Wait for the OAuth provider to redirect to the loopback server with
//     ?code=...&state=... and validate the state.
//  6. Exchange the captured code (with the PKCE verifier) for a token via
//     POST /oauth/token.
//
// Pushing the request keeps sensitive parameters off the browser URL and
// makes the URL short enough to log and click.
//
// Two entry points share the same underlying flow:
//   - Start lands the user on the OAuth login page.
//   - StartSignup appends signup=true so unauthenticated users land on the
//     signup page instead. Everything past the authorize step is identical.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// ClientID is the CircleCI-registered OAuth client identifier for the CLI.
const ClientID = "circleci-cli"

// Flow is an in-progress authorization-code+PKCE exchange.
//
// Typical usage:
//
//	flow, err := oauth.Start(ctx, host)
//	if err != nil { ... }
//	defer flow.Close()
//	// open flow.AuthorizeURL in the user's browser
//	res, err := flow.Wait(ctx)
type Flow struct {
	// AuthorizeURL is the URL the user's browser must visit to start the flow.
	AuthorizeURL string

	cfg      *oauth2.Config
	verifier string
	state    string
	server   *http.Server
	listener net.Listener
	result   chan callbackResult
}

// Result is the outcome of a successful authorization. The Verifier must be
// presented when exchanging Code for an access token.
type Result struct {
	Code     string
	Verifier string
}

// TokenResponse is the parsed response from POST /oauth/token. It mirrors the
// OAuth 2.0 wire format; we expose this rather than oauth2.Token directly so
// the JSON output is deterministic (oauth2.Token.Expiry is computed from
// time.Now() at parse time, which makes it non-reproducible across runs).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type callbackResult struct {
	code string
	err  error
}

// Start binds a loopback listener, generates PKCE + state, and returns a Flow
// whose AuthorizeURL the caller should open in the user's browser. The
// returned Flow owns the listener and HTTP server until Close is called.
//
// host is the CircleCI base URL (e.g. https://circleci.com). The OAuth
// endpoints are derived as host + /oauth/authorize and host + /oauth/token.
// deviceID and osInfo are appended as query parameters on the authorize URL
// so the server can correlate requests by CLI installation and platform.
func Start(ctx context.Context, host, deviceID, osInfo string) (*Flow, error) {
	return start(ctx, host, deviceID, osInfo, false)
}

// StartSignup is like Start, but appends signup=true to the authorize URL so
// the OAuth provider routes unauthenticated users to the signup page rather
// than the login page. The PKCE handshake, loopback callback, and token
// exchange are otherwise identical.
func StartSignup(ctx context.Context, host, deviceID, osInfo string) (*Flow, error) {
	return start(ctx, host, deviceID, osInfo, true)
}

func start(ctx context.Context, host, deviceID, osInfo string, signup bool) (*Flow, error) {
	verifier := oauth2.GenerateVerifier()

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}
	addr := listener.Addr()

	host = strings.TrimRight(host, "/")
	cfg := &oauth2.Config{
		ClientID: ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  host + "/oauth/authorize",
			TokenURL: host + "/oauth/token",
		},
		RedirectURL: fmt.Sprintf("http://%s/callback", addr),
	}

	params := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	if deviceID != "" {
		params = append(params, oauth2.SetAuthURLParam("device_id", deviceID))
	}
	if osInfo != "" {
		params = append(params, oauth2.SetAuthURLParam("os", osInfo))
	}
	if signup {
		params = append(params, oauth2.SetAuthURLParam("signup", "true"))
	}

	// Let the oauth2 library assemble the complete authorization request —
	// response_type, client_id, redirect_uri, state, and the PKCE challenge —
	// then push that exact parameter set to the server (RFC 9126) instead of
	// embedding it in the browser URL. AuthCodeURL builds a URL; we only want
	// its query string as the PAR request body.
	full, err := url.Parse(cfg.AuthCodeURL(state, params...))
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("building authorization request: %w", err)
	}

	requestURI, err := pushAuthorizationRequest(ctx, host+"/oauth/par", full.RawQuery)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	// After a successful push, the browser only needs the request_uri — the
	// server resolves the client and every other parameter from the pushed
	// request it points at.
	authorizeURL := cfg.Endpoint.AuthURL + "?" + url.Values{
		"request_uri": {requestURI},
	}.Encode()

	f := &Flow{
		AuthorizeURL: authorizeURL,
		cfg:          cfg,
		verifier:     verifier,
		state:        state,
		listener:     listener,
		result:       make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback", f.handleCallback)
	f.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = f.server.Serve(listener)
	}()

	return f, nil
}

// Wait blocks until the loopback server receives a callback from the OAuth
// provider, ctx is cancelled, or the result is otherwise resolved.
func (f *Flow) Wait(ctx context.Context) (*Result, error) {
	select {
	case res := <-f.result:
		if res.err != nil {
			return nil, res.err
		}
		return &Result{Code: res.code, Verifier: f.verifier}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Exchange swaps an authorization code for an access token by POSTing to the
// configured /oauth/token endpoint with PKCE verifier. The verifier captured
// during Start is reused.
func (f *Flow) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	tok, err := f.cfg.Exchange(ctx, code, oauth2.VerifierOption(f.verifier))
	if err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		ExpiresIn:    tok.ExpiresIn,
		RefreshToken: tok.RefreshToken,
	}, nil
}

// parResponse is the response to a pushed authorization request
// (RFC 9126 §2.2): the request_uri the browser presents to the authorization
// endpoint, plus how many seconds it remains valid.
type parResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int64  `json:"expires_in"`
}

// pushAuthorizationRequest POSTs an assembled authorization request to the
// server's PAR endpoint (RFC 9126) and returns the request_uri the browser
// must present at the authorization endpoint. body is the urlencoded query
// string of the authorization parameters.
func pushAuthorizationRequest(ctx context.Context, endpoint, body string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building pushed authorization request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := parHTTPClient(ctx).Do(req)
	if err != nil {
		return "", fmt.Errorf("pushing authorization request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading pushed authorization response: %w", err)
	}

	// RFC 9126 §2.2: a successful push returns 201 Created.
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("pushed authorization request rejected (%s): %s",
			resp.Status, strings.TrimSpace(string(data)))
	}

	var par parResponse
	if err := json.Unmarshal(data, &par); err != nil {
		return "", fmt.Errorf("decoding pushed authorization response: %w", err)
	}
	if par.RequestURI == "" {
		return "", errors.New("pushed authorization response did not include a request_uri")
	}
	return par.RequestURI, nil
}

// parHTTPClient returns the HTTP client to use for the PAR call. It honors an
// *http.Client stashed in ctx under oauth2.HTTPClient — the same override the
// oauth2 library uses for token exchange — and otherwise uses the default
// client, so PAR and Exchange always travel the same path.
func parHTTPClient(ctx context.Context) *http.Client {
	if c, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok && c != nil {
		return c
	}
	return http.DefaultClient
}

// Close shuts down the loopback server. Safe to call multiple times.
func (f *Flow) Close() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return f.server.Shutdown(shutdownCtx)
}

func (f *Flow) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/callback" {
		http.NotFound(w, r)
		return
	}

	q := r.URL.Query()

	if errParam := q.Get("error"); errParam != "" {
		msg := errParam
		if desc := q.Get("error_description"); desc != "" {
			msg = errParam + ": " + desc
		}
		writeBrowserResponse(w, http.StatusBadRequest, browserState{
			Title: "Authorization failed",
			Body:  msg,
		})
		f.deliver(callbackResult{err: fmt.Errorf("authorization failed: %s", msg)})
		return
	}

	if got := q.Get("state"); got != f.state {
		writeBrowserResponse(w, http.StatusBadRequest, browserState{
			Title: "Authorization failed",
			Body:  "The state parameter did not match. This may indicate a CSRF attempt.",
		})
		f.deliver(callbackResult{err: errors.New("state parameter does not match the CLI's expected value")})
		return
	}

	code := q.Get("code")
	if code == "" {
		writeBrowserResponse(w, http.StatusBadRequest, browserState{
			Title: "Authorization failed",
			Body:  "The authorization response did not include a code.",
		})
		f.deliver(callbackResult{err: errors.New("authorization response missing code")})
		return
	}

	writeBrowserResponse(w, http.StatusOK, browserState{
		Title:   "Authorization successful",
		Body:    "You can close this window and return to your terminal.",
		Success: true,
	})
	f.deliver(callbackResult{code: code})
}

// deliver sends res on the result channel non-blockingly. The channel is
// buffered to size 1, so duplicate callbacks (which shouldn't happen but
// might from misbehaving clients) are dropped rather than blocking the
// HTTP handler.
func (f *Flow) deliver(res callbackResult) {
	select {
	case f.result <- res:
	default:
	}
}

func writeBrowserResponse(w http.ResponseWriter, status int, state browserState) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	renderPage(w, state)
}

// generateState returns a 32-character hex string suitable for the OAuth
// `state` parameter (16 bytes of CSPRNG output).
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
