package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
)

type testTelemetry struct {
	events []telemetry.Event
}

func (cli *testTelemetry) Close() error { return nil }

func (cli *testTelemetry) Track(event telemetry.Event) error {
	cli.events = append(cli.events, event)
	return nil
}

func TestAskForTelemetryApproval(t *testing.T) {
	// Mock HTTP
	userId := "id"
	uniqueId := "unique-id"
	response := fmt.Sprintf(`{"id":"%s","login":"login","name":"name"}`, userId)
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	// Mock create UUID
	oldUUIDCreate := createUUID
	createUUID = func() string { return uniqueId }
	defer (func() { createUUID = oldUUIDCreate })()

	// Create test cases
	type args struct {
		closeStdin     bool
		promptApproval bool
		config         settings.TelemetrySettings
	}
	type want struct {
		config          settings.TelemetrySettings
		fileNotCreated  bool
		telemetryEvents []telemetry.Event
	}
	type testCase struct {
		name string
		args args
		want want
	}

	testCases := []testCase{
		{
			name: "Prompt approval should be saved in settings",
			args: args{
				promptApproval: true,
				config:         settings.TelemetrySettings{},
			},
			want: want{
				config: settings.TelemetrySettings{
					IsActive:          true,
					HasAnsweredPrompt: true,
					UserID:            userId,
					UniqueID:          uniqueId,
				},
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "enabled"},
				},
			},
		},
		{
			name: "Prompt disapproval should be saved in settings",
			args: args{
				promptApproval: false,
				config:         settings.TelemetrySettings{},
			},
			want: want{
				config: settings.TelemetrySettings{
					IsActive:          false,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
				},
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled"},
				},
			},
		},
		{
			name: "Does not recreate a unique ID if there is one",
			args: args{
				promptApproval: true,
				config: settings.TelemetrySettings{
					UniqueID: "other-id",
				},
			},
			want: want{
				config: settings.TelemetrySettings{
					IsActive:          true,
					HasAnsweredPrompt: true,
					UserID:            userId,
					UniqueID:          "other-id",
				},
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "enabled"},
				},
			},
		},
		{
			name: "Does not change telemetry settings if user already answered prompt",
			args: args{
				config: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
				},
			},
			want: want{
				fileNotCreated:  true,
				telemetryEvents: []telemetry.Event{},
			},
		},
		{
			name: "Does not change telemetry settings if user disabled telemetry",
			args: args{
				config: settings.TelemetrySettings{
					DisabledFromParams: true,
				},
			},
			want: want{
				fileNotCreated:  true,
				telemetryEvents: []telemetry.Event{},
			},
		},
		{
			name: "Does not change telemetry settings if stdin is not open",
			args: args{closeStdin: true},
			want: want{
				fileNotCreated: true,
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled_default"},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Mock FS
			oldFS := settings.FS.Fs
			settings.FS.Fs = afero.NewMemMapFs()
			defer (func() { settings.FS.Fs = oldFS })()

			// Mock stdin
			oldIsStdinOpen := isStdinOpen
			isStdinOpen = !tt.args.closeStdin
			defer (func() { isStdinOpen = oldIsStdinOpen })()

			// Mock telemetry
			telemetryClient := testTelemetry{events: make([]telemetry.Event, 0)}
			oldCreateActiveTelemetry := telemetry.CreateActiveTelemetry
			telemetry.CreateActiveTelemetry = func(_ telemetry.User) telemetry.Client {
				return &telemetryClient
			}
			defer (func() { telemetry.CreateActiveTelemetry = oldCreateActiveTelemetry })()

			// Run askForTelemetryApproval
			config := settings.Config{
				Token:      "testtoken",
				HTTPClient: http.DefaultClient,
				Host:       server.URL,
				Telemetry:  tt.args.config,
			}
			err := askForTelemetryApproval(&config, telemetryTestUI{tt.args.promptApproval})
			assert.NilError(t, err)

			// Verify good telemetry events were sent
			assert.DeepEqual(t, telemetryClient.events, tt.want.telemetryEvents)

			// Verify if settings file exist
			exist, err := settings.FS.Exists(filepath.Join(settings.SettingsPath(), "telemetry.yml"))
			assert.NilError(t, err)
			assert.Equal(t, exist, !tt.want.fileNotCreated)
			if tt.want.fileNotCreated {
				return
			}

			// Verify settings file content
			result := settings.TelemetrySettings{}
			err = result.Load()
			assert.NilError(t, err)
			assert.Equal(t, result, tt.want.config)
		})
	}
}
