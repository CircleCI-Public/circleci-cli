package cmd

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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

	req := Config.Client.NewAuthorizedRequest(string(q))
	err = Config.Client.Run(context.Background(), req, &resp)
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	Config.Logger.Prettyify(resp)

	return nil
}
