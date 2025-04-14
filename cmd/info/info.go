package info

import (
	"encoding/json"

	"github.com/CircleCI-Public/circleci-cli/api/info"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
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

	jsonFormat := false

	opts := infoOptions{
		cfg:       config,
		validator: preRunE,
	}
	infoCommand := &cobra.Command{
		Use:   "info",
		Short: "Check information associated to your user account.",
	}
	orgInfoCmd := orgInfoCommand(client, opts)
	orgInfoCmd.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")
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
			err := getOrgInformation(cmd, client)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(telemetry.CreateInfoEvent(telemetry.GetCommandInformation(cmd, true), err))
			}

			return err
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

	jsonVal, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	if jsonVal {
		// return JSON formatted for output
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		jsonWriter := cmd.OutOrStdout()
		if _, err := jsonWriter.Write(jsonResp); err != nil {
			return err
		}
	} else {
		table := tablewriter.NewWriter(cmd.OutOrStdout())

		table.SetHeader([]string{"ID", "Name"})

		for _, info := range *resp {
			table.Append([]string{
				info.ID, info.Name,
			})
		}
		table.Render()
	}

	return nil
}
