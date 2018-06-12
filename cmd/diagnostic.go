package cmd

import (
	"github.com/circleci/circleci-cli/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var diagnosticCmd = &cobra.Command{
	Use:   "diagnostic",
	Short: "Check the status of your CircleCI CLI.",
	Run:   diagnostic,
}

func diagnostic(cmd *cobra.Command, args []string) {
	endpoint := viper.GetString("endpoint")
	token := viper.GetString("token")

	config.Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---\n")
	config.Logger.Infof("Config found: %v\n", viper.ConfigFileUsed())

	config.Logger.Infof("GraphQL API endpoint: %s\n", endpoint)

	if token == "token" || token == "" {
		var err error
		config.Logger.FatalOnError("Please set a token!", err)
	} else {
		config.Logger.Infoln("OK, got a token.")
	}

	config.Logger.Infof("Verbose mode: %v\n", config.Config.Verbose)
}
