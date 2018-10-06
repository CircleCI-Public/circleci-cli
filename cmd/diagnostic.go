package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	address, err := api.GraphQLServerAddress(api.EnvEndpointHost())
	if err != nil {
		return err
	}

	token := viper.GetString("token")

	Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---")
	Logger.Infof("Debugger mode: %v\n", viper.GetBool("debug"))

	Logger.Infof("Config found: %v\n", viper.ConfigFileUsed())

	Logger.Infof("GraphQL API address: %s\n", address)

	if token == "token" || token == "" {
		return errors.New("please set a token with 'circleci setup'")
	}
	Logger.Infoln("OK, got a token.")

	Logger.Infoln("Trying an introspection query on API... ")
	responseIntro, err := api.IntrospectionQuery(context.Background(), Logger)
	if responseIntro.Data.Schema.QueryType.Name == "" {
		Logger.Infoln("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	Logger.Infoln("Ok.")

	Logger.Debug("Introspection query result with Schema.QueryType of %s", responseIntro.Data.Schema.QueryType.Name)

	responseWho, err := api.WhoamiQuery(context.Background(), Logger)

	if err != nil {
		return err
	}

	if responseWho.Data.Me.Name != "" {
		Logger.Infof("Hello, %s.\n", responseWho.Data.Me.Name)
	}

	return nil
}
