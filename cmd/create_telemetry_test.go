package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func TestLoadTelemetrySettings(t *testing.T) {
	// Delete env.CI variable
	oldEnvCIValue := os.Getenv("CI")
	os.Setenv("CI", "")
	defer os.Setenv("CI", oldEnvCIValue)

	// Mock HTTP
	userId := "id"
	uniqueId := "unique-id"

	// Mock create UUID
	oldUUIDCreate := CreateUUID
	CreateUUID = func() string { return uniqueId }
	defer (func() { CreateUUID = oldUUIDCreate })()

	// Create test cases
	type args struct {
		closeStdin     bool
		closeStdout    bool
		env            map[string]string
		promptApproval bool
		settings       settings.TelemetrySettings
	}
	type want struct {
		settings        settings.TelemetrySettings
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
				settings:       settings.TelemetrySettings{},
			},
			want: want{
				settings: settings.TelemetrySettings{
					IsEnabled:         true,
					HasAnsweredPrompt: true,
					UserID:            userId,
					UniqueID:          uniqueId,
				},
				telemetryEvents: []telemetry.Event{
					{
						Object: "cli-telemetry", Action: "enabled",
						Properties: map[string]interface{}{
							"UUID":           uniqueId,
							"user_id":        userId,
							"is_self_hosted": false,
						},
					},
				},
			},
		},
		{
			name: "Prompt disapproval should be saved in settings",
			args: args{
				promptApproval: false,
				settings:       settings.TelemetrySettings{},
			},
			want: want{
				settings: settings.TelemetrySettings{
					IsEnabled:         false,
					HasAnsweredPrompt: true,
				},
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled", Properties: map[string]interface{}{
						"UUID":           "cli-anonymous-telemetry",
						"is_self_hosted": false,
					}},
				},
			},
		},
		{
			name: "Does not recreate a unique ID if there is one",
			args: args{
				promptApproval: true,
				settings: settings.TelemetrySettings{
					UniqueID: "other-id",
				},
			},
			want: want{
				settings: settings.TelemetrySettings{
					IsEnabled:         true,
					HasAnsweredPrompt: true,
					UserID:            userId,
					UniqueID:          "other-id",
				},
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "enabled", Properties: map[string]interface{}{
						"UUID":           "other-id",
						"user_id":        userId,
						"is_self_hosted": false,
					}},
				},
			},
		},
		{
			name: "Does not change telemetry settings if user already answered prompt",
			args: args{
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
				},
			},
			want: want{
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
				},
				fileNotCreated: true,
			},
		},
		{
			name: "Does not change telemetry settings if stdin is not a TTY",
			args: args{closeStdin: true},
			want: want{
				fileNotCreated: true,
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled_default", Properties: map[string]interface{}{
						"UUID":           "cli-anonymous-telemetry",
						"is_self_hosted": false,
					}},
				},
			},
		},
		{
			name: "Does not change telemetry settings if stdout is not a TTY",
			args: args{closeStdout: true},
			want: want{
				fileNotCreated: true,
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled_default", Properties: map[string]interface{}{
						"UUID":           "cli-anonymous-telemetry",
						"is_self_hosted": false,
					}},
				},
			},
		},
		{
			name: "Does not change telemetry settings if env.CI == true",
			args: args{env: map[string]string{"CI": "true"}},
			want: want{
				fileNotCreated: true,
				telemetryEvents: []telemetry.Event{
					{Object: "cli-telemetry", Action: "disabled_default", Properties: map[string]interface{}{
						"UUID":           "cli-anonymous-telemetry",
						"is_self_hosted": false,
					}},
				},
			},
		},
		{
			name: "Should try loading user id if user already answered prompt but has no user id",
			args: args{
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
					IsEnabled:         true,
				},
			},
			want: want{
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
					IsEnabled:         true,
					UserID:            userId,
				},
			},
		},
		{
			name: "Should consider has_answered_prompt before disabled-default scenarios",
			args: args{
				env: map[string]string{"CI": "true"},
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
					IsEnabled:         true,
				},
			},
			want: want{
				settings: settings.TelemetrySettings{
					HasAnsweredPrompt: true,
					IsEnabled:         true,
					UserID:            userId,
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
			oldIsStdinOpen := isStdinATTY
			isStdinATTY = !tt.args.closeStdin
			defer (func() { isStdinATTY = oldIsStdinOpen })()

			// Mock stdout
			oldIsStdoutOpen := isStdoutATTY
			isStdoutATTY = !tt.args.closeStdout
			defer (func() { isStdoutATTY = oldIsStdoutOpen })()

			// Mock telemetry
			telemetryClient := testTelemetry{events: make([]telemetry.Event, 0)}
			oldCreateActiveTelemetry := telemetry.CreateActiveTelemetry
			telemetry.CreateActiveTelemetry = func(user telemetry.User) telemetry.Client {
				telemetryClient.User = user
				return &telemetryClient
			}
			defer (func() { telemetry.CreateActiveTelemetry = oldCreateActiveTelemetry })()

			// Mock env
			if tt.args.env != nil {
				for k, v := range tt.args.env {
					oldEnvValue := os.Getenv(k)
					os.Setenv(k, v)
					defer os.Setenv(k, oldEnvValue)
				}
			}

			// Run tested function
			loadTelemetrySettings(&tt.args.settings, &telemetry.User{}, telemetryTestAPIClient{userId, nil}, telemetryTestUI{tt.args.promptApproval})
			assert.DeepEqual(t, &tt.args.settings, &tt.want.settings)

			// Verify good telemetry events were sent
			expectedEvents := tt.want.telemetryEvents
			if expectedEvents == nil {
				expectedEvents = []telemetry.Event{}
			}
			assert.DeepEqual(t, telemetryClient.events, expectedEvents)

			// Verify if settings file exist
			exist, err := settings.FS.Exists(filepath.Join(settings.SettingsPath(), "telemetry.yml"))
			assert.NilError(t, err)
			assert.Equal(t, exist, !tt.want.fileNotCreated)
			if tt.want.fileNotCreated {
				return
			}

			// Verify settings file content
			loaded := settings.TelemetrySettings{}
			err = loaded.Load()
			assert.NilError(t, err)
			assert.DeepEqual(t, &loaded, &tt.want.settings)
		})
	}
}

type testTelemetry struct {
	events []telemetry.Event
	User   telemetry.User
}

func (cli *testTelemetry) Enabled() bool { return true }

func (cli *testTelemetry) Close() error { return nil }

func (cli *testTelemetry) Track(event telemetry.Event) error {
	newEvent := event
	properties := map[string]interface{}{}
	if cli.User.UniqueID != "" {
		properties["UUID"] = cli.User.UniqueID
	}

	if cli.User.UserID != "" {
		properties["user_id"] = cli.User.UserID
	}

	properties["is_self_hosted"] = cli.User.IsSelfHosted

	if cli.User.OS != "" {
		properties["os"] = cli.User.OS
	}

	if cli.User.Version != "" {
		properties["cli_version"] = cli.User.Version
	}

	if cli.User.TeamName != "" {
		properties["team_name"] = cli.User.TeamName
	}

	if len(properties) > 0 {
		newEvent.Properties = properties
	}

	cli.events = append(cli.events, newEvent)
	return nil
}

type telemetryTestUI struct {
	Approved bool
}

func (ui telemetryTestUI) AskUserToApproveTelemetry(message string) bool {
	return ui.Approved
}

type telemetryTestAPIClient struct {
	id  string
	err error
}

func (me telemetryTestAPIClient) GetMyUserId() (string, error) {
	return me.id, me.err
}
