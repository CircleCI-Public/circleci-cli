package info

import (
	"github.com/CircleCI-Public/circleci-cli/api/info"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/olekukonko/tablewriter"

	"github.com/spf13/cobra"
)

// infoOptions info command options
type infoOptions struct {
	cfg       *settings.Config
	validator validator.Validator
}

// NewInfoCommand information cobra command creation
func NewInfoCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	client, _ := info.NewInfoClient(*config)

	opts := infoOptions{
		cfg:       config,
		validator: preRunE,
	}
	infoCommand := &cobra.Command{
		Use:   "info",
		Short: "Check information associated to your user account.",
	}
	orgInfoCmd := orgInfoCommand(client, opts)
	infoCommand.AddCommand(orgInfoCmd)

	return infoCommand
}

// orgInfoCommand organization information subcommand cobra command creation
func orgInfoCommand(client info.InfoClient, opts infoOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "org",
		Short:   "View your Organizations' information",
		Long:    `View your Organizations' names and ids.`,
		PreRunE: opts.validator,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getOrgInformation(cmd, client)
		},
		Annotations: make(map[string]string),
		Example:     `circleci info org`,
	}
}

// getOrgInformation gets all of the users organizations' information
func getOrgInformation(cmd *cobra.Command, client info.InfoClient) error {
	resp, err := client.GetInfo()
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(cmd.OutOrStdout())

	table.SetHeader([]string{"ID", "Name"})

	for _, info := range *resp {
		table.Append([]string{
			info.ID, info.Name,
		})
	}
	table.Render()
	return nil
}
