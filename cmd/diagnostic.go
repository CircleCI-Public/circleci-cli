package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newDiagnosticCommand() *cobra.Command {
	diagnosticCommand := &cobra.Command{
		Use:   "diagnostic",
		Short: "Check the status of your CircleCI CLI.",
		RunE:  diagnostic,
	}

	return diagnosticCommand
}

func diagnostic(cmd *cobra.Command, args []string) error {
	Config.Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---")
	Config.Logger.Infof("Debugger mode: %v\n", Config.Debug)
	Config.Logger.Infof("Config found: %v\n", Config.FileUsed)
	Config.Logger.Infof("GraphQL API address: %s\n", Config.Address)

	if Config.Token == "token" || Config.Token == "" {
		return errors.New("please set a token with 'circleci setup'")
	}
	Config.Logger.Infoln("OK, got a token.")

	Config.Logger.Infoln("Trying an introspection query on API... ")
	responseIntro, err := api.IntrospectionQuery(context.Background(), Config)
	if responseIntro.Data.Schema.QueryType.Name == "" {
		Config.Logger.Infoln("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	Config.Logger.Infoln("Ok.")

	Config.Logger.Debug("Introspection query result with Schema.QueryType of %s", responseIntro.Data.Schema.QueryType.Name)

	responseWho, err := api.WhoamiQuery(context.Background(), Config)

	if err != nil {
		return err
	}

	if responseWho.Data.Me.Name != "" {
		Config.Logger.Infof("Hello, %s.\n", responseWho.Data.Me.Name)
	}

	return nil
}
