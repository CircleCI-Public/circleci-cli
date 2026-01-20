package info

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func TestGetOrgSuccess(t *testing.T) {
	id := "id"
	name := "name"

	// Test server
	var serverHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me/collaborations")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf(`[{"id": "%s", "name": "%s"}]`, id, name)))
		assert.NilError(t, err)
	}
	server := httptest.NewServer(serverHandler)
	defer server.Close()

	// Test command
	cmd, stdout, _ := scaffoldCMD(server.URL, defaultValidator)
	args := []string{
		"org",
	}
	cmd.SetArgs(args)

	// Execute
	err := cmd.Execute()

	// Asserts
	assert.NilError(t, err)
	assert.Equal(t, stdout.String(), `+----+------+
| ID | NAME |
+----+------+
| id | name |
+----+------+
`)
}

func TestGetOrgError(t *testing.T) {
	errorMessage := "server error message"

	// Test server
	var serverHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me/collaborations")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(fmt.Sprintf(`{"message": "%s"}`, errorMessage)))
		assert.NilError(t, err)
	}
	server := httptest.NewServer(serverHandler)
	defer server.Close()

	// Test command
	cmd, _, _ := scaffoldCMD(server.URL, defaultValidator)
	args := []string{
		"org",
	}
	cmd.SetArgs(args)

	// Execute
	err := cmd.Execute()

	// Asserts
	assert.Error(t, err, errorMessage)
}

func TestFailedValidator(t *testing.T) {
	errorMessage := "validator error"

	// Test server
	var serverHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me/collaborations")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(fmt.Sprintf(`{"message": "%s"}`, errorMessage)))
		assert.NilError(t, err)
	}
	server := httptest.NewServer(serverHandler)
	defer server.Close()

	// Test command
	cmd, _, _ := scaffoldCMD(server.URL, func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("%s", errorMessage)
	})
	args := []string{
		"org",
	}
	cmd.SetArgs(args)

	// Execute
	err := cmd.Execute()

	// Asserts
	assert.Error(t, err, errorMessage)
}

type testTelemetryClient struct {
	events []telemetry.Event
}

func (cli *testTelemetryClient) Track(event telemetry.Event) error {
	cli.events = append(cli.events, event)
	return nil
}

func (cli *testTelemetryClient) Enabled() bool { return true }

func (cli *testTelemetryClient) Close() error { return nil }

func TestTelemetry(t *testing.T) {
	telemetryClient := testTelemetryClient{make([]telemetry.Event, 0)}
	// Test server
	var serverHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"id", "name":"name"}]`))
	}
	server := httptest.NewServer(serverHandler)
	defer server.Close()

	// Test command
	config := &settings.Config{
		Token:      "testtoken",
		HTTPClient: http.DefaultClient,
		Host:       server.URL,
	}
	cmd := NewInfoCommand(config, nil)
	cmd.SetArgs([]string{"org"})
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(telemetry.NewContext(ctx, &telemetryClient))

	// Execute
	err := cmd.Execute()

	assert.NilError(t, err)

	// Read the telemetry events and compare them
	assert.DeepEqual(t, telemetryClient.events, []telemetry.Event{
		telemetry.CreateInfoEvent(telemetry.CommandInfo{
			Name:      "org",
			LocalArgs: map[string]string{"help": "false", "json": "false"},
		}, nil),
	})
}

func defaultValidator(cmd *cobra.Command, args []string) error {
	return nil
}

func scaffoldCMD(
	baseURL string,
	validator validator.Validator,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Token:               "testtoken",
		HTTPClient:          http.DefaultClient,
		Host:                baseURL,
		IsTelemetryDisabled: true,
	}
	cmd := NewInfoCommand(config, validator)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}
