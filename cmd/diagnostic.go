package cmd

import (
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type diagnosticOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string
}

func newDiagnosticCommand(config *settings.Config) *cobra.Command {
	opts := diagnosticOptions{
		cfg: config,
	}

	diagnosticCommand := &cobra.Command{
		Use:   "diagnostic",
		Short: "Check the status of your CircleCI CLI.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return diagnostic(opts)
		},
	}

	return diagnosticCommand
}

func diagnostic(opts diagnosticOptions) error {
	fmt.Println("\n---\nCircleCI CLI Diagnostics\n---")
	fmt.Printf("Debugger mode: %v\n", opts.cfg.Debug)
	fmt.Printf("Config found: %v\n", opts.cfg.FileUsed)
	fmt.Printf("API host: %s\n", opts.cfg.Host)
	fmt.Printf("API endpoint: %s\n", opts.cfg.Endpoint)

	if err := validateToken(opts.cfg); err != nil {
		return err
	}

	fmt.Println("OK, got a token.")

	fmt.Println("Trying an introspection query on API... ")

	responseIntro, err := api.IntrospectionQuery(opts.cl)
	if responseIntro.Schema.QueryType.Name == "" {
		fmt.Println("Unable to make a query against the GraphQL API, please check your settings")
		if err != nil {
			return err
		}
	}

	fmt.Println("Ok.")

	if opts.cfg.Debug {
		_, err = fmt.Fprintf(os.Stderr, "Introspection query result with Schema.QueryType of %s", responseIntro.Schema.QueryType.Name)
		if err != nil {
			return err
		}
	}

	responseWho, err := api.WhoamiQuery(opts.cl)

	if err != nil {
		return err
	}

	if responseWho.Me.Name != "" {
		fmt.Printf("Hello, %s.\n", responseWho.Me.Name)
	}

	return nil
}
