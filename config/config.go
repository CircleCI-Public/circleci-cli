package config

import (
	"fmt"
	"os"
	"path/filepath"

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

	// reload after creating config
	err := c.read()
	return err
}

// read tries to load the config either from (*Config).defaultPath() or (*Config).File.
func (c *Config) read() error {
	file, err := c.flagOrDefaultFile()
	if err != nil {
		return err
	}
	viper.SetConfigFile(file)

	// read in environment variables that match
	// set a prefix for config, i.e. CIRCLECI_CLI_ENDPOINT
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err = viper.ReadInConfig()
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

func (c *Config) configPath() (string, error) {
	file, err := c.flagOrDefaultFile()
	if err != nil {
		return c.defaultFile(), err
	}

	return filepath.Abs(filepath.Dir(file))
}

func (c *Config) flagOrDefaultFile() (string, error) {
	if c.File != "" {
		absFile, err := filepath.Abs(c.File)
		if err != nil {
			return c.defaultFile(), err
		}
		return absFile, err
	}

	return c.defaultFile(), nil
}

func (c *Config) setup() error {
	path, err := c.configPath()

	if err != nil {
		return err
	}

	if _, err = os.Stat(path); os.IsNotExist(err) {
		if err = os.Mkdir(path, 0700); err != nil {
			return fmt.Errorf("Error creating directory: '%s'", path)
		}
	}

	// Create config file
	file, err := c.flagOrDefaultFile()
	if err != nil {
		return err
	}
	_, err = os.Create(file)
	return err
}

// create will generate a new config, after asking the user for their token.
func (c *Config) create() error {
	cfg, err := c.flagOrDefaultFile()
	if err != nil {
		return err
	}
	fmt.Printf("Setting up default config at: %v\n", cfg)

	if err = c.setup(); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(cfg, os.O_RDWR, 0600)
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

	fmt.Printf("Your configuration has been created in `%v`.\n", cfg)
	fmt.Println("It can edited manually for advanced settings.")
	return err
}
