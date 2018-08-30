package cmd

import (
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newQueryCommand() *cobra.Command {
	queryCommand := &cobra.Command{
		Use:         "query PATH",
		Short:       "Query the CircleCI GraphQL API.",
		RunE:        query,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	queryCommand.Annotations["PATH"] = "The path to your query (use \"-\" for STDIN)"

	return queryCommand
}

func query(cmd *cobra.Command, args []string) error {
	var err error
	// This local is named "q" to avoid confusion with the function name.
	var q []byte
	address, err := api.GraphQLServerAddress(api.EnvEndpointHost())
	if err != nil {
		return err
	}
	c := client.NewClient(address, Logger)

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
