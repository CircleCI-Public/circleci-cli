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
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	cmdapi "github.com/CircleCI-Public/circleci-cli/internal/cmd/api"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/artifacts"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/certificate"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/completion"
	cmdconfig "github.com/CircleCI-Public/circleci-cli/internal/cmd/config"
	cmdcontext "github.com/CircleCI-Public/circleci-cli/internal/cmd/context"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/deploy"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/envvar"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/job"
	cmdlogs "github.com/CircleCI-Public/circleci-cli/internal/cmd/logs"
	cmdnamespace "github.com/CircleCI-Public/circleci-cli/internal/cmd/namespace"
	cmdorb "github.com/CircleCI-Public/circleci-cli/internal/cmd/orb"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/pipeline"
	cmdpolicy "github.com/CircleCI-Public/circleci-cli/internal/cmd/policy"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/project"
	cmdrun "github.com/CircleCI-Public/circleci-cli/internal/cmd/run"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/settings"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/signingconfig"
	cmdversion "github.com/CircleCI-Public/circleci-cli/internal/cmd/version"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/workflow"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
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
	cmd.AddCommand(certificate.NewCertificateCmd())
	cmd.AddCommand(cmdconfig.NewConfigCmd())
	cmd.AddCommand(cmdcontext.NewContextCmd())
	cmd.AddCommand(deploy.NewDeployCmd())
	cmd.AddCommand(cmdlogs.NewLogsCmd())
	cmd.AddCommand(cmdnamespace.NewNamespaceCmd())
	cmd.AddCommand(cmdorb.NewOrbCmd())
	cmd.AddCommand(cmdversion.NewVersionCmd(version))
	cmd.AddCommand(completion.NewCompletionCmd())
	cmd.AddCommand(envvar.NewEnvVarCmd())
	cmd.AddCommand(project.NewGetCmd("info")) // Alias to project get
	cmd.AddCommand(job.NewJobCmd())
	cmd.AddCommand(pipeline.NewPipelineCmd())
	cmd.AddCommand(cmdpolicy.NewPolicyCmd())
	cmd.AddCommand(cmdrun.NewRunCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(settings.NewSettingsCmd())
	cmd.AddCommand(signingconfig.NewSigningConfigCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	// Wire in MCP commands
	cmd.AddCommand(ophis.Command(nil))

	// Register extensions found in PATH. Built-in commands always win on name
	// conflicts — extensions cannot shadow them.
	builtins := map[string]bool{}
	for _, sub := range cmd.Commands() {
		builtins[sub.Name()] = true
	}

	path := os.Getenv("PATH")
	if exts := extension.FindAll(path); len(exts) > 0 {
		cmd.AddGroup(&cobra.Group{ID: "extension", Title: "Extensions"})
		for _, name := range exts {
			if !builtins[name] {
				cmd.AddCommand(extension.NewCmd(name))
			}
		}
	}

	return cmd
}
