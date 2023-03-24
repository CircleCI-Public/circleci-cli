package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type queryOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string
}

func newQueryCommand(config *settings.Config) *cobra.Command {
	opts := queryOptions{
		cfg: config,
	}

	queryCommand := &cobra.Command{
		Use:    "query <path>",
		Short:  "Query the CircleCI GraphQL API.",
		Hidden: true,
		PreRunE: func(_ *cobra.Command, args []string) error {
			opts.args = args
			opts.cl = graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

			return validateToken(opts.cfg)
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
		q, err = io.ReadAll(os.Stdin)
	} else {
		q, err = os.ReadFile(opts.args[0])
	}

	if err != nil {
		return errors.Wrap(err, "Unable to read query from stdin")
	}

	req := graphql.NewRequest(string(q))
	req.SetToken(opts.cl.Token)

	err = opts.cl.Run(req, &resp)
	if err != nil {
		return errors.Wrap(err, "Error occurred when running query")
	}

	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bytes))

	return nil
}
