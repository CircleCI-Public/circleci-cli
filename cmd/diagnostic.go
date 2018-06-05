package cmd

import (
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

	Logger.Info("\n---\nCircleCI CLI Diagnostics\n---\n\n")
	Logger.Infof("Config found: `%v`\n", viper.ConfigFileUsed())

	Logger.Infof("GraphQL API endpoint: %s\n", endpoint)

	if token == "token" || token == "" {
		Logger.Info("Please set a token!")
	} else {
		Logger.Info("OK, got a token.")
	}
}
