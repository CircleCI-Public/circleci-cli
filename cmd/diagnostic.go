package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var diagnosticCmd = &cobra.Command{
	Use:   "diagnostic",
	Short: "Check the status of your CircleCI CLI.",
	Run:   diagnostic,
}

func diagnostic(cmd *cobra.Command, args []string) {
	// TODO: Pass token once figure out how api-service uses it
	host := viper.GetString("host") + "/graphql"
	client := graphql.NewClient(host)
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

	req := graphql.NewRequest(query)

	ctx := context.Background()
	var resp map[string]interface{}

	fmt.Println("Querying", host, "with:\n", query, "\n")
	if err := client.Run(ctx, req, &resp); err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Result: \n")
	fmt.Println(string(b))
}
