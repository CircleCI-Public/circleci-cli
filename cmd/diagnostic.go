package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type diagnosticOptions struct {
	*settings.Config
	args []string
}

func newDiagnosticCommand(config *settings.Config) *cobra.Command {
	opts := diagnosticOptions{
		Config: config,
	}

	diagnosticCommand := &cobra.Command{
		Use:   "diagnostic",
		Short: "Check the status of your CircleCI CLI.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return diagnostic(opts)
		},
	}

	return diagnosticCommand
}

func diagnostic(opts diagnosticOptions) error {
	opts.Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---")
	opts.Logger.Infof("Debugger mode: %v\n", opts.Debug)
	opts.Logger.Infof("Config found: %v\n", opts.FileUsed)
	opts.Logger.Infof("GraphQL API address: %s\n", opts.Address)

	if opts.Token == "token" || opts.Token == "" {
		return errors.New("please set a token with 'circleci setup'")
	}
	opts.Logger.Infoln("OK, got a token.")

	opts.Logger.Infoln("Trying an introspection query on API... ")
	responseIntro, err := api.IntrospectionQuery(context.Background(), opts.Config)
	if responseIntro.Data.Schema.QueryType.Name == "" {
		opts.Logger.Infoln("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	opts.Logger.Infoln("Ok.")

	opts.Logger.Debug("Introspection query result with Schema.QueryType of %s", responseIntro.Data.Schema.QueryType.Name)

	responseWho, err := api.WhoamiQuery(context.Background(), opts.Config)

	if err != nil {
		return err
	}

	if responseWho.Data.Me.Name != "" {
		opts.Logger.Infof("Hello, %s.\n", responseWho.Data.Me.Name)
	}

	return nil
}
