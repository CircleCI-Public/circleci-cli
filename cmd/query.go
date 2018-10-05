package cmd

import (
	"context"
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
		Use:         "query <path>",
		Short:       "Query the CircleCI GraphQL API.",
		RunE:        query,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	queryCommand.Annotations["<path>"] = "The path to your query (use \"-\" for STDIN)"

	return queryCommand
}

func query(cmd *cobra.Command, args []string) error {
	var err error
	// This local is named "q" to avoid confusion with the function name.
	var q []byte
	var resp map[string]interface{}

	if args[0] == "-" {
		q, err = ioutil.ReadAll(os.Stdin)
	} else {
		q, err = ioutil.ReadFile(args[0])
	}

	if err != nil {
		return errors.Wrap(err, "Unable to read query from stdin")
	}

	address, err := api.GraphQLServerAddress(api.EnvEndpointHost())
	if err != nil {
		return err
	}
	c := client.NewClient(address, Logger)

	req := client.NewAuthorizedRequest(viper.GetString("token"), string(q))
	err = c.Run(context.Background(), req, &resp)
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	Logger.Prettyify(resp)

	return nil
}
