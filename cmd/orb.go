package cmd

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newOrbCommand() *cobra.Command {

	orbListCommand := &cobra.Command{
		Use:   "list",
		Short: "List orbs",
		RunE:  listOrbs,
	}

	orbCommand := &cobra.Command{
		Use:   "orb",
		Short: "Operate on orbs",
	}

	orbCommand.AddCommand(orbListCommand)

	return orbCommand
}

func listOrbs(cmd *cobra.Command, args []string) error {

	ctx := context.Background()

	// Define a structure that matches the result of the GQL
	// query, so that we can use mapstructure to convert from
	// nested maps to a strongly typed struct.
	type orbList struct {
		Orbs struct {
			TotalCount int
			Edges      []struct {
				Cursor string
				Node   struct {
					Name string
				}
			}
			PageInfo struct {
				HasNextPage bool
			}
		}
	}

	request := graphql.NewRequest(`
query ListOrbs ($after: String!) {
  orbs(first: 20, after: $after) {
	totalCount,
    edges {
      cursor,
      node {
        name
      }
    }
    pageInfo {
      hasNextPage
    }
  }
}
	`)

	client := client.NewClient(viper.GetString("endpoint"), Logger)

	var result orbList
	currentCursor := ""

	for {
		request.Var("after", currentCursor)
		err := client.Run(ctx, request, &result)

		if err != nil {
			return errors.Wrap(err, "GraphQL query failed")
		}

		Logger.Prettyify(result)

		fmt.Printf("Total Number Of Orbs: %d\n", result.Orbs.TotalCount)

		for i := range result.Orbs.Edges {
			edge := result.Orbs.Edges[i]
			currentCursor = edge.Cursor
			Logger.Infof("Orb: %s\n", edge.Node.Name)
		}

		if !result.Orbs.PageInfo.HasNextPage {
			break
		}
	}
	return nil

}
