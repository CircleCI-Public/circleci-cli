package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/ui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var testing = false

type setupOptions struct {
	cfg      *settings.Config
	noPrompt bool
	// Add host and token for use with --no-prompt
	host  string
	token string
	args  []string
}

func newSetupCommand(config *settings.Config) *cobra.Command {
	opts := setupOptions{
		cfg: config,
	}

	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return setup(opts)
		},
	}

	setupCommand.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := setupCommand.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	setupCommand.Flags().BoolVar(&opts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI. (MUST supply --host and --token)")

	setupCommand.Flags().StringVar(&opts.host, "host", "", "URL to your CircleCI host")
	if err := setupCommand.Flags().MarkHidden("host"); err != nil {
		panic(err)
	}

	setupCommand.Flags().StringVar(&opts.token, "token", "", "your token for using CircleCI")
	if err := setupCommand.Flags().MarkHidden("token"); err != nil {
		panic(err)
	}

	return setupCommand
}

func setup(opts setupOptions) error {
	if opts.noPrompt {
		return setupNoPrompt(opts)
	}

	var tty ui.UserInterface = ui.InteractiveUI{}

	if testing {
		tty = ui.TestingUI{
			Confirm: true,
			Input:   "boondoggle",
		}
	}

	if ui.ShouldAskForToken(opts.cfg.Token, tty) {
		token, err := tty.ReadSecretStringFromUser("CircleCI API Token")
		if err != nil {
			return errors.Wrap(err, "Error reading token")
		}
		opts.cfg.Token = token
		fmt.Println("API token has been set.")
	}
	opts.cfg.Host = tty.ReadStringFromUser("CircleCI Host", defaultHost)
	fmt.Println("CircleCI host has been set.")

	// Reset endpoint to default when running setup
	// This ensures any accidental changes to this field can be fixed simply by rerunning this command.
	if ui.ShouldAskForEndpoint(opts.cfg.Endpoint, tty, defaultEndpoint) {
		opts.cfg.Endpoint = defaultEndpoint
	}

	if err := opts.cfg.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Printf("Setup complete.\nYour configuration has been saved to %s.\n", opts.cfg.FileUsed)
	return nil
}

func shouldKeepExistingConfig(opts setupOptions) bool {
	// Host will always be set, since it has a default value of circleci.com
	// We assume by an empty token there is no existing config.
	if opts.cfg.Token == "" {
		return false
	}

	// If they pass either host or token with a value this will be false, overwriting their existing config
	return opts.host == "" && opts.token == ""
}

func setupNoPrompt(opts setupOptions) error {
	if shouldKeepExistingConfig(opts) {
		fmt.Printf("Setup has kept your existing configuration at %s.\n", opts.cfg.FileUsed)
		return nil
	}

	// Throw an error if both flags are blank are blank!
	if opts.host == "" && opts.token == "" {
		return errors.New("No existing host or token saved.\nThe proper format is `circleci setup --host HOST --token TOKEN --no-prompt")
	}

	config := settings.Config{}

	// First calling load will ensure the new config can be saved to disk
	if err := config.LoadFromDisk(); err != nil {
		return errors.Wrap(err, "Failed to create config file on disk")
	}

	// Use the default endpoint since we don't expose that to users
	config.Endpoint = defaultEndpoint
	config.Host = opts.host   // Set new host to flag
	config.Token = opts.token // Set new token to flag

	// Reset their host if the flag was blank
	if opts.host == "" {
		fmt.Println("Host unchanged from existing config. Use --host with --no-prompt to overwrite it.")
		config.Host = opts.cfg.Host
	}

	// Reset their token if the flag was blank
	if opts.token == "" {
		fmt.Println("Token unchanged from existing config. Use --token with --no-prompt to overwrite it.")
		config.Token = opts.cfg.Token
	}

	// Then save the new config to disk
	if err := config.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Printf("Setup complete.\nYour configuration has been saved to %s.\n", config.FileUsed)
	return nil
}
