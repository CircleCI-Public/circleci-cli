package telemetry

import (
	"fmt"
	"io"

	"github.com/segmentio/analytics-go"
)

type Approval string

const (
	Enabled  Approval = "enabled"
	Disabled Approval = "disabled"
	NoStdin  Approval = "disabled_default"
)

type Client interface {
	io.Closer
	// Send a telemetry event. This method is not to be called directly. Use config.Track instead
	Track(event Event) error
}

type Event struct {
	Object     string
	Action     string
	Properties map[string]interface{}
}

type User struct {
	UniqueID string

	UserID string
}

// Create a telemetry client to be used to send telemetry event
func CreateClient(user User, enabled bool) Client {
	if !enabled {
		return nullClient{}
	}

	client := newSegmentClient()
	if err := client.identify(user); err != nil {
		return nullClient{}
	}

	return client
}

// Sends the user's approval event
func SendTelemetryApproval(user User, approval Approval) error {
	client := newSegmentClient()

	if approval == Enabled {
		if err := client.identify(user); err != nil {
			return err
		}
	}

	return client.Track(Event{
		Object: "cli-telemetry",
		Action: string(approval),
	})
}

// Null client
// Used when telemetry is disabled

type nullClient struct{}

func (cli nullClient) Close() error { return nil }

func (cli nullClient) Track(_ Event) error { return nil }

// Log telemetry
// Used for tests

type logClient struct{}

func (cli logClient) Close() error { return nil }

func (cli logClient) identify(_ User) error { return nil }

func (cli logClient) Track(e Event) error {
	fmt.Printf("\n*** Telemetry event ***\nObject: %s\nAction: %s\nProperties: %+v\n\n", e.Object, e.Action, e.Properties)
	return nil
}

// Segment client
// Used when telemetry is enabled

type segmentClient struct {
	cli analytics.Client
}

func newSegmentClient() logClient {
	return logClient{}
	// return &segmentClient{
	// 	cli: analytics.New(""),
	// }
}

func (segment *segmentClient) identify(user User) error {
	traits := analytics.NewTraits().Set("UUID", user.UniqueID)

	if user.UserID != "" {
		traits = traits.Set("userId", user.UserID)
	}
	return segment.cli.Enqueue(
		analytics.Identify{
			UserId: user.UniqueID,
		},
	)
}

func (segment *segmentClient) Track(event Event) error {
	event.Properties["action"] = event.Action
	return segment.cli.Enqueue(analytics.Track{
		Event:      event.Object,
		Properties: event.Properties,
	})
}

func (segment *segmentClient) Close() error {
	return segment.cli.Close()
}
