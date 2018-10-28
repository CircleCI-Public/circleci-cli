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
	cfg     *settings.Config
	apiOpts api.Options
	args    []string
}

func newDiagnosticCommand(config *settings.Config) *cobra.Command {
	opts := diagnosticOptions{
		apiOpts: api.Options{},
		cfg:     config,
	}

	diagnosticCommand := &cobra.Command{
		Use:   "diagnostic",
		Short: "Check the status of your CircleCI CLI.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.apiOpts.Context = context.Background()
			opts.apiOpts.Log = logger.NewLogger(config.Debug)
			opts.apiOpts.Client = client.NewClient(config.Host, config.Endpoint, config.Token)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return diagnostic(opts)
		},
	}

	return diagnosticCommand
}

func diagnostic(opts diagnosticOptions) error {
	log := opts.apiOpts.Log
	log.Infoln("\n---\nCircleCI CLI Diagnostics\n---")
	log.Infof("Debugger mode: %v\n", opts.cfg.Debug)
	log.Infof("Config found: %v\n", opts.cfg.FileUsed)
	log.Infof("API host: %s\n", opts.cfg.Host)
	log.Infof("API endpoint: %s\n", opts.cfg.Endpoint)

	if opts.cfg.Token == "token" || opts.cfg.Token == "" {
		return errors.New("please set a token with 'circleci setup'")
	}
	log.Infoln("OK, got a token.")

	log.Infoln("Trying an introspection query on API... ")

	responseIntro, err := api.IntrospectionQuery(opts.apiOpts)
	if responseIntro.Schema.QueryType.Name == "" {
		log.Infoln("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	log.Infoln("Ok.")

	log.Debug("Introspection query result with Schema.QueryType of %s", responseIntro.Schema.QueryType.Name)

	responseWho, err := api.WhoamiQuery(opts.apiOpts)

	if err != nil {
		return err
	}

	if responseWho.Me.Name != "" {
		log.Infof("Hello, %s.\n", responseWho.Me.Name)
	}

	return nil
}
