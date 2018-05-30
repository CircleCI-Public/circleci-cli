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

func AddCommands() {
	RootCmd.AddCommand(diagnosticCmd)
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

	RootCmd.PersistentFlags().StringP("host", "H", "https://circleci.com", "the host of your CircleCI install")
	RootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	viper.BindPFlag("host", RootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
}

func initConfig() {
	if viper.ConfigFileUsed() == "" {
		viper.SetConfigName("cli") // name of config file (without extension)
		viper.AddConfigPath("$HOME/.circleci")
		// If a config file is found, read it in.
		viper.AutomaticEnv() // read in environment variables that match
		if err := viper.ReadInConfig(); err != nil {
			fmt.Println("Failed to load config file...")
		}
	}
}
