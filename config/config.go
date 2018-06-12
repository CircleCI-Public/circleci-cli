package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config is a struct of the current configuration available at runtime.
type Config struct {
	Verbose bool
	File    string
	Name    string
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

	c.File = c.defaultFile()

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

// defaultPath is the default path to the parent directory that contains the config file
func (c *Config) defaultPath() string {
	return fmt.Sprintf("%s/.circleci", os.Getenv("HOME"))
}

// defaultFile is the path to the default config file
func (c *Config) defaultFile() string {
	return fmt.Sprintf("%s/%s.yml", c.defaultPath(), c.Name)
}

func (c *Config) setupDefaults() error {
	if _, err := os.Stat(c.defaultPath()); os.IsNotExist(err) {
		if err = os.Mkdir(c.defaultPath(), 0700); err != nil {
			return fmt.Errorf("Error creating directory: '%s'", c.defaultPath())
		}
	}

	// Create default config file
	_, err := os.Create(c.defaultFile())
	return err
}

// create will generate a new config, after asking the user for their token.
func (c *Config) create() error {
	// Don't support creating config at --config flag, only default
	if c.File != "" {
		fmt.Printf("Setting up default config at: %v\n", c.defaultFile())
	}

	if err := c.setupDefaults(); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(c.defaultFile(), os.O_RDWR, 0600)
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
		if n, es := fmt.Scanln(&token); n < 0 {
			return es
		}
		fmt.Println("OK.")
	}

	// format input
	configValues := fmt.Sprintf("endpoint: %v\ntoken: %v\n", endpoint, token)

	// write new config values to file
	if _, err = file.WriteString(configValues); err != nil {
		return err
	}

	fmt.Printf("Your configuration has been created in `%v`.\n", c.defaultFile())
	fmt.Println("It can edited manually for advanced settings.")
	return err
}
