package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var testing = false

func newConfigureCommand() *cobra.Command {

	configureCommand := &cobra.Command{
		Use:   "configure",
		Short: "Configure the tool with your credentials",
		RunE:  configure,
	}

	configureCommand.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := configureCommand.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	return configureCommand
}

// We can't properly run integration tests on code that calls PromptUI.
// This is because the first call to PromptUI reads from stdin correctly,
// but subsequent calls return EOF.
// The `userInterface` is created here to allow us to pass a mock user
// interface for testing.
type userInterface interface {
	readStringFromUser(message string, defaultValue string) string
	askUserToConfirm(message string) bool
}

type interactiveUI struct {
}

func (interactiveUI) readStringFromUser(message string, defaultValue string) string {
	prompt := promptui.Prompt{
		Label: message,
	}

	if defaultValue != "" {
		prompt.Default = defaultValue
	}

	token, err := prompt.Run()

	if err != nil {
		panic(err)
	}

	return token
}

func (interactiveUI) askUserToConfirm(message string) bool {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	return err == nil && strings.ToLower(result) == "y"
}

type testingUI struct {
	input   string
	confirm bool
}

func (ui testingUI) readStringFromUser(message string, defaultValue string) string {
	fmt.Println(message)
	return ui.input
}

func (ui testingUI) askUserToConfirm(message string) bool {
	fmt.Println(message)
	return ui.confirm
}

func shouldAskForToken(token string, ui userInterface) bool {

	if token == "" {
		return true
	}

	return ui.askUserToConfirm("A CircleCI token is already set. Do you want to change it")
}

func configure(cmd *cobra.Command, args []string) error {
	token := viper.GetString("token")

	var ui userInterface = interactiveUI{}

	if testing {
		ui = testingUI{
			confirm: true,
			input:   "boondoggle",
		}
	}

	if shouldAskForToken(token, ui) {
		viper.Set("token", ui.readStringFromUser("CircleCI API Token", ""))
		fmt.Println("API token has been set.")
	}
	viper.Set("endpoint", ui.readStringFromUser("CircleCI API End Point", viper.GetString("endpoint")))
	fmt.Println("API endpoint has been set.")

	// Marc: I can't find a way to prevent the verbose flag from
	// being written to the config file, so set it to false in
	// the config file.
	viper.Set("verbose", false)

	if err := viper.WriteConfig(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Println("Configuration has been saved.")
	return nil
}
