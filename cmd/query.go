package cmd

import (
	"encoding/json"

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
	client := client.NewClient(viper.GetString("host"), viper.GetString("token"), Logger)

	query := `
		  query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      subscriptionType { name }
		      types {
		        ...FullType
		      }
		      directives {
		        name
		        description
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

	resp, err := client.Run(query)
	Logger.FatalOnError("Something happend", err)
	b, err := json.MarshalIndent(resp, "", "  ")
	Logger.FatalOnError("Could not parse graphql response", err)

	Logger.Info("Result: \n\n")
	Logger.Info(string(b) + "\n")
}
