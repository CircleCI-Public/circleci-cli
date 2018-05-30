package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check status of CircleCI",
	Run:   check,
}

func check(cmd *cobra.Command, args []string) {
	client := graphql.NewClient("https://circleci.com/graphql")

	req := graphql.NewRequest(`
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
  }
`)

	ctx := context.Background()
	var resp map[string]interface{}

	if err := client.Run(ctx, req, &resp); err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Print(string(b))
}
