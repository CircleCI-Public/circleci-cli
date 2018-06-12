package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/circleci/circleci-cli/client"
	"github.com/circleci/circleci-cli/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the CircleCI GraphQL API.",
	Run:   query,
}

func query(cmd *cobra.Command, args []string) {
	client := client.NewClient(viper.GetString("endpoint"), viper.GetString("token"))

	query, err := ioutil.ReadAll(os.Stdin)
	config.Logger.FatalOnError("Something happened", err)

	resp, err := client.Run(string(query))
	config.Logger.FatalOnError("Something happend", err)
	b, err := json.MarshalIndent(resp, "", "  ")
	config.Logger.FatalOnError("Could not parse graphql response", err)

	config.Logger.Infoln(string(b))
}
