package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config is a struct of the current configuration available at runtime.
type Config struct {
	Verbose     bool
	File        string
	Name        string
	DefaultPath string
}

// Init is called on initialize of the root command.
func (c *Config) Init() error {
	// try to read the config for the user
	if err := c.read(); err == nil {
		return err
	}

	// create a new config, prompting the user
	if err := c.create(); err != nil {
		return err
	}

	c.File = c.DefaultPath

	// reload after creating config
	err := c.read()
	return err
}

// read tries to load the config either from Config.defaultPath or Config.file.
func (c *Config) read() error {
	if c.File != "" {
		viper.SetConfigFile(c.File)
	}

	if viper.ConfigFileUsed() == "" {
		viper.AddConfigPath("$HOME/.circleci")
		viper.SetConfigName(c.Name)
	}

	// read in environment variables that match
	// set a prefix for config, i.e. CIRCLECI_CLI_ENDPOINT
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	return err
}

// create will generate a new config, after asking the user for their token.
func (c *Config) create() error {
	// Don't support creating config at --config flag, only default
	if c.File != "" {
		fmt.Printf("Setting up default config at: %v\n", c.DefaultPath)
	}

	path := fmt.Sprintf("%s/.circleci", os.Getenv("HOME"))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.Mkdir(path, 0700); err != nil {
			return fmt.Errorf("Error creating directory: '%s'", path)
		}
	} else {
		return fmt.Errorf("Error accessing directory: '%s'", path)
	}

	// Create default config file
	if _, err := os.Create(c.DefaultPath); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(c.DefaultPath, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() {
		cerr := file.Close()
		if err == nil {
			err = cerr
		}
	}()

	// read flag values
	endpoint := viper.GetString("endpoint")
	token := viper.GetString("token")

	if token == "token" || token == "" {
		fmt.Print("Please enter your CircleCI API token: ")
		if _, err = fmt.Scanln(&token); err != nil {
			return err
		}
		fmt.Println("OK.")
	}

	// format input
	configValues := fmt.Sprintf("endpoint: %v\ntoken: %v\n", endpoint, token)

	// write new config values to file
	if _, err = file.WriteString(configValues); err != nil {
		return err
	}

	fmt.Printf("Your configuration has been created in `%v`.\n", c.DefaultPath)
	fmt.Println("It can edited manually for advanced settings.")
	return err
}
