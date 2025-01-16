package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/spf13/cobra"
)

type diagnosticOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string
}

func newDiagnosticCommand(config *settings.Config) *cobra.Command {
	opts := diagnosticOptions{
		cfg: config,
	}

	diagnosticCommand := &cobra.Command{
		Use:   "diagnostic",
		Short: "Check the status of your CircleCI CLI.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.cl = graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := diagnostic(opts)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(telemetry.CreateDiagnosticEvent(err))
			}

			return err
		},
	}

	return diagnosticCommand
}

func diagnostic(opts diagnosticOptions) error {
	fmt.Println("\n---\nCircleCI CLI Diagnostics\n---")
	fmt.Printf("Debugger mode: %v\n", opts.cfg.Debug)
	fmt.Printf("Config found: %v\n", opts.cfg.FileUsed)
	fmt.Printf("API host: %s\n", opts.cfg.Host)
	fmt.Printf("API endpoint: %s\n", opts.cfg.Endpoint)

	if err := validateToken(opts.cfg); err != nil {
		return err
	}

	fmt.Println("OK, got a token.")

	responseWho, err := api.WhoamiQuery(opts.cl)

	if err != nil {
		return err
	}

	if responseWho.Me.Name != "" {
		fmt.Printf("Hello, %s.\n", responseWho.Me.Name)
	}

	return nil
}
