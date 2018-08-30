package cmd

import (
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

	Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---\n")
	Logger.Infof("Config found: %v\n", viper.ConfigFileUsed())

	Logger.Infof("GraphQL API address: %s\n", address)

	if token == "token" || token == "" {
		return errors.New("please set a token")
	}
	Logger.Infoln("OK, got a token.")
	Logger.Infof("Verbose mode: %v\n", viper.GetBool("verbose"))

	return nil
}
