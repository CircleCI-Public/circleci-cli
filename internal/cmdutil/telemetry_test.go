// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package cmdutil_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

func TestRecordTelemetry(t *testing.T) {
	t.Run("records command path and flags", func(t *testing.T) {
		recorder, client := newTelemetry(t)

		cmd := &cobra.Command{
			Use:  "list",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		cmd.Flags().Bool("bool-flag", false, "")
		cmd.Flags().String("string-flag", "", "")

		parent := &cobra.Command{Use: "banana"}
		root := &cobra.Command{Use: "circleci"}
		root.AddCommand(parent)
		parent.AddCommand(cmd)

		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetry(cmd)
		assert.NilError(t, cmd.Flags().Set("bool-flag", "true"))
		assert.NilError(t, cmd.Flags().Set("string-flag", "string-value"))
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command": "circleci banana list",
					"flags":   "bool-flag,string-flag",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("is a no-op when original RunE is nil", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{Use: "test"}
		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))
		cmdutil.RecordTelemetry(cmd)

		assert.Check(t, cmp.Nil(cmd.RunE))
		assert.Check(t, cmp.Len(recorder.Tracks(), 0))
	})

	t.Run("propagates error from original RunE", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		expectedErr := fmt.Errorf("something went wrong")
		cmd := &cobra.Command{
			Use:  "fail",
			RunE: func(cmd *cobra.Command, args []string) error { return expectedErr },
		}

		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetry(cmd)

		err := cmd.RunE(cmd, nil)
		assert.Check(t, cmp.ErrorIs(err, expectedErr))
		assert.Assert(t, cmp.ErrorIs(err, expectedErr))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command": "fail",
					"flags":   "",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("flags are sorted alphabetically", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{
			Use:  "test",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		cmd.Flags().Bool("zebra", false, "")
		cmd.Flags().Bool("alpha", false, "")
		cmd.Flags().Bool("middle", false, "")

		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetry(cmd)
		assert.NilError(t, cmd.Flags().Set("zebra", "true"))
		assert.NilError(t, cmd.Flags().Set("alpha", "true"))
		assert.NilError(t, cmd.Flags().Set("middle", "true"))
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command": "test",
					"flags":   "alpha,middle,zebra",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("no flags set records empty flags string", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{
			Use:  "test",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		cmd.Flags().Bool("unused", false, "")

		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetry(cmd)
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command": "test",
					"flags":   "",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("skips commands with telemetry disabled", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{
			Use:  "internal",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		cmdutil.DisableTelemetry(cmd)
		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))
		cmdutil.RecordTelemetry(cmd)
		assert.NilError(t, client.Close())

		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.Check(t, cmp.Len(recorder.Tracks(), 0))
	})

	t.Run("includes extra props set via SetTelemetryProp", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{
			Use: "api",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmdutil.SetTelemetryProp(cmd, "api_path", "api/v2/me")
				return nil
			},
		}
		cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))
		cmdutil.RecordTelemetry(cmd)
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command":  "api",
					"flags":    "",
					"api_path": "api/v2/me",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})
}

func TestRecordTelemetryForSubcommands(t *testing.T) {
	t.Run("instruments nested subcommands", func(t *testing.T) {
		recorder, client := newTelemetry(t)

		root := &cobra.Command{Use: "circleci"}
		parent := &cobra.Command{Use: "subcommand"}
		child := &cobra.Command{
			Use:  "list",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		root.AddCommand(parent)
		parent.AddCommand(child)

		root.SetContext(cmdutil.WithTelemetry(context.Background(), client))
		parent.SetContext(root.Context())
		child.SetContext(root.Context())

		cmdutil.RecordTelemetryForSubcommands(root)
		assert.NilError(t, child.RunE(child, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Tracks(), []analytics.Track{
			{
				Timestamp: now,
				UserId:    userID,
				Event:     "command_invocation",
				Properties: analytics.Properties{
					"command": "circleci subcommand list",
					"flags":   "",
				},
				Context: &analytics.Context{
					App: analytics.AppInfo{
						Name:    "circleci-cli",
						Version: "1.2.3",
					},
					Device: analytics.DeviceInfo{
						Id:    instanceID,
						Model: "x86_64",
						Type:  "debian",
					},
					OS: analytics.OSInfo{Name: "linux", Version: "24.04"},
				},
				Integrations: analytics.NewIntegrations().Enable("Amplitude"),
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("skips subcommands with nil RunE", func(t *testing.T) {
		recorder, client := newTelemetry(t)

		root := &cobra.Command{Use: "circleci"}
		child := &cobra.Command{Use: "help"} // no RunE
		root.AddCommand(child)
		root.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetryForSubcommands(root)
		assert.NilError(t, client.Close())

		assert.Check(t, cmp.Nil(child.RunE))
		assert.Check(t, cmp.Len(recorder.Tracks(), 0))
	})

	t.Run("skips subcommands with telemetry disabled", func(t *testing.T) {
		recorder, client := newTelemetry(t)

		root := &cobra.Command{Use: "circleci"}
		child := &cobra.Command{
			Use:  "send-telemetry",
			RunE: func(cmd *cobra.Command, args []string) error { return nil },
		}
		cmdutil.DisableTelemetry(child)
		root.AddCommand(child)

		root.SetContext(cmdutil.WithTelemetry(context.Background(), client))

		cmdutil.RecordTelemetryForSubcommands(root)
		assert.NilError(t, child.RunE(child, nil))
		assert.NilError(t, client.Close())

		assert.Check(t, cmp.Len(recorder.Tracks(), 0))
	})
}

// recordingDestination is a telemetry destination that records the events it
// receives so tests can assert on them synchronously, without spawning the
// out-of-process sender or hitting the network.
type recordingDestination struct {
	mu     sync.Mutex
	tracks []analytics.Track
}

func (r *recordingDestination) Enqueue(track analytics.Track) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracks = append(r.tracks, track)
	return nil
}

func (r *recordingDestination) Close() error { return nil }

func (r *recordingDestination) Tracks() []analytics.Track {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.tracks)
}

func newTelemetry(t *testing.T) (*recordingDestination, *telemetry.Sender) {
	t.Helper()

	ctx := iostream.Testing(context.Background())

	recorder := &recordingDestination{}

	client, err := telemetry.NewSender(ctx, telemetry.Config{
		Send:            false,
		Log:             true,
		TestDestination: recorder,
		Metadata: telemetry.Meta{
			Version:    "1.2.3",
			InstanceID: uuid.MustParse(instanceID),
			UserID:     uuid.MustParse(userID),
			HostInfo: &host.InfoStat{
				OS:              "linux",
				Platform:        "ubuntu",
				PlatformFamily:  "debian",
				PlatformVersion: "24.04",
				KernelVersion:   "7.0.3",
				KernelArch:      "x86_64",
			},
		},
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	return recorder, client
}

const (
	userID     = "cb3ce909-79b7-4a12-baa4-ecb986047e37"
	instanceID = "536bbd16-bf6c-4d1c-bf2c-fc2bb474fb42"
)
