package client

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/machinebox/graphql"
)

// NewClient returns a reference to a Client.
// We also call graphql.NewClient to initialize a new GraphQL Client.
// Then we pass the Logger originally constructed as cmd.Logger.
func NewClient(endpoint string, logger *logger.Logger) *graphql.Client {

	client := graphql.NewClient(endpoint)

	client.Log = func(s string) {
		logger.Debug(fmt.Sprintf("[machinebox/graphql] %s", s))
	}

	return client

}

// NewAuthorizedRequest returns a new GraphQL request with the
// authorization headers set for CircleCI auth.
func NewAuthorizedRequest(token, query string) *graphql.Request {
	req := graphql.NewRequest(query)
	req.Header.Set("Authorization", token)
	return req
}

// Run will construct a request using graphql.NewRequest.
// Then it will execute the given query using graphql.Client.Run.
// This function will return the unmarshalled response as JSON.
func Run(client *graphql.Client, token, query string) (map[string]interface{}, error) {
	req := NewAuthorizedRequest(token, query)
	ctx := context.Background()
	var resp map[string]interface{}
	err := client.Run(ctx, req, &resp)
	return resp, err
}
