package info

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
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
		return fmt.Errorf(errorMessage)
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

func TestTelemetry(t *testing.T) {
	tempSettings := clitest.WithTempSettings()
	defer tempSettings.Close()

	// Test server
	var serverHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"id", "name":"name"}]`))
	}
	server := httptest.NewServer(serverHandler)
	defer server.Close()

	// Test command
	config := &settings.Config{
		Token:         "testtoken",
		HTTPClient:    http.DefaultClient,
		Host:          server.URL,
		MockTelemetry: tempSettings.TelemetryDestPath,
	}
	cmd := NewInfoCommand(config, nil)

	// Execute
	err := cmd.Execute()

	assert.NilError(t, err)
	// We compare the
	content, err := os.ReadFile(tempSettings.TelemetryDestPath)
	assert.NilError(t, err)

	result := []telemetry.Event{}
	err = json.Unmarshal(content, &result)
	assert.NilError(t, err)
	assert.DeepEqual(t, result, []telemetry.Event{
		telemetry.CreateInfoEvent(telemetry.CommandInfo{
			Name:      "org",
			LocalArgs: map[string]string{"help": "false"},
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
