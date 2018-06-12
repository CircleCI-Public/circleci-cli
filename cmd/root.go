package cmd

import (
	"os"

	"github.com/circleci/circleci-cli/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute adds all child commands to rootCmd and
// sets flags appropriately. This function is called
// by main.main(). It only needs to happen once to
// the RootCmd.
func Execute() {
	addCommands()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Use CircleCI from the command line.",
	Long:  `Use CircleCI from the command line.`,
}

func addCommands() {
	rootCmd.AddCommand(diagnosticCmd)
	rootCmd.AddCommand(queryCmd)
}

func init() {
	cobra.OnInitialize(config.Init)

	rootCmd.PersistentFlags().BoolVarP(&config.Config.Verbose, "verbose", "v", false, "Enable verbose logging.")

	rootCmd.PersistentFlags().StringVarP(&config.Config.File, "config", "c", "", "config file (default is $HOME/.circleci/cli.yml)")
	rootCmd.PersistentFlags().StringP("endpoint", "e", "https://circleci.com/graphql", "the endpoint of your CircleCI GraphQL API")
	rootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	config.Logger.FatalOnError("Error binding endpoint flag", viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint")))
	config.Logger.FatalOnError("Error binding token flag", viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token")))
}
