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
	"fmt"
	"os"
	"regexp"
)

const (
	agentAmp        = "amp"
	agentClaudeCode = "claude-code"
	agentCodex      = "codex"
	agentCopilotCLI = "copilot-cli"
	agentGeminiCLI  = "gemini-cli"
	agentMCP        = "mcp"
	agentOpencode   = "opencode"
)

func Detect() string {
	if v, ok := os.LookupEnv("AI_AGENT"); ok && v != "" {
		if validName.MatchString(v) {
			return v
		}
	}

	mcp := isMCP()
	underlying := detectUnderlying()

	if mcp && underlying != "" {
		return fmt.Sprintf("%s/%s", agentMCP, underlying)
	}
	if mcp {
		return agentMCP
	}
	return underlying
}

// isMCP reports whether the CLI is running as a subprocess of the MCP server.
func isMCP() bool {
	v, ok := os.LookupEnv("CIRCLECI_MCP")
	return ok && v != ""
}

// detectUnderlying returns the coding agent name based on well-known environment
// variables, without considering the MCP context.
func detectUnderlying() string {
	// Check AGENT=amp before the more generic CLAUDECODE=1 since Amp sets both.
	if v, ok := os.LookupEnv("AGENT"); ok && v == "amp" {
		return agentAmp
	}

	for ev, name := range isSetMap {
		v, ok := os.LookupEnv(ev)
		if ok && v != "" {
			return name
		}
	}

	return ""
}

var (
	validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	isSetMap = map[string]string{
		// OpenAI Codex CLI — https://github.com/openai/codex
		// CODEX_SANDBOX: https://github.com/openai/codex/blob/95e1d5993985019ce0ce0d10689caf1375f95120/codex-rs/core/src/spawn.rs#L25
		// CODEX_THREAD_ID: https://github.com/openai/codex/blob/95e1d5993985019ce0ce0d10689caf1375f95120/codex-rs/core/src/exec_env.rs#L8
		// CODEX_CI: https://github.com/openai/codex/blob/95e1d5993985019ce0ce0d10689caf1375f95120/codex-rs/core/src/unified_exec/process_manager.rs#L64
		"CODEX_SANDBOX":   agentCodex,
		"CODEX_CI":        agentCodex,
		"CODEX_THREAD_ID": agentCodex,

		// Google Gemini CLI — https://github.com/google-gemini/gemini-cli
		// GEMINI_CLI: https://github.com/google-gemini/gemini-cli/blob/46fd7b4864111032a1c7dfa1821b2000fc7531da/docs/tools/shell.md#L96-L97
		"GEMINI_CLI": agentGeminiCLI,

		// GitHub Copilot CLI
		"COPILOT_CLI": agentCopilotCLI,

		// OpenCode — https://github.com/anomalyco/opencode
		// OPENCODE: https://github.com/anomalyco/opencode/blob/fde201c286a83ff32dda9b41d61d734a4449fe70/packages/opencode/src/index.ts#L78-L80
		"OPENCODE": agentOpencode,

		// Anthropic Claude Code — https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview
		// CLAUDECODE: https://code.claude.com/docs/en/env-vars (CLAUDECODE section)
		// Checked last because other agents (e.g. Amp) set CLAUDECODE=1 alongside their own vars.
		"CLAUDECODE": agentClaudeCode,
	}
)
