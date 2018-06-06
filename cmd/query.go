package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/circleci/circleci-cli/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the CircleCI GraphQL API.",
	Run:   query,
}

func query(cmd *cobra.Command, args []string) {
	client := client.NewClient(viper.GetString("endpoint"), viper.GetString("token"), Logger)

	query, err := ioutil.ReadAll(os.Stdin)
	Logger.FatalOnError("Something happened", err)

	resp, err := client.Run(string(query))
	Logger.FatalOnError("Something happend", err)
	b, err := json.MarshalIndent(resp, "", "  ")
	Logger.FatalOnError("Could not parse graphql response", err)

	Logger.Infoln(string(b))
}
