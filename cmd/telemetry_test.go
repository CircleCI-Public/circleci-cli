package cmd

import (
	"path/filepath"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/cmd/create_telemetry"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
)

type telemetryTestAPIClient struct {
	id  string
	err error
}

func (me telemetryTestAPIClient) GetMyUserId() (string, error) {
	return me.id, me.err
}

func TestSetIsTelemetryActive(t *testing.T) {
	type args struct {
		apiClient create_telemetry.TelemetryAPIClient
		isActive  bool
		settings  *settings.TelemetrySettings
	}
	type want struct {
		settings *settings.TelemetrySettings
	}

	type testCase struct {
		name string
		args args
		want want
	}

	userId := "user-id"
	uniqueId := "unique-id"

	testCases := []testCase{
		{
			name: "Enabling telemetry with settings should just update the is active field",
			args: args{
				apiClient: telemetryTestAPIClient{},
				isActive:  true,
				settings: &settings.TelemetrySettings{
					IsEnabled:         false,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
			want: want{
				settings: &settings.TelemetrySettings{
					IsEnabled:         true,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
		},
		{
			name: "Enabling telemetry without settings should fill the settings fields",
			args: args{
				apiClient: telemetryTestAPIClient{id: userId, err: nil},
				isActive:  true,
				settings:  nil,
			},
			want: want{
				settings: &settings.TelemetrySettings{
					IsEnabled:         true,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
		},
		{
			name: "Disabling telemetry with settings should just update the is active field",
			args: args{
				apiClient: telemetryTestAPIClient{},
				isActive:  false,
				settings: &settings.TelemetrySettings{
					IsEnabled:         true,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
			want: want{
				settings: &settings.TelemetrySettings{
					IsEnabled:         false,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
		},
		{
			name: "Enabling telemetry without settings should fill the settings fields",
			args: args{
				apiClient: telemetryTestAPIClient{id: userId, err: nil},
				isActive:  false,
				settings:  nil,
			},
			want: want{
				settings: &settings.TelemetrySettings{
					IsEnabled:         false,
					HasAnsweredPrompt: true,
					UniqueID:          uniqueId,
					UserID:            userId,
				},
			},
		},
	}

	// Mock create UUID
	oldUUIDCreate := create_telemetry.CreateUUID
	create_telemetry.CreateUUID = func() string { return uniqueId }
	defer (func() { create_telemetry.CreateUUID = oldUUIDCreate })()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Mock FS
			oldFS := settings.FS.Fs
			settings.FS.Fs = afero.NewMemMapFs()
			defer (func() { settings.FS.Fs = oldFS })()

			if tt.args.settings != nil {
				err := tt.args.settings.Write()
				assert.NilError(t, err)
			}

			err := setIsTelemetryActive(tt.args.apiClient, tt.args.isActive)
			assert.NilError(t, err)

			exist, err := settings.FS.Exists(filepath.Join(settings.SettingsPath(), "telemetry.yml"))
			assert.NilError(t, err)
			if tt.want.settings == nil {
				assert.Equal(t, exist, false)
			} else {
				assert.Equal(t, exist, true)

				loadedSettings := &settings.TelemetrySettings{}
				err := loadedSettings.Load()
				assert.NilError(t, err)

				assert.DeepEqual(t, tt.want.settings, loadedSettings)
			}
		})
	}
}
