package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/segmentio/analytics-go"
)

type Approval string

var (
	// Overwrite this function for tests
	CreateActiveTelemetry = newSegmentClient
)

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
	fmt.Printf("telemetry enabled = %+v\n", enabled)
	if !enabled {
		return nullClient{}
	}

	return CreateActiveTelemetry(user)
}

// Sends the user's approval event
func SendTelemetryApproval(user User, approval Approval) error {
	client := CreateActiveTelemetry(user)

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

// Segment client
// Used when telemetry is enabled

type segmentClient struct {
	cli  analytics.Client
	user User
}

const (
	segmentKey = ""
)

func newSegmentClient(user User) Client {
	cli := analytics.New(segmentKey)

	userID := user.UniqueID
	if userID == "" {
		userID = "none"
	}

	err := cli.Enqueue(
		analytics.Identify{
			UserId: userID,
			Traits: analytics.NewTraits().Set("os", user.OS),
		},
	)
	fmt.Printf("Error while identifying with telemetry: %s\n", err)

	return &segmentClient{cli, user}
}

func (segment *segmentClient) Track(event Event) error {
	if event.Properties == nil {
		event.Properties = make(map[string]interface{})
	}
	event.Properties["action"] = event.Action

	if segment.user.UniqueID != "" {
		event.Properties["UUID"] = segment.user.UniqueID
	}

	if segment.user.UserID != "" {
		event.Properties["user_id"] = segment.user.UserID
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

	return segment.cli.Enqueue(analytics.Track{
		UserId:     segment.user.UniqueID,
		Event:      event.Object,
		Properties: event.Properties,
	})
}

func (segment *segmentClient) Close() error {
	return segment.cli.Close()
}

// File telemetry
// Used for E2E tests

type fileTelemetry struct {
	file *os.File
}

func CreateFileTelemetry(filePath string) Client {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	return &fileTelemetry{file}
}

func (cli *fileTelemetry) Track(event Event) error {
	content, err := json.Marshal(&event)
	if err != nil {
		return err
	}

	content = append(content, '\n')
	_, err = cli.file.Write(content)

	return err
}

func (cli *fileTelemetry) Close() error {
	file := cli.file
	cli.file = nil
	return file.Close()
}
