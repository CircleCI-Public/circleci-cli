package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var diagnosticCmd = &cobra.Command{
	Use:   "diagnostic",
	Short: "Check the status of your CircleCI CLI.",
	Run:   diagnostic,
}

func diagnostic(cmd *cobra.Command, args []string) {
	host := viper.GetString("host")
	token := viper.GetString("token")

	fmt.Printf("\n---\nCircleCI CLI Diagnostics\n---\n\n")
	fmt.Printf("Config found: `%v`\n", viper.ConfigFileUsed())

	if host == "host" || host == "" {
		fmt.Println("Please set a host!")
	} else {
		fmt.Printf("Host is: %s\n", host)
	}

	if token == "token" || token == "" {
		fmt.Println("Please set a token!")
	} else {
		fmt.Println("OK, got a token.")
	}
}
