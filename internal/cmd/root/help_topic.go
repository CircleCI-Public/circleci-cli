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

package root

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

// HelpTopicAnnotation marks a command as a `circleci help <topic>` page rather
// than a real command. Topics are hidden from the command listing but are
// user-facing long-form documentation, so they are goldened like any other help
// output and exempted from the per-command help line budget. Exported because
// the golden walk in usage_test.go keys off it — a rename should break the build
// rather than silently stop covering the topics.
const HelpTopicAnnotation = "help:topic"

type helpTopic struct {
	name    string
	short   string
	long    string
	example string
}

var helpTopics = []helpTopic{
	{
		name:  "getting-started",
		short: "Install, authenticate, and start using circleci",
		long: heredoc.Docf(`
			%[1]scircleci%[1]s is a command-line interface to CircleCI for use in your terminal or your scripts.

			## Installation

			Installation instructions are in the [README](https://github.com/CircleCI-Public/circleci-cli#readme)

			## Configuration

			Run %[1]scircleci auth login%[1]s to authenticate with your CircleCI account. You can also set the
			%[1]sCIRCLE_TOKEN%[1]s environment variable.

			## Get Running

			Run %[1]scircleci run get%[1]s to view the runs for the current project in an interactive terminal UI (TUI).

			## Model Context Protocol

			The CLI supports the MCP protocol. To enable it, run:

			Claude:
			%[1]s%[1]s%[1]sshell
			circleci mcp claude enable # Enable in Claude desktop
			claude mcp add-from-claude-desktop -s user # Add with current user scope
			%[1]s%[1]s%[1]s

			Cursor:
			%[1]s%[1]s%[1]sshell
			circleci mcp cursor enable
			%[1]s%[1]s%[1]s

			VS Code:
			%[1]s%[1]s%[1]sshell
			circleci mcp vscode enable
			%[1]s%[1]s%[1]s

			## Support

			Report bugs or search for existing feature requests in our
			[issue tracker](https://github.com/CircleCI-Public/circleci-cli/issues)
		`, "`"),
	},
	{
		name:  "environment",
		short: "Environment variables that can be used with circleci",
		long: heredoc.Docf(`
			%[1]scircleci%[1]s reads the following environment variables to configure authentication, output, and
			behavior. Each takes precedence over the corresponding stored setting, so they are useful for
			scripting and CI where you want explicit, per-invocation control.

			%[1]sCIRCLE_TOKEN%[1]s: an authentication token that will be used for API requests. Setting this avoids
			being prompted to authenticate and takes precedence over previously stored credentials.

			%[1]sCIRCLE_HOST%[1]s: specify the CircleCI hostname.

			%[1]sNO_COLOR%[1]s: set to any value to avoid printing ANSI escape sequences for color output.
			The %[1]s--no-color%[1]s flag has the same effect.

			%[1]sCIRCLE_NO_COLOR%[1]s: set to any value to disable ANSI color output, same as %[1]sNO_COLOR%[1]s.

			%[1]sCIRCLE_NO_INTERACTIVE%[1]s: set to any value to suppress all interactive prompts.

			%[1]sCI%[1]s: when set (as CI systems do), interactive prompts, the animated spinner, and update
			notifications are all disabled automatically.

			%[1]sCIRCLE_SPINNER_DISABLED%[1]s: set to any value to replace the animated spinner with plain text.

			%[1]sCIRCLE_NO_UPDATE_CHECK%[1]s: set to any value to disable checking for newer CLI releases.
			Same effect as %[1]scircleci setting set update-check off%[1]s.

			%[1]sCIRCLE_NO_PAGER%[1]s: set to any value to print long output directly instead of through a pager.

			%[1]sPAGER%[1]s: names the pager program to send long output through (for example %[1]sless%[1]s or %[1]smore%[1]s).
			When unset, a built-in scrollable viewer is used. Set it to %[1]scat%[1]s or an empty value to disable
			paging entirely.

			%[1]sCIRCLE_NO_TELEMETRY%[1]s: set to any value to disable telemetry.

			%[1]sNO_ANALYTICS%[1]s: set to any value to disable telemetry.

			%[1]sDO_NOT_TRACK%[1]s: set to any value to disable telemetry.

		`, "`"),
	},
	{
		name:  "telemetry",
		short: "Information about telemetry in circleci",
		long: heredoc.Doc(`
			circleci collects telemetry to help us understand how the CLI is being used and to improve it.

			To learn more about what data is collected, how it is used, and how to opt out, see:
			<https://circleci.com/docs/guides/toolkit/circleci-cli/#telemetry>
		`),
	},
	{
		name:  "triggers",
		short: "Event source providers and event presets for project triggers",
		long: heredoc.Docf(`
			A trigger connects an event source to a pipeline definition, so that matching events
			automatically start a pipeline run. Create one with %[1]scircleci project trigger create%[1]s.

			## Providers

			The %[1]s--provider%[1]s flag names the event source. %[1]sgithub_app%[1]s is the default.

			| Provider | Event source |
			|---|---|
			| %[1]sgithub_app%[1]s | A repository in a GitHub org with the CircleCI GitHub App installed |
			| %[1]sgithub_server%[1]s | A repository on a self-hosted GitHub Enterprise Server |
			| %[1]sgithub_oauth%[1]s | A repository connected through the legacy GitHub OAuth integration |
			| %[1]swebhook%[1]s | An inbound HTTP webhook, for sources CircleCI does not integrate with directly |
			| %[1]sschedule%[1]s | A time-based schedule rather than a repository event |

			The three repository-backed providers need %[1]s--repo-id%[1]s, the repository's external ID
			as the provider knows it. %[1]swebhook%[1]s and %[1]sschedule%[1]s do not.

			## Event presets

			The %[1]s--event-preset%[1]s flag filters which events actually start a run. Omit it to run on
			every event the provider sends.

			| Preset | Runs on |
			|---|---|
			| %[1]sall-pushes%[1]s | Every push to any branch |
			| %[1]sdefault-branch-pushes%[1]s | Pushes to the default branch only |
			| %[1]sonly-tags%[1]s | Tag pushes only |
			| %[1]sonly-branch-delete%[1]s | Branch deletions |
			| %[1]sonly-build-prs%[1]s | Pushes to branches that have an open pull request |
			| %[1]sonly-open-prs%[1]s | Pull requests being opened |
			| %[1]snon-draft-pr-opened%[1]s | Pull requests opened in a non-draft state |
			| %[1]sonly-build-pushes-to-non-draft-prs%[1]s | Pushes to non-draft pull requests |
			| %[1]sonly-ready-for-review-prs%[1]s | Pull requests marked ready for review |
			| %[1]sonly-labeled-prs%[1]s | Pull requests being labeled |
			| %[1]sonly-merged-prs%[1]s | Pull requests being merged |
			| %[1]sonly-merged-or-closed-prs%[1]s | Pull requests being merged or closed |
			| %[1]spr-comment-equals-run-ci%[1]s | A pull request comment of exactly "run ci" |
			| %[1]spushes-to-merge-queues%[1]s | Pushes to a GitHub merge queue |
		`, "`"),
		example: heredoc.Docf(`
			### Build every push to a GitHub App repository
			%[1]s$ circleci project trigger create --pipeline-definition-id <id> --repo-id 123456789 --event-preset all-pushes%[1]s
			### Build only tagged releases
			%[1]s$ circleci project trigger create --pipeline-definition-id <id> --repo-id 123456789 --event-preset only-tags%[1]s
			### List the triggers already attached to a pipeline definition
			%[1]s$ circleci project trigger list --pipeline-definition-id <id>%[1]s
		`, "`"),
	},
	{
		name:  "reference",
		short: "A comprehensive reference of all circleci commands",
	},
	{
		name:  "formatting",
		short: "Formatting options for JSON data exported from circleci",
		long: heredoc.Docf(`
			By default, the result of %[1]scircleci%[1]s commands are output in markdown text format.
			Some commands support passing the %[1]s--json%[1]s flag, which converts the output to JSON format.
			Once in JSON, the output can be further formatted according to a required formatting string by
			adding either the %[1]s--jq%[1]s or %[1]s--template%[1]s flag. This is useful for selecting a subset of data,
			creating new data structures, displaying the data in a different format, or as input to another
			command line script.

			The %[1]s--json%[1]s flag requires a comma separated list of fields to fetch. To view the possible JSON
			field names for a command omit the string argument to the %[1]s--json%[1]s flag when you run the command.
			Note that you must pass the %[1]s--json%[1]s flag and field names to use the %[1]s--jq%[1]s flag.

			The %[1]s--jq%[1]s flag requires a string argument in jq query syntax, and will only print
			those JSON values which match the query. jq queries can be used to select elements from an
			array, fields from an object, create a new array, and more. The %[1]sjq%[1]s utility does not need
			to be installed on the system to use this formatting directive. When connected to a terminal,
			the output is automatically pretty-printed. To learn about jq query syntax, see:
			<https://jqlang.github.io/jq/manual/>

		`, "`"),
		example: heredoc.Docf(`
			### Default output format
			%[1]scircleci auth me%[1]s
			%[1]s%[1]s%[1]stext
			# User
			- ID: %[1]sc257a143-1fde-4dfe-8cf9-2a85a955f1f7%[1]s
			- Name: Your Name
			- Login: username
			- Avatar URL: https://avatars.githubusercontent.com/u/9812739817239?v=4
			%[1]s%[1]s%[1]s

			### Adding the --json flag with a list of field names
			%[1]scircleci auth me --json%[1]s
			%[1]s%[1]s%[1]sjson
			{
			  "name": "Your Name",
			  "login": "username",
			  "id": "c257a143-1fde-4dfe-8cf9-2a85a955f1f7",
			  "avatar_url": "https://avatars.githubusercontent.com/u/9812739817239?v=4"
			}
			%[1]s%[1]s%[1]s

			### Adding the --jq flag and selecting a field
			%[1]scircleci auth me --json --jq '.login'%[1]s
			%[1]s%[1]s%[1]stext
			username
			%[1]s%[1]s%[1]s
		`, "`"),
	},
}

