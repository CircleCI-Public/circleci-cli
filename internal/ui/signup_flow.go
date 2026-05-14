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
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/pkg/browser"

	"github.com/CircleCI-Public/circleci-cli/internal/oauth"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

type signupStage int

const (
	signupStageStarting signupStage = iota
	signupStagePrompt
	signupStageCheckingCallback
	signupStageLoginWaiting
	signupStageDone
)

type SignupFailure int

const (
	// SignupFailureNone means the flow did not fail.
	SignupFailureNone SignupFailure = iota
	// SignupFailureSignupListen means the signup callback server could not start.
	SignupFailureSignupListen
	// SignupFailureSignupCallback means the signup callback returned an error.
	SignupFailureSignupCallback
	// SignupFailureSignupExchange means the signup authorization code could not be exchanged.
	SignupFailureSignupExchange
	// SignupFailureLoginListen means the fallback login callback server could not start.
	SignupFailureLoginListen
	// SignupFailureLoginCallback means the fallback login callback returned an error.
	SignupFailureLoginCallback
	// SignupFailureLoginExchange means the fallback login authorization code could not be exchanged.
	SignupFailureLoginExchange
	// SignupFailureUsername means the authenticated username lookup failed.
	SignupFailureUsername
)

type SignupResult struct {
	Cancelled bool
	Host      string
	Token     string
	Username  string
	Err       error
	Failure   SignupFailure
}

type SignupFlowOptions struct {
	Host                  string
	DeviceID              string
	OSInfo                string
	NoBrowser             bool
	Color                 bool
	SignupCallbackTimeout time.Duration
	LoginCallbackTimeout  time.Duration
	GetUsername           func(ctx context.Context, host, token string) (string, error)
}

type SignupFlowModel struct {
	ctx  context.Context
	opts SignupFlowOptions

	width  int
	stage  signupStage
	spin   spinner.Model
	signup *oauth.Flow
	login  *oauth.Flow
	result SignupResult
}

type signupOAuthStartedMsg struct {
	flow *oauth.Flow
	err  error
}

type signupCallbackCheckedMsg struct {
	result *oauth.Result
	err    error
}

type signupTokenExchangedMsg struct {
	token string
	err   error
}

type signupLoginStartedMsg struct {
	flow *oauth.Flow
	err  error
}

type signupLoginCallbackMsg struct {
	result *oauth.Result
	err    error
}

type signupLoginTokenExchangedMsg struct {
	token string
	err   error
}

type signupUsernameFetchedMsg struct {
	username string
	err      error
}

func NewSignupFlow(ctx context.Context, opts SignupFlowOptions) SignupFlowModel {
	if opts.SignupCallbackTimeout <= 0 {
		opts.SignupCallbackTimeout = time.Second
	}
	if opts.LoginCallbackTimeout <= 0 {
		opts.LoginCallbackTimeout = 5 * time.Minute
	}
	return SignupFlowModel{
		ctx:   ctx,
		opts:  opts,
		stage: signupStageStarting,
		spin:  components.NewSpinner(opts.Color),
		result: SignupResult{
			Host: opts.Host,
		},
	}
}

func (m SignupFlowModel) Result() SignupResult { return m.result }

func (m SignupFlowModel) Close() {
	if m.signup != nil {
		_ = m.signup.Close()
	}
	if m.login != nil {
		_ = m.login.Close()
	}
}

func (m SignupFlowModel) Init() tea.Cmd {
	return m.cmdStartSignup()
}

