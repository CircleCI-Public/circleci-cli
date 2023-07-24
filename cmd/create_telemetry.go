package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

var (
	CreateUUID  = func() string { return uuid.New().String() }
	isStdinATTY = term.IsTerminal(int(os.Stdin.Fd()))
)

type telemetryUI interface {
	AskUserToApproveTelemetry(message string) bool
}

type telemetryInteractiveUI struct{}

func (telemetryInteractiveUI) AskUserToApproveTelemetry(message string) bool {
	return prompt.AskUserToConfirmWithDefault(message, true)
}

type TelemetryAPIClient interface {
	GetMyUserId() (string, error)
}

func CreateAPIClient(config *settings.Config) TelemetryAPIClient {
	return telemetryCircleCIAPI{
		cli: rest.NewFromConfig(config.Host, config),
	}
}

type telemetryCircleCIAPI struct {
	cli *rest.Client
}

func (client telemetryCircleCIAPI) GetMyUserId() (string, error) {
	me, err := api.GetMe(client.cli)
	if err != nil {
		return "", err
	}
	return me.ID, nil
}

type nullTelemetryAPIClient struct{}

func (client nullTelemetryAPIClient) GetMyUserId() (string, error) {
	panic("Should not be called")
}

// Make sure the user gave their approval for the telemetry and
func CreateTelemetry(config *settings.Config) telemetry.Client {
	mockTelemetry := config.MockTelemetry
	if mockTelemetry == "" {
		mockTelemetry = os.Getenv("MOCK_TELEMETRY")
	}
	if mockTelemetry != "" {
		return telemetry.CreateFileTelemetry(mockTelemetry)
	}

	if config.IsTelemetryDisabled || len(os.Getenv("CIRCLECI_CLI_TELEMETRY_OPTOUT")) != 0 {
		return telemetry.CreateClient(telemetry.User{}, false)
	}

	var apiClient TelemetryAPIClient = nullTelemetryAPIClient{}
	if config.HTTPClient != nil {
		apiClient = telemetryCircleCIAPI{
			cli: rest.NewFromConfig(config.Host, config),
		}
	}
	ui := telemetryInteractiveUI{}

	telemetrySettings := settings.TelemetrySettings{}
	user := telemetry.User{
		IsSelfHosted: config.Host != defaultHost,
		OS:           runtime.GOOS,
		Version:      version.Version,
		TeamName:     "devex",
	}

	loadTelemetrySettings(&telemetrySettings, &user, apiClient, ui)
	client := telemetry.CreateClient(user, telemetrySettings.IsEnabled)

	return client
}

func loadTelemetrySettings(settings *settings.TelemetrySettings, user *telemetry.User, apiClient TelemetryAPIClient, ui telemetryUI) {
	err := settings.Load()
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error loading telemetry configuration: %s\n", err)
	}

	user.UniqueID = settings.UniqueID
	user.UserID = settings.UserID

	// If we already have telemetry information or that telemetry is explicitly disabled, skip
	if settings.HasAnsweredPrompt {
		// If we have no user id, we try requesting the user id again
		if settings.UserID == "" && settings.IsEnabled {
			myID, err := apiClient.GetMyUserId()
			if err == nil {
				settings.UserID = myID
				user.UserID = myID
				if err := settings.Write(); err != nil {
					fmt.Printf("Error writing telemetry settings to disk: %s\n", err)
				}
			}
		}

		return
	}

	// If stdin is not available, send telemetry event, disable telemetry and return
	if !isStdinATTY {
		settings.IsEnabled = false
		err := telemetry.SendTelemetryApproval(telemetry.User{
			UniqueID: settings.UniqueID,
		}, telemetry.NoStdin)
		if err != nil {
			fmt.Printf("Error while sending telemetry approval %s\n", err)
		}
		return
	}

	// Else ask user for telemetry approval
	fmt.Println("CircleCI would like to collect CLI usage data for product improvement purposes.")
	fmt.Println("")
	fmt.Println("Participation is voluntary, and your choice can be changed at any time through the command `cli telemetry enable` and `cli telemetry disable`.")
	fmt.Println("For more information, please see our privacy policy at https://circleci.com/legal/privacy/.")
	fmt.Println("")
	settings.IsEnabled = ui.AskUserToApproveTelemetry("Enable telemetry?")
	settings.HasAnsweredPrompt = true

	// Make sure we have user info and set them
	if settings.IsEnabled {
		if settings.UniqueID == "" {
			settings.UniqueID = CreateUUID()
		}
		user.UniqueID = settings.UniqueID

		if settings.UserID == "" {
			myID, err := apiClient.GetMyUserId()
			if err == nil {
				settings.UserID = myID
			}
		}
		user.UserID = settings.UserID
	} else {
		*user = telemetry.User{}
	}

	// Send telemetry approval event
	approval := telemetry.Enabled
	if !settings.IsEnabled {
		approval = telemetry.Disabled
	}

	if err := telemetry.SendTelemetryApproval(*user, approval); err != nil {
		fmt.Printf("Unable to send approval telemetry event: %s\n", err)
	}

	// Write telemetry
	if err := settings.Write(); err != nil {
		fmt.Printf("Error writing telemetry settings to disk: %s\n", err)
	}
}

// Utility function used when creating telemetry events.
// It takes a cobra Command and creates a telemetry.CommandInfo of it
// If getParent is true, puts both the command's args in `LocalArgs` and the parent's args
// Else only put the command's args
// Note: child flags overwrite parent flags with same name
func GetCommandInformation(cmd *cobra.Command, getParent bool) telemetry.CommandInfo {
	localArgs := map[string]string{}

	parent := cmd.Parent()
	if getParent && parent != nil {
		parent.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			localArgs[flag.Name] = flag.Value.String()
		})
	}

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		localArgs[flag.Name] = flag.Value.String()
	})

	return telemetry.CommandInfo{
		Name:      cmd.Name(),
		LocalArgs: localArgs,
	}
}
