package project

import (
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/slug"
	"github.com/spf13/cobra"
)

func newProjectIDCommand(ops *projectOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "id <project-slug>",
		Short: "Print the project ID (UUID) for a given project slug.",
		Long: `Print the project ID (UUID) for a given project slug.

Examples:
  circleci project id gh/CircleCI-Public/circleci-cli
  circleci project id circleci/9YytKzouJxzu4TjCRFqAoD/44n9wujWcTnVZ2b5S8Fnat`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectSlug := args[0]
			parsed, err := slug.ParseProject(projectSlug)
			if err != nil {
				return err
			}

			info, err := ops.projectClient.ProjectInfo(parsed.VCS, parsed.Org, parsed.Repo)
			if err != nil {
				return err
			}

			cmd.Println(info.Id)
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	return cmd
}
