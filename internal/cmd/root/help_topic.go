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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

type helpTopic struct {
	name    string
	short   string
	long    string
	example string
}

var helpTopics = []helpTopic{
	{
		name:  "environment",
		short: "Environment variables that can be used with circleci",
		long: heredoc.Docf(`
			%[1]sCIRCLE_TOKEN%[1]s: an authentication token that will be used for API requests. Setting this avoids
			being prompted to authenticate and takes precedence over previously stored credentials.

			%[1]sCIRCLE_HOST%[1]s: specify the CircleCI hostname.

			%[1]sNO_COLOR%[1]s: set to any value to avoid printing ANSI escape sequences for color output.

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
			<https://circleci.com/docs/local-cli>
		`),
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
			## Default output format
			$ circleci auth me
			%[1]s%[1]s%[1]stext
			# User
			- ID: %[1]sc257a143-1fde-4dfe-8cf9-2a85a955f1f7%[1]s
			- Name: Your Name
			- Login: username
			- Avatar URL: https://avatars.githubusercontent.com/u/9812739817239?v=4
			%[1]s%[1]s%[1]s

			## Adding the --json flag with a list of field names
			$ circleci auth me --json
			%[1]s%[1]s%[1]sjson
			{
			  "name": "Your Name",
			  "login": "username",
			  "id": "c257a143-1fde-4dfe-8cf9-2a85a955f1f7",
			  "avatar_url": "https://avatars.githubusercontent.com/u/9812739817239?v=4"
			}
			%[1]s%[1]s%[1]s

			## Adding the --jq flag and selecting a field
			$ circleci auth me --json --jq '.login'
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
	_, _ = fmt.Fprintf(&md, "# %s\n", titleCase(command.Name()))
	md.WriteString(command.Long)
	if command.Example != "" {
		_, _ = fmt.Fprintf(&md, "\n\nExamples\n")
		_, _ = fmt.Fprint(&md, iostream.Indent(command.Example, "  "))
	}

	iostream.PrintMarkdown(ctx, md.String())
}

func helpTopicUsageFunc(ctx context.Context, command *cobra.Command) error {
	iostream.ErrPrintf(ctx, "Usage: circleci help %s", command.Use)
	return nil
}
