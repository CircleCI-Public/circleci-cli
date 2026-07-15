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

package agent

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantAgent string
	}{
		{
			name:      "clean environment",
			env:       map[string]string{},
			wantAgent: "",
		},
		{
			name:      "empty var is not detected",
			env:       map[string]string{"GEMINI_CLI": ""},
			wantAgent: "",
		},
		{
			name:      "AGENT=amp detected as amp",
			env:       map[string]string{"AGENT": "amp"},
			wantAgent: "amp",
		},
		{
			name:      "AGENT with non-amp value is ignored",
			env:       map[string]string{"AGENT": "other"},
			wantAgent: "",
		},
		{
			name:      "AI_AGENT returns value as agent name",
			env:       map[string]string{"AI_AGENT": "some-agent"},
			wantAgent: "some-agent",
		},
		{
			name:      "AI_AGENT with invalid characters is ignored",
			env:       map[string]string{"AI_AGENT": "bad\nagent"},
			wantAgent: "",
		},
		{
			name:      "AI_AGENT with spaces is ignored",
			env:       map[string]string{"AI_AGENT": "bad agent"},
			wantAgent: "",
		},
		{
			name:      "AI_AGENT takes priority over AGENT",
			env:       map[string]string{"AGENT": "amp", "AI_AGENT": "other"},
			wantAgent: "other",
		},
		{
			name:      "CODEX_SANDBOX",
			env:       map[string]string{"CODEX_SANDBOX": "seatbelt"},
			wantAgent: "codex",
		},
		{
			name:      "CODEX_CI",
			env:       map[string]string{"CODEX_CI": "1"},
			wantAgent: "codex",
		},
		{
			name:      "CODEX_THREAD_ID",
			env:       map[string]string{"CODEX_THREAD_ID": "abc"},
			wantAgent: "codex",
		},
		{
			name:      "GEMINI_CLI",
			env:       map[string]string{"GEMINI_CLI": "1"},
			wantAgent: "gemini-cli",
		},
		{
			name:      "COPILOT_CLI",
			env:       map[string]string{"COPILOT_CLI": "1"},
			wantAgent: "copilot-cli",
		},
		{
			name:      "OPENCODE",
			env:       map[string]string{"OPENCODE": "1"},
			wantAgent: "opencode",
		},
		{
			name:      "CLAUDECODE",
			env:       map[string]string{"CLAUDECODE": "1"},
			wantAgent: "claude-code",
		},
		{
			name:      "AGENT=amp takes priority over CLAUDECODE",
			env:       map[string]string{"AGENT": "amp", "CLAUDECODE": "1"},
			wantAgent: "amp",
		},
		{
			name:      "invalid AI_AGENT falls through to tool-specific detection",
			env:       map[string]string{"AI_AGENT": "bad agent", "GEMINI_CLI": "1"},
			wantAgent: "gemini-cli",
		},
		{
			name:      "CIRCLE_MCP alone produces mcp",
			env:       map[string]string{"CIRCLE_MCP": "1"},
			wantAgent: "mcp",
		},
		{
			name:      "CIRCLE_MCP with CLAUDECODE produces mcp/claude-code",
			env:       map[string]string{"CIRCLE_MCP": "1", "CLAUDECODE": "1"},
			wantAgent: "mcp/claude-code",
		},
		{
			name:      "CIRCLE_MCP with AGENT=amp produces mcp/amp",
			env:       map[string]string{"CIRCLE_MCP": "1", "AGENT": "amp"},
			wantAgent: "mcp/amp",
		},
		{
			name:      "CIRCLE_MCP with GEMINI_CLI produces mcp/gemini-cli",
			env:       map[string]string{"CIRCLE_MCP": "1", "GEMINI_CLI": "1"},
			wantAgent: "mcp/gemini-cli",
		},
		{
			name:      "AI_AGENT takes priority over CIRCLE_MCP composite",
			env:       map[string]string{"CIRCLE_MCP": "1", "CLAUDECODE": "1", "AI_AGENT": "custom"},
			wantAgent: "custom",
		},
	}

	// All env vars that Detect() inspects — must be cleared before each case
	// so ambient vars (e.g. CLAUDECODE=1 from Claude Code) don't leak through.
	allDetectionVars := []string{
		"AI_AGENT", "AGENT", "CIRCLE_MCP",
		"CODEX_SANDBOX", "CODEX_CI", "CODEX_THREAD_ID",
		"GEMINI_CLI", "COPILOT_CLI", "OPENCODE", "CLAUDECODE",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range allDetectionVars {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := Detect()
			assert.Check(t, cmp.Equal(tt.wantAgent, got))
		})
	}
}

func Test_validAgent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "valid lowercase", input: "my-agent", want: true},
		{name: "valid with underscore", input: "my_agent_v2", want: true},
		{name: "valid uppercase", input: "MyAgent", want: true},
		{name: "valid numbers", input: "agent123", want: true},
		{name: "spaces rejected", input: "my agent", want: false},
		{name: "newline rejected", input: "my\nagent", want: false},
		{name: "carriage return rejected", input: "my\ragent", want: false},
		{name: "null byte rejected", input: "my\x00agent", want: false},
		{name: "dot rejected", input: "my.agent", want: false},
		{name: "slash rejected", input: "my/agent", want: false},
		{name: "empty rejected", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validName.MatchString(tt.input)
			assert.Check(t, cmp.Equal(tt.want, got))
		})
	}
}
