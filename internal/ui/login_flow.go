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

package ui

import (
	"context"
	"net/url"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pkg/browser"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/oauth"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/ui/theme"
)

const defaultHost = "https://circleci.com"

type loginStage int

const (
	stageHostSelect   loginStage = iota
	stageHostInput               // custom base URL entry
	stageMethodSelect            // browser vs token
	stageTokenInput              // paste-a-token text entry
	stageEnterPrompt             // "Press Enter to open browser..."
	stageWaiting                 // spinner while browser auth completes
	stageDone
)

// LoginMethod indicates which authentication path the user selected.
type LoginMethod int

// LoginMethodBrowser selects the browser-based OAuth PKCE flow.
const (
	LoginMethodBrowser LoginMethod = iota
)

// LoginResult is the outcome of a completed LoginFlowModel run.
type LoginResult struct {
	Cancelled bool
	Host      string // resolved base URL
	Token     string // set when auth succeeds (either method)
	Username  string // set when GetUsername is provided and succeeds
	Err       error
}

// LoginFlowOptions configures a LoginFlowModel.
type LoginFlowOptions struct {
	DeviceID string
	OSInfo   string
	// GetUsername, if non-nil, is called after token exchange to display
	// the authenticated user's login name.
	GetUsername func(ctx context.Context, host, token string) (string, error)
	Color       bool
}

// LoginFlowModel is a multi-stage bubbletea model that walks the user through
// CircleCI authentication:
//
//  1. Pick a CircleCI host (circleci.com or a custom URL).
//  2. Pick an auth method (browser OAuth or paste a token).
//     3a. For browser OAuth: press Enter → open browser → wait for callback.
//     3b. For token: type/paste the personal access token.
//
// After tea.Program.Run() returns, call Result() to read the outcome and
// Close() to release the OAuth server if one was started.
type LoginFlowModel struct {
	ctx  context.Context
	opts LoginFlowOptions

	width        int
	stage        loginStage
	hostSelect   components.SelectModel // stage 1 — host picker
	hostInput    textinput.Model
	methodSelect components.SelectModel // stage 2 — auth method picker
	tokenInput   components.TokenModel  // stage 3b — paste-a-token

	flow   *oauth.Flow
	spin   spinner.Model
	result LoginResult
}

// async message types
type oauthStartedMsg struct {
	flow *oauth.Flow
	err  error
}
type oauthCallbackMsg struct {
	result *oauth.Result
	err    error
}
type tokenExchangedMsg struct {
	token string
	err   error
}
type usernameFetchedMsg struct {
	username string
	err      error
}

var loginMethodOptions = []string{"Login with a web browser", "Paste an authentication token"}

// NewLoginFlow returns a LoginFlowModel ready to pass to tea.NewProgram.
func NewLoginFlow(ctx context.Context, opts LoginFlowOptions) LoginFlowModel {
	ti := textinput.New()
	ti.Placeholder = "https://example.circleci.com"
	ti.SetWidth(50)
	ti.SetVirtualCursor(false)

	s := components.NewSpinner(opts.Color)

	return LoginFlowModel{
		ctx:        ctx,
		opts:       opts,
		stage:      stageHostSelect,
		hostSelect: newHostSelect(),
		hostInput:  ti,
		tokenInput: components.NewTokenModel(),
		spin:       s,
	}
}

// newHostSelect returns a fresh host-picker components.SelectModel.
func newHostSelect() components.SelectModel {
	return components.NewSelectModel(
		"Where do you use CircleCI?",
		[]string{"circleci.com", "Other"},
	)
}

// newMethodSelect returns a fresh auth-method components.SelectModel for the given host.
func newMethodSelect(host string) components.SelectModel {
	return components.NewSelectModel(
		"How would you like to authenticate "+loginHostDisplay(host)+"?",
		loginMethodOptions,
	).WithHint("(↑/↓ to move, enter to select, esc to go back)")
}

// Result returns the final login outcome. Only valid after tea.Program.Run() returns.
func (m LoginFlowModel) Result() LoginResult { return m.result }

// Close shuts down the OAuth callback server if one was started.
func (m LoginFlowModel) Close() {
	if m.flow != nil {
		_ = m.flow.Close()
	}
}

func (m LoginFlowModel) Init() tea.Cmd { return nil }

func (m LoginFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}

	// Async OAuth messages are handled regardless of stage.
	switch msg := msg.(type) {
	case oauthStartedMsg:
		return m.onOAuthStarted(msg)
	case oauthCallbackMsg:
		return m.onOAuthCallback(msg)
	case tokenExchangedMsg:
		return m.onTokenExchanged(msg)
	case usernameFetchedMsg:
		return m.onUsernameFetched(msg)
	}

	switch m.stage {
	case stageHostSelect:
		return m.updateHostSelect(msg)
	case stageHostInput:
		return m.updateHostInput(msg)
	case stageMethodSelect:
		return m.updateMethodSelect(msg)
	case stageTokenInput:
		return m.updateTokenInput(msg)
	case stageEnterPrompt:
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.keyEnterPrompt(keyMsg)
		}
	case stageWaiting:
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == components.KeyCtrlC {
			m.result.Cancelled = true
			return m, tea.Quit
		}
		s, cmd := m.spin.Update(msg)
		m.spin = s
		return m, cmd
	case stageDone:
	}
	return m, nil
}

