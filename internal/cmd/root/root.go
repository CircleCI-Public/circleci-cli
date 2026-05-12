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
	"github.com/MakeNowJust/heredoc"
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	cmdapi "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/api"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/artifacts"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/completion"
	cmdcontext "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/context"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/deploy"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/envvar"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/job"
	cmdlogs "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/logs"
	cmdnamespace "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/namespace"
	cmdopen "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/open"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/project"
	cmdrepo "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/repo"
	cmdrun "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/run"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/settings"
	cmdversion "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/version"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/workflow"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// NewRootCmd builds the root cobra command and wires all subcommands.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "circleci",
		Short: "The CircleCI CLI",
		Long: heredoc.Doc(`
			Work with CircleCI from the command line.

			Run 'circleci help <command>' for usage of a specific command.

			Docs:    https://circleci.com/docs/local-cli/
			Support: https://github.com/CircleCI-Public/circleci-cli/issues
		`),
		SilenceErrors: true, // main.go handles error printing
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			configPath, _ := cmd.Flags().GetString("config")
			ctx = cmdutil.WithConfigPath(ctx, configPath)
			apiclient.DeviceID = config.EnsureDeviceID(ctx, configPath)

			jqFilter, _ := cmd.Flags().GetString("jq")
			ctx = iostream.WithJQFilter(ctx, jqFilter)

			cmd.SetContext(ctx)
			return nil
		},
	}

	cmd.Version = version
	cmd.SetVersionTemplate("circleci version {{.Version}}\n")

	cmd.PersistentFlags().StringP("config", "c", "", "path to config file (default: ~/.config/circleci/config.yml)")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output; data on stdout is unaffected")
	cmd.PersistentFlags().BoolP("debug", "", false, "enable debug logging")
	cmd.PersistentFlags().StringP("theme", "", "auto", "set the color theme (default: auto)")
	_ = cmd.PersistentFlags().MarkHidden("theme")

	cmd.PersistentFlags().BoolP("insecure-storage", "", false, "do not use the system's secure storage for storing tokens")
	_ = cmd.PersistentFlags().MarkHidden("insecure-storage")

	cmd.AddCommand(artifacts.NewArtifactsCmd())
	cmd.AddCommand(cmdapi.NewAPICmd())
	cmd.AddCommand(cmdauth.NewAuthCmd())
	cmd.AddCommand(cmdcontext.NewContextCmd())
	cmd.AddCommand(deploy.NewDeployCmd())
	cmd.AddCommand(cmdlogs.NewLogsCmd())
	cmd.AddCommand(cmdnamespace.NewNamespaceCmd())
	cmd.AddCommand(cmdopen.NewOpenCmd())
	cmd.AddCommand(cmdversion.NewVersionCmd(version))
	cmd.AddCommand(completion.NewCompletionCmd())
	cmd.AddCommand(envvar.NewEnvVarCmd())
	cmd.AddCommand(project.NewGetCmd("info")) // Alias to project get
	cmd.AddCommand(job.NewJobCmd())
	cmd.AddCommand(cmdrun.NewRunCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(cmdrepo.NewRepoCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(settings.NewSettingsCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	// Wire in MCP commands
	cmd.AddCommand(ophis.Command(nil))

	return cmd
}
