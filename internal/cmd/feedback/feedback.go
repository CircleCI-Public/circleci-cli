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

package feedback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/agent"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Length limits enforced by POST /api/v3/tool/send-feedback, in characters.
const (
	maxMessageLen   = 2000
	maxToolNameLen  = 200
	maxAgentTypeLen = 100
)

type feedbackOutput struct {
	ID string `json:"id"`
}

func NewFeedbackCmd() *cobra.Command {
	var (
		tool    string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "feedback <message>",
		Short:   "Send feedback about the CLI to CircleCI",
		GroupID: "user",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<message>%[1]s is the feedback itself, up to %[2]d characters.
			`, "`", maxMessageLen),
		},
		Long: heredoc.Doc(`
			Report an unexpected error, a missing capability or confusing output to
			CircleCI. Say what you tried, what you ran and what happened; no secrets.
			Reports include your CircleCI user, CLI version, AI agent and MCP use.

			JSON fields: id
		`),
		Example: heredoc.Doc(`
			# Report a missing capability
			$ circleci feedback "I wanted to compare job timings between two runs but found no command for it"

			# Report confusing output from a command
			$ circleci feedback "run get shows no jobs for a run that has three" --tool "run get"

			# Capture just the report ID
			$ circleci feedback "context list printed nothing for an org with contexts" --json --jq '.id'
		`),
		// Counted in RunE rather than by cobra.MaximumNArgs, whose error exits 1 as
		// plain text: an unquoted message is the likeliest mistake here.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "message"); err != nil {
				return err
			}
			if len(args) > 1 {
				return clierrors.New("feedback.too_many_args", "Too many arguments",
					fmt.Sprintf("The feedback message must be a single argument (got: %d).", len(args))).
					WithSuggestions(`Quote the whole message: circleci feedback "<message>"`).
					WithExitCode(clierrors.ExitBadArguments)
			}
			if err := validate(args[0], tool); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runFeedback(ctx, client, newReport(ctx, args[0], tool), jsonOut)
		},
	}

	cmd.Flags().StringVar(&tool, "tool", "", `CLI command or MCP tool the feedback is about (e.g. "run get")`)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runFeedback(ctx context.Context, client *apiclient.Client, report apiclient.FeedbackReport, jsonOut bool) error {
	id, err := client.SendFeedback(ctx, report)
	if err != nil {
		return feedbackErr(err)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, feedbackOutput{ID: id.String()})
	}

	iostream.Printf(ctx, "%s Sent feedback (%s). Thank you!\n", iostream.SymbolOK(ctx), id)
	return nil
}

// validate mirrors the endpoint's limits, so a report the API would reject is never sent.
func validate(message, tool string) error {
	if strings.TrimSpace(message) == "" {
		return clierrors.New("feedback.message_blank", "Empty feedback message",
			"The feedback message is empty.").
			WithSuggestions("Say what you were trying to do, what you ran and what happened").
			WithExitCode(clierrors.ExitBadArguments)
	}
	if n := utf8.RuneCountInString(message); n > maxMessageLen {
		return clierrors.New("feedback.message_too_long", "Feedback message too long",
			fmt.Sprintf("The feedback message must be at most %d characters (got: %d).", maxMessageLen, n)).
			WithSuggestions("Shorten the message to what you ran and what went wrong").
			WithExitCode(clierrors.ExitBadArguments)
	}
	if n := utf8.RuneCountInString(tool); n > maxToolNameLen {
		return clierrors.New("feedback.tool_too_long", "Tool name too long",
			fmt.Sprintf("--tool must be at most %d characters (got: %d).", maxToolNameLen, n)).
			WithSuggestions(`Pass only the command or tool name, e.g. --tool "run get"`).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return nil
}

// newReport derives source, agent_type and client_version, which are not flags.
func newReport(ctx context.Context, message, tool string) apiclient.FeedbackReport {
	source := apiclient.FeedbackSourceCLI
	if agent.IsMCP() {
		source = apiclient.FeedbackSourceCLIMCP
	}

	// Under the MCP server the name is "mcp/<agent>", which source already says.
	// An over-long name is dropped rather than get the whole report rejected.
	agentType := strings.TrimPrefix(cmdutil.GetAgentName(ctx), "mcp/")
	if agentType == "mcp" || utf8.RuneCountInString(agentType) > maxAgentTypeLen {
		agentType = ""
	}

	return apiclient.FeedbackReport{
		Message:       message,
		ToolName:      tool,
		Source:        source,
		AgentType:     agentType,
		ClientVersion: cmdutil.GetVersion(ctx),
	}
}

// feedbackErr handles a 5xx itself rather than via cmdutil.APIErr, so a failed
// report does not suggest sending another one.
func feedbackErr(err error) *clierrors.CLIError {
	if httpcl.HasStatusCode(err, http.StatusBadRequest) {
		msg := "CircleCI rejected the feedback report."
		if apiErr, ok := apiclient.ParseError(err); ok {
			msg += "\n" + apiErr.Message()
		}
		// validate has already applied the API's limits, so a rejection means this
		// CLI and the API disagree about the request, not that the input is wrong.
		return clierrors.New("feedback.rejected", "Feedback rejected", msg).
			WithSuggestions("Upgrade to the latest version: https://circleci.com/docs/local-cli/").
			WithExitCode(clierrors.ExitAPIError)
	}
	if he, ok := errors.AsType[*httpcl.HTTPError](err); ok && he.StatusCode >= http.StatusInternalServerError {
		msg := fmt.Sprintf("API returned %d", he.StatusCode)
		if apiErr, ok := apiclient.ParseError(err); ok {
			msg += ": " + apiErr.Message()
		}
		return clierrors.New("feedback.api_error", "CircleCI API error", msg).
			WithSuggestions("Try again later").
			WithExitCode(clierrors.ExitAPIError)
	}
	return cmdutil.APIErr(err, "feedback reports", "feedback.unavailable", "This CircleCI host doesn't accept %s.",
		"Report the problem at https://github.com/CircleCI-Public/circleci-cli/issues")
}
