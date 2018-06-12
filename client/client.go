package client

import (
	"context"
	"encoding/json"

	"github.com/circleci/circleci-cli/logger"
	"github.com/machinebox/graphql"
)

// Client wraps a graphql.Client and other fields for making API calls.
type Client struct {
	endpoint string
	token    string
	client   *graphql.Client
	logger   *logger.Logger
}

// NewClient returns a reference to a Client.
// We also call graphql.NewClient to initialize a new GraphQL Client.
// Then we pass the Logger originally constructed as cmd.Logger.
func NewClient(endpoint string, token string, logger *logger.Logger) *Client {
	return &Client{
		endpoint,
		token,
		graphql.NewClient(endpoint),
		logger,
	}
}

// Run will construct a request using graphql.NewRequest.
// Then it will execute the given query using graphql.Client.Run.
// This function will return the unmarshalled response as JSON.
func (c *Client) Run(query string) (string, error) {
	req := graphql.NewRequest(query)
	req.Header.Set("Authorization", c.token)

	ctx := context.Background()
	var resp map[string]interface{}

	c.logger.Debug("Querying %s with:\n\n%s\n\n", c.endpoint, query)
	err := c.client.Run(ctx, req, &resp)
	if err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), err
}
