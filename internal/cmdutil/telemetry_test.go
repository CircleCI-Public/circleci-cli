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
	"net/http/httptest"
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

		cmdutil.RecordTelemetry(cmd, client)
		assert.NilError(t, cmd.Flags().Set("bool-flag", "true"))
		assert.NilError(t, cmd.Flags().Set("string-flag", "string-value"))
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Batches(), []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
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
				},
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("is a no-op when original RunE is nil", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		cmd := &cobra.Command{Use: "test"}

		cmdutil.RecordTelemetry(cmd, client)

		assert.Check(t, cmp.Nil(cmd.RunE))
		assert.Check(t, cmp.Len(recorder.Batches(), 0))
	})

	t.Run("propagates error from original RunE", func(t *testing.T) {
		recorder, client := newTelemetry(t)
		expectedErr := fmt.Errorf("something went wrong")
		cmd := &cobra.Command{
			Use:  "fail",
			RunE: func(cmd *cobra.Command, args []string) error { return expectedErr },
		}

		cmdutil.RecordTelemetry(cmd, client)

		err := cmd.RunE(cmd, nil)
		assert.Check(t, cmp.ErrorIs(err, expectedErr))
		assert.Assert(t, cmp.ErrorIs(err, expectedErr))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Batches(), []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
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
				},
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

		cmdutil.RecordTelemetry(cmd, client)
		assert.NilError(t, cmd.Flags().Set("zebra", "true"))
		assert.NilError(t, cmd.Flags().Set("alpha", "true"))
		assert.NilError(t, cmd.Flags().Set("middle", "true"))
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Batches(), []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
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
				},
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

		cmdutil.RecordTelemetry(cmd, client)
		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Batches(), []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
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
				},
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
		cmdutil.RecordTelemetry(cmd, client)
		assert.NilError(t, client.Close())

		assert.NilError(t, cmd.RunE(cmd, nil))
		assert.Check(t, cmp.Len(recorder.Batches(), 0))
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

		cmdutil.RecordTelemetryForSubcommands(root, client)
		assert.NilError(t, child.RunE(child, nil))
		assert.NilError(t, client.Close())

		now := time.Now()
		assert.Check(t, cmp.DeepEqual(recorder.Batches(), []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
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
				},
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})

	t.Run("skips subcommands with nil RunE", func(t *testing.T) {
		recorder, client := newTelemetry(t)

		root := &cobra.Command{Use: "circleci"}
		child := &cobra.Command{Use: "help"} // no RunE
		root.AddCommand(child)

		cmdutil.RecordTelemetryForSubcommands(root, client)
		assert.NilError(t, client.Close())

		assert.Check(t, cmp.Nil(child.RunE))
		assert.Check(t, cmp.Len(recorder.Batches(), 0))
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

		cmdutil.RecordTelemetryForSubcommands(root, client)
		assert.NilError(t, child.RunE(child, nil))
		assert.NilError(t, client.Close())

		assert.Check(t, cmp.Len(recorder.Batches(), 0))
	})
}

func newTelemetry(t *testing.T) (*fakesegment.Service, *telemetry.Client) {
	t.Helper()

	ctx := iostream.Testing(context.Background())

	const goodAPIKey = "fc269e01-cf68-4244-ba14-55d040af0cd1"

	fs := fakesegment.New(ctx, goodAPIKey)
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)

	client, err := telemetry.New(ctx, telemetry.Config{
		Send:     true,
		Log:      true,
		WriteKey: goodAPIKey,
		Endpoint: srv.URL,
		Metadata: telemetry.Meta{
			IsSelfHosted: false,
			Version:      "1.2.3",
			InstanceID:   uuid.MustParse(instanceID),
			UserID:       uuid.MustParse(userID),
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

	return fs, client
}

const (
	userID     = "cb3ce909-79b7-4a12-baa4-ecb986047e37"
	instanceID = "536bbd16-bf6c-4d1c-bf2c-fc2bb474fb42"
)
