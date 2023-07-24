package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/segmentio/analytics-go"
)

var (
	// Overwrite this function for tests
	CreateActiveTelemetry = newSegmentClient

	SegmentEndpoint = "http://localhost"
	segmentKey      = "AbgkrgN4cbRhAVEwlzMkHbwvrXnxHh35"
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

// A segment event to be sent to the telemetry
// Important: this is not meant to be constructed directly apart in tests
// If you want to create a new event, add its constructor in ./events.go
type Event struct {
	Object     string                 `json:"object"`
	Action     string                 `json:"action"`
	Properties map[string]interface{} `json:"properties"`
}

type User struct {
	UniqueID     string
	UserID       string
	IsSelfHosted bool
	OS           string
	Version      string
	TeamName     string
}

// Create a telemetry client to be used to send telemetry events
func CreateClient(user User, enabled bool) Client {
	if !enabled {
		return nullClient{}
	}

	return CreateActiveTelemetry(user)
}

// Sends the user's approval event
func SendTelemetryApproval(user User, approval Approval) error {
	client := CreateActiveTelemetry(user)
	defer client.Close()

	return client.Track(Event{
		Object: "cli-telemetry",
		Action: string(approval),
	})
}

// Null client
// Used when telemetry is disabled

func CreateNullClient() Client {
	return nullClient{}
}

type nullClient struct{}

func (cli nullClient) Close() error { return nil }

func (cli nullClient) Track(_ Event) error { return nil }

// Segment client
// Used when telemetry is enabled

// Nil segment logger
type nilSegmentEmptyLogger struct{}

func (nilSegmentEmptyLogger) Logf(format string, args ...interface{})   {}
func (nilSegmentEmptyLogger) Errorf(format string, args ...interface{}) {}

type segmentClient struct {
	analyticsClient analytics.Client
	user            User
}

func newSegmentClient(user User) Client {
	cli, err := analytics.NewWithConfig(segmentKey, analytics.Config{
		Endpoint: SegmentEndpoint,
		Logger:   nilSegmentEmptyLogger{},
	})

	if err != nil {
		return CreateNullClient()
	}

	if len(user.UniqueID) == 0 {
		user.UniqueID = "null"
	}

	err = cli.Enqueue(
		analytics.Identify{
			UserId: user.UniqueID,
			Traits: analytics.NewTraits().Set("os", user.OS),
		},
	)
	if err != nil {
		fmt.Printf("Error while identifying with telemetry: %s\n", err)
	}

	return &segmentClient{cli, user}
}

func (segment *segmentClient) Track(event Event) error {
	if event.Properties == nil {
		event.Properties = make(map[string]interface{})
	}
	if event.Action != "" {
		event.Properties["action"] = event.Action
	}

	if segment.user.UniqueID != "" {
		event.Properties["anonymous_id"] = segment.user.UniqueID
	}

	if segment.user.UserID != "" {
		event.Properties["cci_user_id"] = segment.user.UserID
	}

	event.Properties["is_self_hosted"] = segment.user.IsSelfHosted

	if segment.user.OS != "" {
		event.Properties["os"] = segment.user.OS
	}

	if segment.user.Version != "" {
		event.Properties["cli_version"] = segment.user.Version
	}

	if segment.user.TeamName != "" {
		event.Properties["team_name"] = segment.user.TeamName
	}

	return segment.analyticsClient.Enqueue(analytics.Track{
		UserId:     segment.user.UniqueID,
		Event:      event.Object,
		Properties: event.Properties,
	})
}

func (segment *segmentClient) Close() error {
	return segment.analyticsClient.Close()
}

// File telemetry
// Used for E2E tests

type fileTelemetry struct {
	filePath string
	events   []Event
}

func CreateFileTelemetry(filePath string) Client {
	return &fileTelemetry{filePath, make([]Event, 0)}
}

func (cli *fileTelemetry) Track(event Event) error {
	cli.events = append(cli.events, event)
	return nil
}

func (cli *fileTelemetry) Close() error {
	file, err := os.OpenFile(cli.filePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	content, err := json.Marshal(&cli.events)
	if err != nil {
		return err
	}

	if _, err = file.Write(content); err != nil {
		return err
	}

	return file.Close()
}
