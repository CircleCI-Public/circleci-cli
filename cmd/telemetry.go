package cmd

import (
	"os"

	"github.com/CircleCI-Public/circleci-cli/cmd/create_telemetry"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newTelemetryCommand(config *settings.Config) *cobra.Command {
	apiClient := create_telemetry.CreateAPIClient(config)

	telemetryEnable := &cobra.Command{
		Use:   "enable",
		Short: "Allow telemetry events to be sent to CircleCI servers",
		RunE: func(_ *cobra.Command, _ []string) error {
			return setIsTelemetryActive(apiClient, true)
		},
		Args: cobra.ExactArgs(0),
	}

	telemetryDisable := &cobra.Command{
		Use:   "disable",
		Short: "Make sure no telemetry events is sent to CircleCI servers",
		RunE: func(_ *cobra.Command, _ []string) error {
			return setIsTelemetryActive(apiClient, false)
		},
		Args: cobra.ExactArgs(0),
	}

	telemetryCommand := &cobra.Command{
		Use:   "telemetry",
		Short: "Configure telemetry preferences",
		Long: `Configure telemetry preferences.

Note: If you have not configured your telemetry preferences and call the CLI with a closed stdin, telemetry will be disabled`,
	}

	telemetryCommand.AddCommand(telemetryEnable)
	telemetryCommand.AddCommand(telemetryDisable)

	return telemetryCommand
}

func setIsTelemetryActive(apiClient create_telemetry.TelemetryAPIClient, isActive bool) error {
	settings := settings.TelemetrySettings{}
	if err := settings.Load(); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "Loading telemetry configuration")
	}

	settings.HasAnsweredPrompt = true
	settings.IsEnabled = isActive

	if settings.UniqueID == "" {
		settings.UniqueID = create_telemetry.CreateUUID()
	}

	if settings.UserID == "" {
		if myID, err := apiClient.GetMyUserId(); err == nil {
			settings.UserID = myID
		}
	}

	if err := settings.Write(); err != nil {
		return errors.Wrap(err, "Writing telemetry configuration")
	}

	return nil
}
