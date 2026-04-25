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

package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

// fakePipeline returns a minimal pipeline payload for the fake MCP server tests.
func fakePipelinePayload(id string, number int, slug, branch string) map[string]any {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	return map[string]any{
		"id":           id,
		"state":        "created",
		"number":       number,
		"project_slug": slug,
		"created_at":   now.Format(time.RFC3339),
		"updated_at":   now.Format(time.RFC3339),
		"trigger": map[string]any{
			"type":        "webhook",
			"received_at": now.Format(time.RFC3339),
			"actor":       map[string]any{"login": "testuser", "avatar_url": ""},
		},
		"vcs": map[string]any{
			"revision": "abc1234",
			"branch":   branch,
		},
	}
}

// TestPipelineListHandler_DebugLoggingDoesNotPanic is a regression test for a
// nil-slog panic that crashed the MCP server process on every tool call.
// iostream.Test() previously left slog nil; the HTTP client's debug defer
// hit s.slog.DebugContext and the server exited with "connection closed".
func TestPipelineListHandler_DebugLoggingDoesNotPanic(t *testing.T) {
	const slug = "gh/testorg/testrepo"

	fake := fakes.NewCircleCI(t)
	fake.AddProjectPipelines(slug, fakePipelinePayload("pipeline-abc", 1, slug, "main"))

	t.Setenv("CIRCLECI_TOKEN", "testtoken")
	t.Setenv("CIRCLECI_HOST", fake.URL())

	result, _, err := pipelineListHandler(context.Background(), nil, &pipelineListArgs{
		ProjectSlug: slug,
	})

	assert.NilError(t, err)
	assert.Assert(t, !result.IsError, "expected success, got error result")
	text := result.Content[0].(*sdkmcp.TextContent).Text
	assert.Assert(t, strings.Contains(text, "pipeline-abc"), "expected pipeline ID in output, got: %s", text)
}
