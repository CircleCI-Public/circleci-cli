package cmd

import (
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

var (
	createUUID  = func() string { return uuid.New().String() }
	isStdinOpen = term.IsTerminal(int(os.Stdin.Fd()))
)

type telemetryUI interface {
	AskUserToApproveTelemetry(message string) bool
}

type telemetryInteractiveUI struct{}

func (telemetryInteractiveUI) AskUserToApproveTelemetry(message string) bool {
	return prompt.AskUserToConfirmWithDefault(message, true)
}

type telemetryTestUI struct {
	Approved bool
}

func (ui telemetryTestUI) AskUserToApproveTelemetry(message string) bool {
	return ui.Approved
}

// Make sure the user gave their approval for the telemetry and
func checkTelemetry(config *settings.Config, ui telemetryUI) error {
	config.Telemetry.Load()

	if err := askForTelemetryApproval(config, ui); err != nil {
		config.Telemetry.Client = telemetry.CreateClient(telemetry.User{}, false)
		return err
	}

	config.Telemetry.Client = telemetry.CreateClient(telemetry.User{
		UniqueID: config.Telemetry.UniqueID,
		UserID:   config.Telemetry.UserID,
	}, config.Telemetry.IsActive)
	return nil
}

func askForTelemetryApproval(config *settings.Config, ui telemetryUI) error {
	// If we already have telemetry information or that telemetry is explicitly disabled, skip
	if config.Telemetry.HasAnsweredPrompt || config.Telemetry.DisabledFromParams {
		return nil
	}

	// If stdin is not available, send telemetry event, disactive telemetry and return
	if !isStdinOpen {
		config.Telemetry.IsActive = false
		return telemetry.SendTelemetryApproval(telemetry.User{
			UniqueID: config.Telemetry.UniqueID,
		}, telemetry.NoStdin)
	}

	// Else ask user for telemetry approval
	fmt.Println("CircleCI would like to collect CLI usage data for product improvement purposes.")
	fmt.Println("")
	fmt.Println("Participation is voluntary, and your choice can be changed at any time through the command `cli telemetry enable` and `cli telemetry disable`.")
	fmt.Println("For more information, please see our privacy policy at https://circleci.com/legal/privacy/.")
	fmt.Println("")
	config.Telemetry.IsActive = ui.AskUserToApproveTelemetry("Enable telemetry?")
	config.Telemetry.HasAnsweredPrompt = true

	// If user allows telemetry, create a telemetry user
	user := telemetry.User{
		UniqueID: config.Telemetry.UniqueID,
	}
	if config.Telemetry.UniqueID == "" {
		user.UniqueID = createUUID()
	}

	if config.Telemetry.IsActive && config.Token != "" {
		me, err := api.GetMe(rest.NewFromConfig(config.Host, config))
		if err == nil {
			user.UserID = me.ID
		}
	}
	config.Telemetry.UniqueID = user.UniqueID
	config.Telemetry.UserID = user.UserID

	// Send telemetry approval event
	approval := telemetry.Enabled
	if !config.Telemetry.IsActive {
		approval = telemetry.Disabled
	}
	if err := telemetry.SendTelemetryApproval(user, approval); err != nil {
		return err
	}

	// Write telemetry
	if err := config.Telemetry.Write(); err != nil {
		return errors.Wrap(err, "Writing telemetry to disk")
	}

	return nil
}