// updateHostSelect intercepts navigation keys then delegates to the embedded
// components.SelectModel.
func (m LoginFlowModel) updateHostSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC, components.KeyEsc:
			m.result.Cancelled = true
			return m, tea.Quit
		}
	}

	updated, subCmd := m.hostSelect.Update(msg)
	m.hostSelect = updated.(components.SelectModel)

	if !m.hostSelect.Done() {
		return m, subCmd
	}
	if m.hostSelect.Selected() == 1 { // "Other"
		m.stage = stageHostInput
		m.hostInput.Focus()
		return m, textinput.Blink
	}

	m.result.Host = defaultHost
	m.methodSelect = newMethodSelect(m.result.Host)
	m.stage = stageMethodSelect
	return m, nil
}

// updateHostInput handles key and text-input messages for the custom URL stage.
func (m LoginFlowModel) updateHostInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		return m.keyHostInput(keyMsg)
	}
	var cmd tea.Cmd
	m.hostInput, cmd = m.hostInput.Update(msg)
	return m, cmd
}

// updateMethodSelect intercepts Esc (go back) and Ctrl+C (quit), then
// delegates remaining input to the embedded components.SelectModel.
func (m LoginFlowModel) updateMethodSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC:
			m.result.Cancelled = true
			return m, tea.Quit
		case components.KeyEsc:
			// Go back: restore fresh host select, or return to URL input.
			if m.result.Host == defaultHost {
				m.hostSelect = newHostSelect()
				m.stage = stageHostSelect
				return m, nil
			}
			m.stage = stageHostInput
			m.hostInput.Focus()
			return m, textinput.Blink
		}
	}

	updated, subCmd := m.methodSelect.Update(msg)
	m.methodSelect = updated.(components.SelectModel)

	if !m.methodSelect.Done() {
		return m, subCmd
	}

	if m.methodSelect.Selected() == 1 {
		m.stage = stageTokenInput
		return m, textinput.Blink
	}
	return m, m.cmdStartOAuth()
}

func (m LoginFlowModel) keyHostInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case components.KeyCtrlC:
		m.result.Cancelled = true
		return m, tea.Quit
	case components.KeyEsc:
		m.hostInput.Blur()
		m.hostSelect = newHostSelect()
		m.stage = stageHostSelect
		return m, nil
	case components.KeyEnter:
		raw := strings.TrimSpace(m.hostInput.Value())
		if raw == "" {
			return m, nil
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			raw = "https://" + raw
		}
		if _, err := url.ParseRequestURI(raw); err != nil {
			return m, nil
		}
		m.result.Host = strings.TrimRight(raw, "/")
		m.methodSelect = newMethodSelect(m.result.Host)
		m.stage = stageMethodSelect
		return m, nil
	}
	var cmd tea.Cmd
	m.hostInput, cmd = m.hostInput.Update(msg)
	return m, cmd
}

// updateTokenInput intercepts navigation keys then delegates to the embedded
// components.TokenModel. Esc goes back to the method picker; Ctrl+C cancels.
func (m LoginFlowModel) updateTokenInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC:
			m.result.Cancelled = true
			return m, tea.Quit
		case components.KeyEsc:
			m.tokenInput = components.NewTokenModel()
			m.methodSelect = newMethodSelect(m.result.Host)
			m.stage = stageMethodSelect
			return m, nil
		}
	}

	updated, subCmd := m.tokenInput.Update(msg)
	m.tokenInput = updated.(components.TokenModel)

	tok := m.tokenInput.Token()
	if tok == "" {
		return m, subCmd
	}
	m.result.Token = tok
	if m.opts.GetUsername != nil {
		return m, m.cmdGetUsername(tok)
	}
	m.stage = stageDone
	return m, tea.Quit
}

func (m LoginFlowModel) keyEnterPrompt(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case components.KeyCtrlC, components.KeyEsc:
		m.result.Cancelled = true
		return m, tea.Quit
	case components.KeyEnter:
		_ = browser.OpenURL(m.flow.AuthorizeURL)
		m.stage = stageWaiting
		return m, m.spin.Tick
	}
	return m, nil
}

func (m LoginFlowModel) onOAuthStarted(msg oauthStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = stageDone
		return m, tea.Quit
	}
	m.flow = msg.flow
	m.stage = stageEnterPrompt
	// Start listening for the callback immediately so the user can open the
	// URL manually without having to press Enter first.
	return m, m.cmdWaitCallback()
}

