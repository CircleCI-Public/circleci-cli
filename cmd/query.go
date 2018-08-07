package cmd

import (
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newQueryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "query PATH (use \"-\" for STDIN)",
		Short: "Query the CircleCI GraphQL API.",
		RunE:  query,
		Args:  cobra.ExactArgs(1),
	}
}

func query(cmd *cobra.Command, args []string) error {
	var err error
	var q []byte
	c := client.NewClient(viper.GetString("endpoint"), Logger)

	if args[0] == "-" {
		q, err = ioutil.ReadAll(os.Stdin)
	} else {
		q, err = ioutil.ReadFile(args[0])
	}

	if err != nil {
		return errors.Wrap(err, "Unable to read query from stdin")
	}

	resp, err := client.Run(c, viper.GetString("token"), string(q))
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	Logger.Prettyify(resp)

	return nil
}
