package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Use CircleCI from the command line.",
	Long:  `Use CircleCI from the command line.`,
}

var (
	cfgFile           string
	cfgName           = "cli"
	configPathDefault = fmt.Sprintf("%s/.circleci/%s.yml", os.Getenv("HOME"), cfgName)
)

func AddCommands() {
	RootCmd.AddCommand(diagnosticCmd)
	RootCmd.AddCommand(queryCmd)
}

// TODO: This convention was carried over from admin-cli, do we still need it?
// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	AddCommands()
	if err := RootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.circleci/cli.yml)")
	RootCmd.PersistentFlags().StringP("host", "H", "https://circleci.com", "the host of your CircleCI install")
	RootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	viper.BindPFlag("host", RootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
}

// TODO: move config stuff to it's own package
func initConfig() {
	if err := readConfig(); err != nil {
		if err = createConfig(); err != nil {
			fmt.Println(err.Error())
			os.Exit(-1)
		}
		cfgFile = configPathDefault
		readConfig() // reload config after creating it
	}
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
	// set a prefix for config, i.e. CIRCLECI_CLI_HOST
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	err = viper.ReadInConfig()
	return err
}

func createConfig() (err error) {
	// Don't support creating config at --config flag, only default
	if cfgFile != "" {
		fmt.Printf("Setting up default config at: %v\n", configPathDefault)
	}

	var host, token string

	path := fmt.Sprintf("%s/.circleci", os.Getenv("HOME"))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, 0777)
	}

	// Create default config file
	if _, err := os.Create(configPathDefault); err != nil {
		return err
	}

	// open file with read & write
	file, err := os.OpenFile(configPathDefault, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}

	// read flag values
	host = viper.GetString("host")
	token = viper.GetString("token")

	if host == "host" || host == "" {
		fmt.Print("Please enter the HTTP(S) host of your CircleCI installation:")
		fmt.Scanln(&host)
		fmt.Println("OK.")
	}

	if token == "token" || token == "" {
		fmt.Print("Please enter your CircleCI API token:")
		fmt.Scanln(&token)
		fmt.Println("OK.")
	}

	// format input
	configValues := fmt.Sprintf("host: %v\ntoken: %v\n", host, token)

	// write new config values to file
	if _, err = file.WriteString(configValues); err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}

	fmt.Printf("Your configuration has been created in `%v`.\n", configPathDefault)
	fmt.Println("It can edited manually for advanced settings.")
	return err
}