func (m LoginFlowModel) onOAuthCallback(msg oauthCallbackMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = stageDone
		return m, tea.Quit
	}
	// The callback may arrive before the user pressed Enter (manual URL open),
	// so ensure we show the waiting spinner during the token exchange.
	m.stage = stageWaiting
	return m, tea.Batch(m.spin.Tick, m.cmdExchange(msg.result))
}

func (m LoginFlowModel) onTokenExchanged(msg tokenExchangedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = stageDone
		return m, tea.Quit
	}
	m.result.Token = msg.token
	if m.opts.GetUsername != nil {
		return m, m.cmdGetUsername(msg.token)
	}
	m.stage = stageDone
	return m, tea.Quit
}

func (m LoginFlowModel) onUsernameFetched(msg usernameFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Token = ""
	} else {
		m.result.Username = msg.username
	}
	m.stage = stageDone
	return m, tea.Quit
}

// --- commands ---

func (m LoginFlowModel) cmdStartOAuth() tea.Cmd {
	ctx, host, deviceID, osInfo := m.ctx, m.result.Host, m.opts.DeviceID, m.opts.OSInfo
	return func() tea.Msg {
		flow, err := oauth.Start(ctx, host, deviceID, osInfo)
		return oauthStartedMsg{flow: flow, err: err}
	}
}

func (m LoginFlowModel) cmdWaitCallback() tea.Cmd {
	ctx, flow := m.ctx, m.flow
	return func() tea.Msg {
		res, err := flow.Wait(ctx)
		return oauthCallbackMsg{result: res, err: err}
	}
}

func (m LoginFlowModel) cmdExchange(res *oauth.Result) tea.Cmd {
	ctx, flow := m.ctx, m.flow
	return func() tea.Msg {
		tok, err := flow.Exchange(ctx, res.Code)
		if err != nil {
			return tokenExchangedMsg{err: err}
		}
		return tokenExchangedMsg{token: tok.AccessToken}
	}
}

func (m LoginFlowModel) cmdGetUsername(token string) tea.Cmd {
	ctx, host, fn := m.ctx, m.result.Host, m.opts.GetUsername
	return func() tea.Msg {
		username, err := fn(ctx, host, token)
		return usernameFetchedMsg{username: username, err: err}
	}
}

// --- views ---

func (m LoginFlowModel) View() tea.View {
	var b strings.Builder
	switch m.stage {
	case stageHostSelect:
		return m.hostSelect.View()
	case stageHostInput:
		return m.viewHostInput()
	case stageMethodSelect:
		return m.methodSelect.View()
	case stageTokenInput:
		return m.tokenInput.View()
	case stageEnterPrompt:
		m.renderEnterPrompt(&b)
	case stageWaiting:
		m.renderWaiting(&b)
	case stageDone:
		m.renderDone(&b)
	}
	return tea.NewView(b.String())
}

func (m LoginFlowModel) viewHostInput() tea.View {
	header := theme.TitleStyle.Render("? Base URL:")

	var c *tea.Cursor
	if !m.hostInput.VirtualCursor() {
		c = m.hostInput.Cursor()
		c.Y += lipgloss.Height(header)
	}

	str := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		m.hostInput.View(),
		theme.HelperStyle.Render("(enter to confirm, esc to go back)"),
	)
	v := tea.NewView(str)
	v.Cursor = c
	return v
}

func (m LoginFlowModel) renderEnterPrompt(b *strings.Builder) {
	authURL := m.flow.AuthorizeURL
	if m.width > 0 {
		authURL = hardWrap(authURL, m.width)
	}
	b.WriteString(components.LinkStyle.Hyperlink(authURL).Render(authURL) + "\n")
	b.WriteString(theme.HelperStyle.Render("Press Enter to open in your browser..."))
}

func (m LoginFlowModel) renderWaiting(b *strings.Builder) {
	b.WriteString(m.spin.View() + " Waiting for browser authentication...")
}

func (m LoginFlowModel) renderDone(b *strings.Builder) {
	if m.result.Err != nil {
		b.WriteString(theme.ErrorStyle.Render(theme.IconFail) + " " + m.result.Err.Error() + "\n")
		return
	}
	b.WriteString(theme.SuccessStyle.Render(theme.IconOK) + " Authentication complete.\n")
	if m.result.Username != "" {
		b.WriteString(theme.SuccessStyle.Render(theme.IconOK) + " Logged in as " + m.result.Username + ".\n")
	}
}

// hardWrap breaks s into lines of at most width characters. OAuth URLs contain
// no spaces, so word-wrapping would never break them.
func hardWrap(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	var b strings.Builder
	for len(s) > width {
		b.WriteString(s[:width])
		b.WriteByte('\n')
		s = s[width:]
	}
	b.WriteString(s)
	return b.String()
}

// loginHostDisplay returns the bare hostname from a base URL for display.
func loginHostDisplay(host string) string {
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		return u.Host
	}
	return host
}
