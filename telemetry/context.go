package telemetry

import "context"

type contextKey string

const telemetryClientContextKey contextKey = "telemetryClientContextKey"

func NewContext(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, telemetryClientContextKey, client)
}

func FromContext(ctx context.Context) (Client, bool) {
	client, ok := ctx.Value(telemetryClientContextKey).(Client)
	return client, ok
}
