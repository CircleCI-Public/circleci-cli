package cmd

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type queryOptions struct {
	*settings.Config
	args []string
}

func newQueryCommand(config *settings.Config) *cobra.Command {
	opts := queryOptions{
		Config: config,
	}

	queryCommand := &cobra.Command{
		Use:   "query <path>",
		Short: "Query the CircleCI GraphQL API.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return query(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	queryCommand.Annotations["<path>"] = "The path to your query (use \"-\" for STDIN)"

	return queryCommand
}

func query(opts queryOptions) error {
	var err error
	// This local is named "q" to avoid confusion with the function name.
	var q []byte
	var resp map[string]interface{}

	if opts.args[0] == "-" {
		q, err = ioutil.ReadAll(os.Stdin)
	} else {
		q, err = ioutil.ReadFile(opts.args[0])
	}

	if err != nil {
		return errors.Wrap(err, "Unable to read query from stdin")
	}

	req, err := opts.Client.NewAuthorizedRequest(string(q))
	if err != nil {
		return err
	}
	err = opts.Client.Run(context.Background(), req, &resp)
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	opts.Logger.Prettyify(resp)

	return nil
}
