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

package telemetry_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

const goodWriteKey = "b4b250188e5994cf45e7b0e5"

func TestClient_Track_with_user_id(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	fs := fakesegment.New(ctx, goodWriteKey)
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)

	userID := uuid.New()
	instanceID := uuid.New()
	ac, err := telemetry.New(ctx, telemetry.Config{
		Send:     true,
		Log:      true,
		Endpoint: srv.URL,
		WriteKey: goodWriteKey,
		Metadata: telemetry.Meta{
			InstanceID: instanceID,
			UserID:     userID,
			Version:    "1.2.3",
			Extra: map[string]any{
				"extra_a": "extra_value_a",
				"extra_b": "extra_value_b",
			},
			HostInfo: &host.InfoStat{
				Hostname:             "unused",
				OS:                   "darwin",
				Platform:             "darwin",
				PlatformFamily:       "Standalone Workstation",
				PlatformVersion:      "26.4.1",
				KernelVersion:        "unused",
				KernelArch:           "arm64",
				VirtualizationSystem: "unused",
				VirtualizationRole:   "unused",
				HostID:               "unused",
			},
		},
	})
	assert.NilError(t, err)

	err = ac.Identify()
	assert.NilError(t, err)

	err = ac.Track("myevent", map[string]any{
		"foo": "bar",
		"baz": 42,
	})
	assert.NilError(t, err)

	// Close will flush the events
	err = ac.Close()
	assert.NilError(t, err)

	batches := fs.Batches()
	now := time.Now()
	assert.Check(t, cmp.DeepEqual(batches, []fakesegment.Batch{
		{
			SentAt: now,
			Messages: []analytics.Track{
				{
					Type:      "identify",
					MessageId: "ignored",
					Timestamp: now,
					UserId:    userID.String(),
					Context: &analytics.Context{
						App: analytics.AppInfo{Name: "circleci-cli", Version: "1.2.3"},
						Device: analytics.DeviceInfo{
							Id:    instanceID.String(),
							Type:  "Standalone Workstation",
							Model: "arm64",
						},
						OS: analytics.OSInfo{
							Name:    "darwin",
							Version: "26.4.1",
						},
						Traits: map[string]any{
							"extra_a": "extra_value_a",
							"extra_b": "extra_value_b",
						},
					},
					Integrations: analytics.NewIntegrations().Enable("Amplitude"),
				},
				{
					Type:      "track",
					MessageId: "ignored",
					Timestamp: now,
					UserId:    userID.String(),
					Event:     "myevent",
					Properties: analytics.Properties{
						"foo": "bar",
						"baz": float64(42),
					},
					Context: &analytics.Context{
						App: analytics.AppInfo{Name: "circleci-cli", Version: "1.2.3"},
						Device: analytics.DeviceInfo{
							Id:    instanceID.String(),
							Type:  "Standalone Workstation",
							Model: "arm64",
						},
						OS: analytics.OSInfo{
							Name:    "darwin",
							Version: "26.4.1",
						},
						Traits: map[string]any{
							"extra_a": "extra_value_a",
							"extra_b": "extra_value_b",
						},
					},
					Integrations: analytics.NewIntegrations().Enable("Amplitude"),
				},
			},
		},
	}, fakesegment.CompareTrack, fakesegment.CompareTime))
}

func TestClient_Track_without_userid(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	fs := fakesegment.New(ctx, goodWriteKey)
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)

	instanceID := uuid.New()
	ac, err := telemetry.New(ctx, telemetry.Config{
		Send:     true,
		Log:      true,
		Endpoint: srv.URL,
		WriteKey: goodWriteKey,
		Metadata: telemetry.Meta{
			InstanceID: instanceID,
			Version:    "1.2.3",
			Extra: map[string]any{
				"extra_1": "extra_value_1",
				"extra_2": "extra_value_2",
			},
			HostInfo: &host.InfoStat{
				Hostname:             "unused",
				OS:                   "linux",
				Platform:             "ubuntu",
				PlatformFamily:       "debian",
				PlatformVersion:      "1.2.3",
				KernelVersion:        "unused",
				KernelArch:           "x86_64",
				VirtualizationSystem: "unused",
				VirtualizationRole:   "unused",
				HostID:               "unused",
			},
		},
	})
	assert.NilError(t, err)

	err = ac.Identify()
	assert.NilError(t, err)

	err = ac.Track("user-event", map[string]any{
		"foo": "bar",
		"baz": 84,
	})
	assert.NilError(t, err)

	// Teardown will flush the events
	err = ac.Close()
	assert.NilError(t, err)

	batches := fs.Batches()
	now := time.Now()
	assert.Check(t, cmp.DeepEqual(batches, []fakesegment.Batch{
		{
			SentAt: now,
			Messages: []analytics.Track{
				{
					Type:      "identify",
					MessageId: "ignored",
					Timestamp: now,
					UserId:    telemetry.AnonymousID.String(),
					Context: &analytics.Context{
						App: analytics.AppInfo{Name: "circleci-cli", Version: "1.2.3"},
						Device: analytics.DeviceInfo{
							Id:    instanceID.String(),
							Type:  "debian",
							Model: "x86_64",
						},
						OS: analytics.OSInfo{
							Name:    "linux",
							Version: "1.2.3",
						},
						Traits: map[string]any{
							"extra_1": "extra_value_1",
							"extra_2": "extra_value_2",
						},
					},
					Integrations: analytics.NewIntegrations().Enable("Amplitude"),
				},
				{
					Type:      "track",
					MessageId: "ignored",
					Timestamp: now,
					UserId:    telemetry.AnonymousID.String(),
					Event:     "user-event",
					Properties: analytics.Properties{
						"foo": "bar",
						"baz": float64(84),
					},
					Context: &analytics.Context{
						App: analytics.AppInfo{Name: "circleci-cli", Version: "1.2.3"},
						Device: analytics.DeviceInfo{
							Id:    instanceID.String(),
							Type:  "debian",
							Model: "x86_64",
						},
						OS: analytics.OSInfo{
							Name:    "linux",
							Version: "1.2.3",
						},
						Traits: map[string]any{
							"extra_1": "extra_value_1",
							"extra_2": "extra_value_2",
						},
					},
					Integrations: analytics.NewIntegrations().Enable("Amplitude"),
				},
			},
		},
	}, fakesegment.CompareTrack, fakesegment.CompareTime))
}
