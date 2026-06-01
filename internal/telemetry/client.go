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

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
)

// SegmentKey is the Segment write key for CircleCI.
const SegmentKey = "AbgkrgN4cbRhAVEwlzMkHbwvrXnxHh35"

type Client struct {
	client analytics.Client
	meta   Meta
}

type Config struct {
	// Send enables sending events to Segment.
	Send bool
	// Log enables logging events to stderr.
	Log bool

	// WriteKey is the Segment write key, and if not provided, will disable telemtry.
	WriteKey string
	// Endpoint is the Segment endpoint, and is optional, defaulting to segment io.
	// This is normally only set for testing.
	Endpoint string
	// Specifies the number of events to batch together before sending. If zero the client will use a default.
	// This is likely only useful for testing.
	BatchSize int

	Metadata Meta
}

type Meta struct {
	IsSelfHosted bool
	Version      string

	InstanceID uuid.UUID
	UserID     uuid.UUID

	// HostInfo is the host info to associate with events.
	HostInfo *host.InfoStat
}

func (m *Meta) toContext() *analytics.Context {
	return &analytics.Context{
		App: analytics.AppInfo{
			Name:    "circleci-cli",
			Version: m.Version,
		},
		OS: analytics.OSInfo{
			Name:    m.HostInfo.OS,
			Version: m.HostInfo.PlatformVersion,
		},
		Device: analytics.DeviceInfo{
			Id:    m.InstanceID.String(),
			Model: m.HostInfo.KernelArch,
			Type:  m.HostInfo.PlatformFamily,
		},
		Extra: analytics.NewProperties().
			Set("is_self_hosted", m.IsSelfHosted),
	}
}

// New creates a new segment client
func New(ctx context.Context, cfg Config) (_ *Client, err error) {
	client := &multiClient{}

	if cfg.Log {
		client.Add(&loggingClient{
			ctx: ctx,
		})
	}

	if cfg.Send {
		if cfg.WriteKey == "" {
			return nil, errors.New("write key is required")
		}
		c, err := analytics.NewWithConfig(cfg.WriteKey, analytics.Config{
			Endpoint:  cfg.Endpoint,
			BatchSize: cfg.BatchSize,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create segment client: %w", err)
		}
		client.Add(c)
	}

	if cfg.Metadata.UserID == uuid.Nil {
		cfg.Metadata.UserID = AnonymousID
	}

	return &Client{
		client: client,
		meta:   cfg.Metadata,
	}, nil
}

func (c *Client) Identify() error {
	return c.client.Enqueue(analytics.Identify{
		UserId:       c.meta.UserID.String(),
		Context:      c.meta.toContext(),
		Integrations: analytics.NewIntegrations().Enable("Amplitude"),
	})
}

func (c *Client) Close() error {
	return c.client.Close()
}

// AnonymousID is hard-coded to a well-known value for unknown users.
// Callers should provide a real user id where possible.
var AnonymousID = uuid.MustParse("66f35d3e-40f6-4ade-909b-a6314990de53")

// Track sends an analytics event.
func (c *Client) Track(eventName string, props map[string]any) error {
	extras := analytics.NewProperties()
	for key, val := range props {
		extras.Set(key, val)
	}

	return c.client.Enqueue(analytics.Track{
		Event:      eventName,
		Timestamp:  time.Now(),
		Properties: extras,

		UserId:       c.meta.UserID.String(),
		Context:      c.meta.toContext(),
		Integrations: analytics.NewIntegrations().Enable("Amplitude"),
	})
}