func newCmdHelpTopic(ht helpTopic, initConfig func(cmd *cobra.Command) (func(), error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         ht.name,
		Short:       ht.short,
		Long:        ht.long,
		Example:     ht.example,
		Hidden:      true,
		Annotations: map[string]string{HelpTopicAnnotation: "true"},
	}

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		cleanup, err := initConfig(c)
		if err != nil {
			return err
		}
		cleanup()

		ctx := c.Context()
		return helpTopicUsageFunc(ctx, c)
	})

	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		cleanup, err := initConfig(c)
		if err == nil {
			cleanup()
		}
		ctx := c.Context()
		helpTopicHelpFunc(ctx, c)
	})

	return cmd
}

func helpTopicHelpFunc(ctx context.Context, command *cobra.Command) {
	var md bytes.Buffer
	_, _ = fmt.Fprintf(&md, "# %s\n", topicTitle(command.Name()))
	md.WriteString(command.Long)
	if command.Example != "" {
		_, _ = fmt.Fprintf(&md, "\n\n## Examples\n")
		_, _ = fmt.Fprint(&md, command.Example)
	}

	iostream.PrintMarkdown(ctx, md.String())
}

func helpTopicUsageFunc(ctx context.Context, command *cobra.Command) error {
	iostream.ErrPrintf(ctx, "Usage: circleci help %s", command.Use)
	return nil
}

// topicTitle turns a topic's command name into a display heading: hyphens
// become spaces and only the first letter is capitalized, so "getting-started"
// reads as "Getting started" (single-word topics are unaffected).
func topicTitle(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
