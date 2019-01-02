package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/ui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var testing = false

type setupOptions struct {
	cfg      *settings.Config
	cl       *client.Client
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
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
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

func existingConfigExists(opts setupOptions) bool {
	return opts.cfg.Host != "" && opts.cfg.Token != ""
}

func setupNoPrompt(opts setupOptions) error {
	if existingConfigExists(opts) {
		fmt.Printf("Setup has kept your existing configuation at %s.\n", opts.cfg.FileUsed)
		return nil
	}

	var err error

	if opts.host == "" {
		err = errors.New("You must specify --host to use --no-prompt")
	}

	if opts.token == "" {
		msg := "You must specify --token to use --no-prompt"

		if err != nil {
			err = errors.Wrap(err, msg)
		} else {
			err = errors.New(msg)
		}
	}

	if err != nil {
		return errors.Wrap(err, "No existing host or token saved.\nThe proper format is `circleci setup --host HOST --token TOKEN --no-prompt")
	}

	config := settings.Config{}

	// First calling load will ensure the new config can be saved to disk
	if err = config.LoadFromDisk(); err != nil {
		return errors.Wrap(err, "Failed to create config file on disk")
	}

	// Update the config with flags passed in and inherit default endpoint
	config.Endpoint = defaultEndpoint
	config.Host = opts.host
	config.Token = opts.token

	// Then save the new config to disk
	if err := config.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Printf("Setup complete.\nYour configuration has been saved to %s.\n", config.FileUsed)
	return nil
}
