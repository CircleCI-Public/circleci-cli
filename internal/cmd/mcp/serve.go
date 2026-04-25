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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/MakeNowJust/heredoc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/pipeline"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newServeCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		Long: heredoc.Doc(`
			Start an MCP server over stdio, exposing CircleCI pipeline operations
			as tools for AI agents such as Claude Code.

			The server reads your existing CircleCI credentials (token, host) from
			the standard config file or environment variables, and inherits your
			current working directory so git-based project detection works exactly
			as it does in the CLI.

			JSON fields for each tool are documented in the tool descriptions.
		`),
		Example: heredoc.Doc(`
			# Add to Claude Code's MCP configuration (~/.claude.json or project .claude.json):
			{
			  "mcpServers": {
			    "circleci": {
			      "command": "circleci",
			      "args": ["mcp", "serve"]
			    }
			  }
			}

			# Verify the server starts cleanly
			$ echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' | circleci mcp serve

			# Run with a custom config file
			$ circleci --config /path/to/config.yml mcp serve
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			return runServe(ctx, version)
		},
	}
}

func runServe(ctx context.Context, version string) error {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "circleci",
		Version: version,
	}, nil)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "pipeline_list",
		Description: "List recent pipelines for a CircleCI project. " +
			"JSON fields: id, number, state, project_slug, branch, revision, created_at, trigger{type,actor}.",
	}, pipelineListHandler)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "pipeline_get",
		Description: "Get a pipeline's status, including its workflows and job details. " +
			"JSON fields: id, number, status, project_slug, branch, revision, created_at, updated_at, " +
			"trigger{type,actor}, errors[], workflows[]{id,name,status,jobs[]{number,name,status,type}}.",
	}, pipelineGetHandler)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "pipeline_trigger",
		Description: "Trigger a new pipeline for a CircleCI project. " +
			"JSON fields: id, number, state, created_at.",
	}, pipelineTriggerHandler)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "pipeline_cancel",
		Description: "Cancel a running pipeline by number or UUID. " +
			"Cancels all active workflows within it; completed workflows are unaffected.",
	}, pipelineCancelHandler)

	return server.Run(ctx, &sdkmcp.StdioTransport{})
}

// --- pipeline_list ---

type pipelineListArgs struct {
	ProjectSlug string `json:"project_slug,omitempty" jsonschema:"Project slug (e.g. gh/org/repo); inferred from the git remote of the current directory if omitted"`
	Branch      string `json:"branch,omitempty"       jsonschema:"Filter results to this branch; optional"`
	Limit       int    `json:"limit,omitempty"        jsonschema:"Maximum number of pipelines to return; defaults to 10"`
}

func pipelineListHandler(ctx context.Context, _ *sdkmcp.CallToolRequest, args *pipelineListArgs) (*sdkmcp.CallToolResult, any, error) {
	client, err := cmdutil.LoadClientForMCP(ctx)
	if err != nil {
		return toolErr(err)
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	var buf bytes.Buffer
	runCtx := iostream.Test(ctx, &buf, io.Discard)
	if err := pipeline.RunList(runCtx, client, args.ProjectSlug, args.Branch, limit, true); err != nil {
		return toolErr(err)
	}
	return toolText(buf.String())
}

// --- pipeline_get ---

type pipelineGetArgs struct {
	PipelineID  string `json:"pipeline_id,omitempty"  jsonschema:"Pipeline UUID or number; inferred from the current branch if omitted"`
	ProjectSlug string `json:"project_slug,omitempty" jsonschema:"Project slug (e.g. gh/org/repo); required when pipeline_id is a number"`
	Branch      string `json:"branch,omitempty"       jsonschema:"Branch name; used when pipeline_id is omitted to find the latest pipeline"`
}

func pipelineGetHandler(ctx context.Context, _ *sdkmcp.CallToolRequest, args *pipelineGetArgs) (*sdkmcp.CallToolResult, any, error) {
	client, err := cmdutil.LoadClientForMCP(ctx)
	if err != nil {
		return toolErr(err)
	}
	var cmdArgs []string
	if args.PipelineID != "" {
		cmdArgs = []string{args.PipelineID}
	}
	var buf bytes.Buffer
	runCtx := iostream.Test(ctx, &buf, io.Discard)
	if err := pipeline.RunGet(runCtx, client, cmdArgs, args.ProjectSlug, args.Branch, true); err != nil {
		return toolErr(err)
	}
	return toolText(buf.String())
}

// --- pipeline_trigger ---

type pipelineTriggerArgs struct {
	ProjectSlug string         `json:"project_slug,omitempty" jsonschema:"Project slug (e.g. gh/org/repo); inferred from the git remote if omitted"`
	Branch      string         `json:"branch,omitempty"       jsonschema:"Branch to trigger; defaults to the current branch"`
	Parameters  map[string]any `json:"parameters,omitempty"   jsonschema:"Pipeline parameters as a key-value map; optional"`
}

func pipelineTriggerHandler(ctx context.Context, _ *sdkmcp.CallToolRequest, args *pipelineTriggerArgs) (*sdkmcp.CallToolResult, any, error) {
	client, err := cmdutil.LoadClientForMCP(ctx)
	if err != nil {
		return toolErr(err)
	}

	projectSlug := args.ProjectSlug
	branch := args.Branch
	if projectSlug == "" || branch == "" {
		info, gitErr := gitremote.Detect()
		if gitErr != nil {
			return toolErr(cmdutil.GitDetectErr(gitErr, "Or specify project_slug and branch explicitly"))
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if branch == "" {
			branch = info.Branch
		}
	}

	resp, err := client.TriggerPipeline(ctx, projectSlug, branch, args.Parameters)
	if err != nil {
		return toolErr(cmdutil.APIErr(err, projectSlug,
			"pipeline.not_found", "No project found for %q.",
			"Check the project slug and try again"))
	}

	out, _ := json.Marshal(map[string]any{
		"id":         resp.ID,
		"number":     resp.Number,
		"state":      resp.State,
		"created_at": resp.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
	return toolText(string(out))
}

// --- pipeline_cancel ---

type pipelineCancelArgs struct {
	PipelineID  string `json:"pipeline_id"            jsonschema:"Pipeline UUID or number to cancel"`
	ProjectSlug string `json:"project_slug,omitempty" jsonschema:"Project slug (e.g. gh/org/repo); required when pipeline_id is a number"`
}

func pipelineCancelHandler(ctx context.Context, _ *sdkmcp.CallToolRequest, args *pipelineCancelArgs) (*sdkmcp.CallToolResult, any, error) {
	if args.PipelineID == "" {
		return toolErr(clierrors.New("args.missing", "Missing pipeline_id",
			"pipeline_id is required").WithExitCode(clierrors.ExitBadArguments))
	}
	client, err := cmdutil.LoadClientForMCP(ctx)
	if err != nil {
		return toolErr(err)
	}
	var buf bytes.Buffer
	runCtx := iostream.Test(ctx, &buf, io.Discard)
	if err := pipeline.RunPipelineCancel(runCtx, client, args.PipelineID, args.ProjectSlug, true); err != nil {
		return toolErr(err)
	}
	return toolText(buf.String())
}

// --- helpers ---

func toolText(text string) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, nil, nil
}

func toolErr(err error) (*sdkmcp.CallToolResult, any, error) {
	msg := err.Error()
	var cliErr *clierrors.CLIError
	if errors.As(err, &cliErr) {
		msg = cliErr.Message
		for _, s := range cliErr.Suggestions {
			msg += "\n  - " + s
		}
	}
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}, nil, nil
}
