package client

import (
	"context"

	"github.com/circleci/circleci-cli/logger"
	"github.com/machinebox/graphql"
)

type Client struct {
	endpoint string
	token    string
	client   *graphql.Client
	logger   *logger.Logger
}

func NewClient(endpoint string, token string, logger *logger.Logger) *Client {
	return &Client{
		endpoint,
		token,
		graphql.NewClient(endpoint),
		logger,
	}
}

func (c *Client) Run(query string) (map[string]interface{}, error) {
	req := graphql.NewRequest(query)
	req.Header.Set("Authorization", c.token)

	ctx := context.Background()
	var resp map[string]interface{}

	c.logger.Info("Querying ", c.endpoint, " with:\n\n", query, "\n\n")
	err := c.client.Run(ctx, req, &resp)
	return resp, err
}