func (m SignupFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}

	switch msg := msg.(type) {
	case signupOAuthStartedMsg:
		return m.onSignupStarted(msg)
	case signupCallbackCheckedMsg:
		return m.onSignupCallbackChecked(msg)
	case signupTokenExchangedMsg:
		return m.onSignupTokenExchanged(msg)
	case signupLoginStartedMsg:
		return m.onLoginStarted(msg)
	case signupLoginCallbackMsg:
		return m.onLoginCallback(msg)
	case signupLoginTokenExchangedMsg:
		return m.onLoginTokenExchanged(msg)
	case signupUsernameFetchedMsg:
		return m.onUsernameFetched(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case components.KeyCtrlC, components.KeyEsc:
			m.result.Cancelled = true
			return m, tea.Quit
		case components.KeyEnter:
			if m.stage == signupStagePrompt {
				m.stage = signupStageCheckingCallback
				return m, tea.Batch(m.spin.Tick, m.cmdCheckSignupCallback())
			}
		}
	}

	if m.stage == signupStageCheckingCallback || m.stage == signupStageLoginWaiting {
		s, cmd := m.spin.Update(msg)
		m.spin = s
		return m, cmd
	}

	return m, nil
}

func (m SignupFlowModel) onSignupStarted(msg signupOAuthStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureSignupListen
		m.stage = signupStageDone
		return m, tea.Quit
	}
	m.signup = msg.flow
	m.stage = signupStagePrompt
	if m.opts.NoBrowser {
		return m, nil
	}
	return m, m.cmdOpenURL(msg.flow.AuthorizeURL)
}

func (m SignupFlowModel) onSignupCallbackChecked(msg signupCallbackCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureSignupCallback
		m.stage = signupStageDone
		return m, tea.Quit
	}
	if msg.result == nil {
		if m.signup != nil {
			_ = m.signup.Close()
			m.signup = nil
		}
		return m, m.cmdStartLogin()
	}
	return m, m.cmdExchangeSignup(msg.result)
}

func (m SignupFlowModel) onSignupTokenExchanged(msg signupTokenExchangedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureSignupExchange
		m.stage = signupStageDone
		return m, tea.Quit
	}
	return m.afterToken(msg.token)
}

func (m SignupFlowModel) onLoginStarted(msg signupLoginStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureLoginListen
		m.stage = signupStageDone
		return m, tea.Quit
	}
	m.login = msg.flow
	m.stage = signupStageLoginWaiting

	cmds := []tea.Cmd{m.spin.Tick, m.cmdWaitLoginCallback()}
	if !m.opts.NoBrowser {
		cmds = append(cmds, m.cmdOpenURL(msg.flow.AuthorizeURL))
	}
	return m, tea.Batch(cmds...)
}

func (m SignupFlowModel) onLoginCallback(msg signupLoginCallbackMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureLoginCallback
		m.stage = signupStageDone
		return m, tea.Quit
	}
	return m, m.cmdExchangeLogin(msg.result)
}

func (m SignupFlowModel) onLoginTokenExchanged(msg signupLoginTokenExchangedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureLoginExchange
		m.stage = signupStageDone
		return m, tea.Quit
	}
	return m.afterToken(msg.token)
}

func (m SignupFlowModel) afterToken(token string) (tea.Model, tea.Cmd) {
	m.result.Token = token
	if m.opts.GetUsername != nil {
		return m, m.cmdGetUsername(token)
	}
	m.stage = signupStageDone
	return m, tea.Quit
}

func (m SignupFlowModel) onUsernameFetched(msg signupUsernameFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.result.Failure = SignupFailureUsername
		m.result.Token = ""
	} else {
		m.result.Username = msg.username
	}
	m.stage = signupStageDone
	return m, tea.Quit
}

func (m SignupFlowModel) cmdStartSignup() tea.Cmd {
	ctx, host, deviceID, osInfo := m.ctx, m.opts.Host, m.opts.DeviceID, m.opts.OSInfo
	return func() tea.Msg {
		flow, err := oauth.StartSignup(ctx, host, deviceID, osInfo)
		return signupOAuthStartedMsg{flow: flow, err: err}
	}
}

func (m SignupFlowModel) cmdCheckSignupCallback() tea.Cmd {
	ctx, flow, timeout := m.ctx, m.signup, m.opts.SignupCallbackTimeout
	return func() tea.Msg {
		waitCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		res, err := flow.Wait(waitCtx)
		if errors.Is(err, context.DeadlineExceeded) {
			return signupCallbackCheckedMsg{}
		}
		return signupCallbackCheckedMsg{result: res, err: err}
	}
}

