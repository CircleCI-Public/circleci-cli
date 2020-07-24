package rest_client

import (
	"context"
	"net/http"
	"time"
)

type Context struct{
	CreatedAt time.Time
	Id string
	Name string
}

func authenticateFn(token string) func(context.Context, *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Add("circle-token", token)
		req.Header.Add("Accept", "application/json")
		return nil
	}
}


func NewAuthenticatedClient(server string, token string) (*ClientWithResponses, error) {
	return NewClientWithResponses(server, []ClientOption{
		WithRequestEditorFn(authenticateFn(token)),
	}...)
}
