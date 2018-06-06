package cmd

import (
	"fmt"
	"os"

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

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Use CircleCI from the command line.",
	Long:  `Use CircleCI from the command line.`,
}

var (
	verbose        bool
	cfgFile        string
	cfgName        = "cli"
	cfgPathDefault = fmt.Sprintf("%s/.circleci/%s.yml", os.Getenv("HOME"), cfgName)
)

// Logger is exposed here so we can access it from subcommands.
// This allows us to print to the log at anytime from within the `cmd` package.
var Logger *logger.Logger

func addCommands() {
	rootCmd.AddCommand(diagnosticCmd)
	rootCmd.AddCommand(queryCmd)
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging.")
	Logger = logger.NewLogger(verbose)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.circleci/cli.yml)")
	rootCmd.PersistentFlags().StringP("endpoint", "e", "https://circleci.com/graphql", "the endpoint of your CircleCI GraphQL API")
	rootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	Logger.FatalOnError("Error binding endpoint flag", viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint")))
	Logger.FatalOnError("Error binding token flag", viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token")))
}

// TODO: move config stuff to it's own package
func initConfig() {
	if err := readConfig(); err == nil {
		return
	}

	Logger.FatalOnError("Error creating a new config file", createConfig())

	cfgFile = cfgPathDefault
	Logger.FatalOnError(
		"Failed to re-read config after creating a new file",
		readConfig(), // reload config after creating it
	)
}

func readConfig() (err error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	if viper.ConfigFileUsed() == "" {
		viper.AddConfigPath("$HOME/.circleci")
		viper.SetConfigName(cfgName)
	}

	// read in environment variables that match
	// set a prefix for config, i.e. CIRCLECI_CLI_ENDPOINT
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err = viper.ReadInConfig()
	return err
}

func createConfig() (err error) {
	// Don't support creating config at --config flag, only default
	if cfgFile != "" {
		Logger.Debug("Setting up default config at: %v\n", cfgPathDefault)
	}

	path := fmt.Sprintf("%s/.circleci", os.Getenv("HOME"))

	if _, err = os.Stat(path); os.IsNotExist(err) {
		Logger.FatalOnError(
			fmt.Sprintf("Error creating directory: '%s'", path),
			os.Mkdir(path, 0644),
		)
	} else {
		Logger.FatalOnError(fmt.Sprintf("Error accessing '%s'", path), err)
	}

	// Create default config file
	if _, err = os.Create(cfgPathDefault); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(cfgPathDefault, os.O_RDWR, 0600)
	if err != nil {
		Logger.FatalOnError("", err)
	}
	defer func() {
	  Logger.FatalOnError("Error closing config file", file.Close())
	}()

	// read flag values
	endpoint := viper.GetString("endpoint")
	token := viper.GetString("token")

	if token == "token" || token == "" {
		Logger.Info("Please enter your CircleCI API token: ")
		fmt.Scanln(&token)
		Logger.Infoln("OK.")
	}

	// format input
	configValues := fmt.Sprintf("endpoint: %v\ntoken: %v\n", endpoint, token)

	// write new config values to file
	if _, err = file.WriteString(configValues); err != nil {
		Logger.FatalOnError("", err)
	}

	Logger.Info("Your configuration has been created in `%v`.\n", cfgPathDefault)
	Logger.Infoln("It can edited manually for advanced settings.")
	return err
}
