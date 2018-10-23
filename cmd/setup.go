package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/ui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var testing = false

type setupOptions struct {
	cfg  *settings.Config
	cl   *client.Client
	log  *logger.Logger
	args []string
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return setup(opts)
		},
	}

	setupCommand.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := setupCommand.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	return setupCommand
}

func setup(opts setupOptions) error {
	var tty ui.UserInterface = ui.InteractiveUI{}

	if testing {
		tty = ui.TestingUI{
			Confirm: true,
			Input:   "boondoggle",
		}
	}

	if ui.ShouldAskForToken(opts.cfg.Token, opts.log, tty) {
		token, err := tty.ReadSecretStringFromUser(opts.log, "CircleCI API Token")
		if err != nil {
			return errors.Wrap(err, "Error reading token")
		}
		opts.cfg.Token = token
		opts.log.Info("API token has been set.")
	}
	opts.cfg.Host = tty.ReadStringFromUser(opts.log, "CircleCI Host", defaultHost)
	opts.log.Info("CircleCI host has been set.")

	// Reset endpoint to default when running setup
	// This ensures any accidental changes to this field can be fixed simply by rerunning this command.
	if ui.ShouldAskForEndpoint(opts.cfg.Endpoint, opts.log, tty, defaultEndpoint) {
		opts.cfg.Endpoint = defaultEndpoint
	}

	if err := opts.cfg.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	opts.log.Info("Setup complete. Your configuration has been saved.")
	return nil
}
