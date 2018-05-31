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

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the CircleCI GraphQL API.",
	Run:   query,
}

func query(cmd *cobra.Command, args []string) {
	host := viper.GetString("host")
	token := viper.GetString("token")
	client := graphql.NewClient(host + "/graphql")

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
	req.Header.Set("Authorization", token)

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
