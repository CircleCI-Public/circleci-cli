package telemetry

import "context"

type contextKey string

const (
	telemetryClientContextKey contextKey = "telemetryClientContextKey"
	telemetryInvocationIDKey  contextKey = "telemetryInvocationIDKey"
)

func NewContext(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, telemetryClientContextKey, client)
}

func FromContext(ctx context.Context) (Client, bool) {
	client, ok := ctx.Value(telemetryClientContextKey).(Client)
	return client, ok
}

// WithInvocationID stores the invocation ID in the context so that funnel
// step events in subpackages can correlate with the wrapper start/finish events.
func WithInvocationID(ctx context.Context, invocationID string) context.Context {
	return context.WithValue(ctx, telemetryInvocationIDKey, invocationID)
}

// InvocationIDFromContext retrieves the invocation ID set by the command middleware.
// Returns ("", false) if no ID is present.
func InvocationIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(telemetryInvocationIDKey).(string)
	return id, ok && id != ""
}
