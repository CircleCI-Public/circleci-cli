package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type diagnosticOptions struct {
	cfg  *settings.Config
	cl   *client.Client
	log  *logger.Logger
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return diagnostic(opts)
		},
	}

	return diagnosticCommand
}

func diagnostic(opts diagnosticOptions) error {
	opts.log.Infoln("\n---\nCircleCI CLI Diagnostics\n---")
	opts.log.Infof("Debugger mode: %v\n", opts.cfg.Debug)
	opts.log.Infof("Config found: %v\n", opts.cfg.FileUsed)
	opts.log.Infof("API host: %s\n", opts.cfg.Host)
	opts.log.Infof("API endpoint: %s\n", opts.cfg.Endpoint)

	if opts.cfg.Token == "token" || opts.cfg.Token == "" {
		return errors.New("please set a token with 'circleci setup'")
	}
	opts.log.Infoln("OK, got a token.")

	opts.log.Infoln("Trying an introspection query on API... ")
	responseIntro, err := api.IntrospectionQuery(context.Background(), opts.log, opts.cl)
	if responseIntro.Schema.QueryType.Name == "" {
		opts.log.Infoln("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	opts.log.Infoln("Ok.")

	opts.log.Debug("Introspection query result with Schema.QueryType of %s", responseIntro.Schema.QueryType.Name)

	responseWho, err := api.WhoamiQuery(context.Background(), opts.log, opts.cl)

	if err != nil {
		return err
	}

	if responseWho.Me.Name != "" {
		opts.log.Infof("Hello, %s.\n", responseWho.Me.Name)
	}

	return nil
}
