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

package apiclient

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Feedback report sources: the CLI run directly, or run as a tool call of the
// CLI's own MCP server.
const (
	FeedbackSourceCLI    = "cli"
	FeedbackSourceCLIMCP = "cli_mcp"
)

// FeedbackReport is the body of POST /api/v3/tool/send-feedback. The endpoint
// rejects unknown members, so nothing outside this shape may be sent.
type FeedbackReport struct {
	Message       string `json:"message"`
	ToolName      string `json:"tool_name,omitempty"`
	Source        string `json:"source,omitempty"`
	AgentType     string `json:"agent_type,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
}

// SendFeedback sends a feedback report to CircleCI and returns the ID of the
// stored report.
func (c *Client) SendFeedback(ctx context.Context, r FeedbackReport) (uuid.UUID, error) {
	var resp v3Entity[struct {
		ID uuid.UUID `json:"id"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/tool/send-feedback",
		httpcl.Body(r),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return uuid.Nil, err
	}
	return resp.Data.ID, nil
}
