package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/local"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newLocalExecuteCommand(config *settings.Config) *cobra.Command {
	var args []string
	buildCommand := &cobra.Command{
		Use:   "execute <job-name>",
		Short: "Run a job in a container on the local machine",
		PreRunE: func(cmd *cobra.Command, _args []string) error {
			args = _args
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return local.Execute(cmd.Flags(), config, args)
		},
		Args: cobra.MinimumNArgs(1),
	}

	local.AddFlagsForDocumentation(buildCommand.Flags())
	buildAgentVersionUsage := `The version of the build agent image you want to use. This can be configured by writing in $HOME/.circleci/build_agent_settings.json: '{"LatestSha256":"<version-of-build-agent>"}'`
	buildCommand.Flags().String("build-agent-version", "", buildAgentVersionUsage)
	buildCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")
	buildCommand.Flags().String("org-id", "", "organization id, used when a config depends on private orbs belonging to that org")

	return buildCommand
}

// hidden command for backwards compatibility
func newBuildCommand(config *settings.Config) *cobra.Command {
	cmd := newLocalExecuteCommand(config)
	cmd.Hidden = true
	cmd.Use = "build"
	return cmd
}

func newLocalCommand(config *settings.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Debug jobs on the local machine",
	}
	cmd.AddCommand(newLocalExecuteCommand(config))
	return cmd
}
