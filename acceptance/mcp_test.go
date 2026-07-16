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
	"context"
	"fmt"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

// TestMCPServerAttributesToolCallsAsMCP verifies the CIRCLE_MCP plumbing in
// internal/cmd/root: `mcp start` sets CIRCLE_MCP=1 on itself before serving, so
// every CLI subprocess it spawns per tool call inherits it and agent.Detect()
// reports "mcp/<agent>". Because the var is set in the server's RunE — after the
// root PersistentPreRunE has already run agent.Detect() — the server's *own*
// command_invocation is attributed to the plain underlying agent, not MCP.
//
// The test drives a real `mcp start` server over stdio with the MCP client SDK,
// calls the `circleci_setting_list` tool (no API needed — it reads local
// config), and inspects the telemetry both processes emit to a fake Segment
// endpoint. CLAUDECODE=1 stands in for a real coding agent so the underlying
// name is deterministic.
func TestMCPServerAttributesToolCallsAsMCP(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	env := testenv.New(t)
	env.Telemetry = true
	fs := fakesegment.New(ctx, telemetry.SegmentKey)
	fsSrv := httptest.NewServer(fs)
	t.Cleanup(fsSrv.Close)
	env.Extra["CIRCLE_TELEMETRY_ENDPOINT"] = fsSrv.URL
	// Stand in for a real coding agent so detectUnderlying() is deterministic.
	env.Extra["CLAUDECODE"] = "1"

	// Launch the MCP server as a subprocess over stdio. Its environment is what
	// the per-tool-call subprocesses inherit, so the CIRCLE_MCP=1 that its RunE
	// sets propagates to them.
	serverCmd := exec.Command(binaryPath, "mcp", "start")
	serverCmd.Env = env.Environ()
	serverCmd.Dir = t.TempDir()

	client := mcp.NewClient(&mcp.Implementation{Name: "acceptance-test", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: serverCmd}, nil)
	assert.NilError(t, err)

	// The generated tool schema requires a "flags" object; an empty one is fine.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "circleci_setting_list",
		Arguments: map[string]any{"flags": map[string]any{}},
	})
	assert.NilError(t, err)
	assert.Check(t, !res.IsError, "tool call reported an error")

	// Closing the session shuts down the server, which then flushes its own
	// telemetry. Poll until both events land.
	assert.NilError(t, session.Close())

	poll.WaitOn(t, func(poll.LogT) poll.Result {
		byCommand := map[string]string{} // command path -> agent trait
		for _, batch := range fs.Batches() {
			for _, msg := range batch.Messages {
				if msg.Event != "command_invocation" || msg.Context == nil {
					continue
				}
				command, _ := msg.Properties["command"].(string)
				agent, _ := msg.Context.Traits["agent"].(string)
				byCommand[command] = agent
			}
		}

		// The tool-call subprocess inherits CIRCLE_MCP=1 -> "mcp/claude-code".
		if got, ok := byCommand["circleci setting list"]; !ok {
			return poll.Continue("no telemetry yet for the tool-call subprocess")
		} else if got != "mcp/claude-code" {
			return poll.Error(fmt.Errorf("tool-call subprocess: expected agent %q, got %q", "mcp/claude-code", got))
		}

		// The server process itself detected its agent before RunE set
		// CIRCLE_MCP, so it is attributed to the plain underlying agent.
		if got, ok := byCommand["circleci mcp start"]; !ok {
			return poll.Continue("no telemetry yet for the server process")
		} else if got != "claude-code" {
			return poll.Error(fmt.Errorf("server process: expected agent %q, got %q", "claude-code", got))
		}

		return poll.Success()
	})
}