func (m SignupFlowModel) cmdStartLogin() tea.Cmd {
	ctx, host, deviceID, osInfo := m.ctx, m.opts.Host, m.opts.DeviceID, m.opts.OSInfo
	return func() tea.Msg {
		flow, err := oauth.Start(ctx, host, deviceID, osInfo)
		return signupLoginStartedMsg{flow: flow, err: err}
	}
}

func (m SignupFlowModel) cmdWaitLoginCallback() tea.Cmd {
	ctx, flow, timeout := m.ctx, m.login, m.opts.LoginCallbackTimeout
	return func() tea.Msg {
		waitCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		res, err := flow.Wait(waitCtx)
		return signupLoginCallbackMsg{result: res, err: err}
	}
}

func (m SignupFlowModel) cmdExchangeSignup(res *oauth.Result) tea.Cmd {
	ctx, flow := m.ctx, m.signup
	return func() tea.Msg {
		tok, err := flow.Exchange(ctx, res.Code)
		if err != nil {
			return signupTokenExchangedMsg{err: err}
		}
		return signupTokenExchangedMsg{token: tok.AccessToken}
	}
}

func (m SignupFlowModel) cmdExchangeLogin(res *oauth.Result) tea.Cmd {
	ctx, flow := m.ctx, m.login
	return func() tea.Msg {
		tok, err := flow.Exchange(ctx, res.Code)
		if err != nil {
			return signupLoginTokenExchangedMsg{err: err}
		}
		return signupLoginTokenExchangedMsg{token: tok.AccessToken}
	}
}

func (m SignupFlowModel) cmdGetUsername(token string) tea.Cmd {
	ctx, host, fn := m.ctx, m.result.Host, m.opts.GetUsername
	return func() tea.Msg {
		username, err := fn(ctx, host, token)
		return signupUsernameFetchedMsg{username: username, err: err}
	}
}

func (m SignupFlowModel) cmdOpenURL(url string) tea.Cmd {
	return func() tea.Msg {
		_ = browser.OpenURL(url)
		return nil
	}
}

func (m SignupFlowModel) View() tea.View {
	var b strings.Builder
	switch m.stage {
	case signupStageStarting:
		b.WriteString(m.spin.View() + " Preparing signup...")
	case signupStagePrompt:
		m.renderSignupPrompt(&b)
	case signupStageCheckingCallback:
		b.WriteString(m.spin.View() + " Checking signup authentication...")
	case signupStageLoginWaiting:
		m.renderLoginWaiting(&b)
	case signupStageDone:
		m.renderSignupDone(&b)
	}
	return tea.NewView(b.String())
}

func (m SignupFlowModel) renderSignupPrompt(b *strings.Builder) {
	b.WriteString("Open this URL in your browser to sign up:\n\n")
	b.WriteString("  " + m.link(m.signup.AuthorizeURL) + "\n\n")
	b.WriteString(theme.HelperStyle.Render("Once you're signed in to CircleCI in the browser, press Enter here to continue with CLI authentication..."))
}

func (m SignupFlowModel) renderLoginWaiting(b *strings.Builder) {
	b.WriteString("Open this URL in your browser to continue:\n\n")
	b.WriteString("  " + m.link(m.login.AuthorizeURL) + "\n\n")
	b.WriteString(m.spin.View() + " Waiting for browser authentication...")
}

func (m SignupFlowModel) renderSignupDone(b *strings.Builder) {
	if m.result.Err != nil {
		b.WriteString(theme.ErrorStyle.Render(theme.IconFail) + " " + m.result.Err.Error() + "\n")
		return
	}
	b.WriteString(theme.SuccessStyle.Render(theme.IconOK) + " Authentication complete.\n")
	if m.result.Username != "" {
		b.WriteString(theme.SuccessStyle.Render(theme.IconOK) + " Logged in as " + m.result.Username + ".\n")
	}
}

func (m SignupFlowModel) link(raw string) string {
	if m.width > 0 {
		raw = hardWrap(raw, m.width)
	}
	return components.LinkStyle.Hyperlink(raw).Render(raw)
}
