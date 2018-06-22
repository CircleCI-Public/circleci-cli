package cmd

import (
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the CircleCI GraphQL API.",
	RunE:  query,
}

func query(cmd *cobra.Command, args []string) error {
	c := client.NewClient(viper.GetString("endpoint"), Logger)

	query, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "Unable to read query from stdin")
	}

	resp, err := client.Run(c, viper.GetString("token"), string(query))
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	Logger.Prettyify(resp)

	return nil
}
