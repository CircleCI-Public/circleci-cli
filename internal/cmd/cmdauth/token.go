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

package cmdauth

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Set personal access token for authenticating to CircleCI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			if !streams.IsInteractive() {
				return clierrors.New("auth.token.aborted", "Login aborted",
					"Login requires an interactive session.").
					WithExitCode(clierrors.ExitCancelled)
			}
			secureStorage := cmdutil.IsSecureStorage(cmd)
			return runToken(ctx, secureStorage, streams)
		},
	}
	return cmd
}

func runToken(ctx context.Context, secureStorage bool, streams iostream.Streams) error {
	cfg, err := config.Load(ctx, secureStorage)
	if err != nil {
		return clierrors.New("settings.load_failed", "Failed to load settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	p := tea.NewProgram(initialTokenModel())
	anyModel, err := p.Run()
	if err != nil {
		return err
	}

	m := anyModel.(tokenModel)
	if m.quitting {
		return nil
	}

	cfg.Token = m.token
	if err := config.Save(ctx, cfg, secureStorage); err != nil {
		return clierrors.New("settings.save_failed", "Failed to save settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	path, _ := config.Path()
	if secureStorage {
		path = "keyring"
	}
	streams.ErrPrintf("%s Saved %s to %s\n", streams.Symbol("✓", "OK:"), "token", path)
	return nil
}

type tokenModel struct {
	textInput textinput.Model
	quitting  bool
	token     string
}

func initialTokenModel() tokenModel {
	ti := textinput.New()
	ti.Placeholder = "CCIPAT_XXXXXXXXXXXXXXXXXXXXXX_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.CharLimit = len(ti.Placeholder)
	ti.SetWidth(len(ti.Placeholder))
	ti.EchoMode = textinput.EchoPassword

	return tokenModel{textInput: ti}
}

func (m tokenModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tokenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.token = m.textInput.Value()
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m tokenModel) View() tea.View {
	if m.token != "" {
		return tea.NewView("")
	}

	var c *tea.Cursor
	if !m.textInput.VirtualCursor() {
		c = m.textInput.Cursor()
		c.Y += lipgloss.Height(m.headerView())
	}

	str := lipgloss.JoinVertical(lipgloss.Top, m.headerView(), m.textInput.View(), m.footerView())
	if m.quitting {
		str += "\n"
	}

	v := tea.NewView(str)
	v.Cursor = c
	return v
}

func (m tokenModel) headerView() string { return "Enter CircleCI personal access token\n" }
func (m tokenModel) footerView() string { return "\n(esc to quit)" }
