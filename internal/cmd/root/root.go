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
	"github.com/spf13/cobra"

	cmdapi "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/api"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/artifacts"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/completion"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/envvar"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/job"
	cmdlogs "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/logs"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/pipeline"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/project"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/settings"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/workflow"
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
	}

	cmd.Version = version
	cmd.SetVersionTemplate("circleci version {{.Version}}\n")

	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output; data on stdout is unaffected")

	cmd.PersistentFlags().BoolP("insecure-storage", "", false, "do not use the system's secure storage for storing tokens")
	_ = cmd.PersistentFlags().MarkHidden("insecure-storage")

	cmd.AddCommand(cmdapi.NewAPICmd())
	cmd.AddCommand(artifacts.NewArtifactsCmd())
	cmd.AddCommand(completion.NewCompletionCmd())
	cmd.AddCommand(envvar.NewEnvVarCmd())
	cmd.AddCommand(job.NewJobCmd())
	cmd.AddCommand(cmdlogs.NewLogsCmd())
	cmd.AddCommand(pipeline.NewPipelineCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(settings.NewSettingsCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	return cmd
}
