package cmd

import (
	"fmt"
	"os"

	"github.com/circleci/circleci-cli/config"
	"github.com/circleci/circleci-cli/logger"
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

// Logger is exposed here so we can access it from subcommands.
// This allows us to print to the log at anytime from within the `cmd` package.
var Logger *logger.Logger

// Config is the current configuration available to all commands in `cmd` package.
var Config = &config.Config{
	Verbose:     false,
	Name:        "cli",
	DefaultPath: fmt.Sprintf("%s/.circleci/cli.yml", os.Getenv("HOME")),
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
	cobra.OnInitialize(setup)

	rootCmd.PersistentFlags().BoolVarP(&Config.Verbose, "verbose", "v", false, "Enable verbose logging.")

	rootCmd.PersistentFlags().StringVarP(&Config.File, "config", "c", "", "config file (default is $HOME/.circleci/cli.yml)")
	rootCmd.PersistentFlags().StringP("endpoint", "e", "https://circleci.com/graphql", "the endpoint of your CircleCI GraphQL API")
	rootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	Logger.FatalOnError("Error binding endpoint flag", viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint")))
	Logger.FatalOnError("Error binding token flag", viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token")))
}

func setup() {
	Logger = logger.NewLogger(Config.Verbose)
	Logger.FatalOnError(
		"Failed to setup configuration",
		Config.Init(),
	)
}
