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

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
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

			Pick the entry for your platform. Package managers that need their repository
			registered first list the setup command and the install command, in that order.

			macOS or Linux — Homebrew:
			%[1]s%[1]s%[1]sshell
			brew install circleci
			%[1]s%[1]s%[1]s

			Windows — WinGet:
			%[1]s%[1]s%[1]sshell
			winget install --id CircleCI.CLI
			%[1]s%[1]s%[1]s

			Linux — apt (Debian, Ubuntu, Mint, Raspberry Pi):
			%[1]s%[1]s%[1]sshell
			curl -1sLf 'https://packages.circleci.com/public/setup.deb.sh' | sudo -E bash
			sudo apt install circleci
			%[1]s%[1]s%[1]s

			Linux — rpm (Fedora, RHEL, SUSE, Amazon):
			%[1]s%[1]s%[1]sshell
			curl -1sLf 'https://packages.circleci.com/public/setup.rpm.sh' | sudo -E bash
			sudo dnf install circleci
			%[1]s%[1]s%[1]s

			Linux — Snap:
			%[1]s%[1]s%[1]sshell
			sudo snap install circleci
			sudo snap connect circleci:password-manager-service
			%[1]s%[1]s%[1]s

			Prebuilt binaries and Linux packages for every release are on the
			[releases page](https://github.com/CircleCI-Public/circleci-cli/releases), and the
			[README](https://github.com/CircleCI-Public/circleci-cli#readme) covers the
			less common cases.

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

			%[1]sCIRCLE_NO_UPDATE_CHECK%[1]s: set to any value to disable checking for newer CLI and
			extension releases. Same effect as %[1]scircleci setting set update-check off%[1]s.

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
			| %[1]sorigin%[1]s | A repository in Cursor, reached through the CircleCI Origin app |
			| %[1]swebhook%[1]s | An inbound HTTP webhook, for sources CircleCI does not integrate with directly |
			| %[1]sschedule%[1]s | A time-based schedule rather than a repository event |

			The four repository-backed providers need %[1]s--repo-id%[1]s, the repository's external ID
			as the provider knows it. %[1]swebhook%[1]s and %[1]sschedule%[1]s do not.

			%[1]sorigin%[1]s additionally needs %[1]s--repo-full-name%[1]s, the repository's %[1]sowner/repo%[1]s name.
			Origin addresses repositories by owner and name and offers no lookup by id, so the name
			is what resolves the repository; a name and an ID that disagree are rejected. The GitHub
			providers key repositories by ID alone and ignore the flag.

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
		name:  "functions",
		short: "Declaring and invoking CircleCI functions in your config",
		long: heredoc.Docf(`
			A function is a versioned binary invoked as a step. Declare it once in the
			top-level %[1]sfunctions%[1]s block, then name it directly wherever a step goes.

			## Declaring

			Each entry maps an alias to a pinned reference, %[1]s<path>@<version>%[1]s:

			%[2]syaml
			functions:
			  setup-go: github.com/circleci-functions/setup-go@v0.5.1-684fd5b
			%[2]s

			The alias is the name a step invokes, and yours to choose. It must not clash
			with an orb, a command, or a built-in step in the same config. The path is
			%[1]shost.tld/org/name%[1]s and the version a semver tag led by %[1]sv%[1]s. Both halves are
			required: an unpinned function is rejected, so a config always records exactly
			which build ran.

			## Invoking

			Name the alias where a step goes. Arguments go under %[1]swith%[1]s, and %[1]sid%[1]s labels the
			step so later steps can refer to its output:

			%[2]syaml
			jobs:
			  build:
			    steps:
			      - setup-go:
			          id: go
			          with: {version: "1.24"}
			%[2]s

			Arguments are passed to a binary, so each value must be a single value rather
			than a list or a map. A step with no arguments can be written bare, the way
			%[1]scheckout%[1]s is. Naming one of the function's commands after a %[1]s/%[1]s —
			%[1]ssetup-go/cache%[1]s — runs that command instead of the function's root; one level
			only.

			## Where functions come from

			Functions are published independently of your config and discovered through
			CircleCI, not fetched from your repository. %[1]scircleci function list%[1]s shows what
			is published. Only functions published under a %[1]shost.tld/org/name%[1]s identifier
			are listed.
		`, "`", "```"),
		example: heredoc.Docf(`
			### See what is published
			%[1]s$ circleci function list%[1]s
			### Read the versions as JSON
			%[1]s$ circleci function list --json --jq '.[] | {name, latest_version}'%[1]s
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
		Use:     ht.name,
		Short:   ht.short,
		Long:    ht.long,
		Example: ht.example,
		Hidden:  true,
		// A topic is runnable so that an argument is an error rather than
		// ignored. Cobra returns flag.ErrHelp for a command with no RunE before
		// it validates arguments, so a topic one plural away from a real command
		// group ("functions" vs "function") would answer `circleci functions
		// list` with the topic on stdout and exit 0.
		RunE:        helpTopicRunE,
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

// helpTopicRunE prints the topic when invoked bare, and rejects any argument:
// a topic has no subcommands, so an argument means the user was reaching for a
// command of a similar name.
func helpTopicRunE(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return clierrors.New("help.topic_not_a_command", "Not a command",
		fmt.Sprintf("%q is a help topic, not a command, and takes no arguments.", cmd.Name())).
		WithSuggestions(
			fmt.Sprintf("Read the topic: circleci help %s", cmd.Name()),
			"List the available commands: circleci --help",
		).
		WithExitCode(clierrors.ExitBadArguments)
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
