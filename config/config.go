package config

import (
	"fmt"
	"os"

	"github.com/circleci/circleci-cli/logger"
	"github.com/spf13/viper"
)

type config struct {
	Verbose     bool
	File        string
	Name        string
	DefaultPath string
}

// Logger is exposed here so we can access it from subcommands.
// Use this to print to the log at anytime.
var Logger *logger.Logger

// Config is a struct of the current configuration available at runtime.
var Config = &config{
	Verbose:     false,
	Name:        "cli",
	DefaultPath: fmt.Sprintf("%s/.circleci/cli.yml", os.Getenv("HOME")),
}

// Init is called on initialize of the root command.
func Init() {
	Logger = logger.NewLogger(Config.Verbose)
	if err := Read(); err == nil {
		return
	}

	Logger.FatalOnError("Error creating a new config file", Create())

	Config.File = Config.DefaultPath
	Logger.FatalOnError(
		"Failed to re-read config after creating a new file",
		Read(), // reload config after creating it
	)
}

// Read tries to load the config either from Config.DefaultPath or Config.File.
func Read() (err error) {
	if Config.File != "" {
		viper.SetConfigFile(Config.File)
	}

	if viper.ConfigFileUsed() == "" {
		viper.AddConfigPath("$HOME/.circleci")
		viper.SetConfigName(Config.Name)
	}

	// read in environment variables that match
	// set a prefix for config, i.e. CIRCLECI_CLI_ENDPOINT
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err = viper.ReadInConfig()
	return err
}

// Create will generate a new config, after asking the user for their token.
func Create() (err error) {
	// Don't support creating config at --config flag, only default
	if Config.File != "" {
		Logger.Debug("Setting up default config at: %v\n", Config.DefaultPath)
	}

	path := fmt.Sprintf("%s/.circleci", os.Getenv("HOME"))

	if _, err = os.Stat(path); os.IsNotExist(err) {
		Logger.FatalOnError(
			fmt.Sprintf("Error creating directory: '%s'", path),
			os.Mkdir(path, 0700),
		)
	} else {
		Logger.FatalOnError(fmt.Sprintf("Error accessing '%s'", path), err)
	}

	// Create default config file
	if _, err = os.Create(Config.DefaultPath); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(Config.DefaultPath, os.O_RDWR, 0600)
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

	Logger.Infof("Your configuration has been created in `%v`.\n", Config.DefaultPath)
	Logger.Infoln("It can edited manually for advanced settings.")
	return err
}
