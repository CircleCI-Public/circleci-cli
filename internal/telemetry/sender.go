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
	"io"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
)

// SegmentKey is the Segment write key for CircleCI.
const SegmentKey = "AbgkrgN4cbRhAVEwlzMkHbwvrXnxHh35"

type Sender struct {
	dest destination
	meta Meta

	closed atomic.Bool
}

type destination interface {
	io.Closer
	Enqueue(track analytics.Track) error
}

type Config struct {
	// Send enables sending events to Segment.
	Send bool
	// Log enables logging events to stderr.
	Log bool

	// Binary will be executed with JSON-encoded events as the stdin.
	Binary string
	// WriteKey is the Segment write key for the CLI.
	WriteKey string
	// Endpoint is the Segment endpoint, and is optional, defaulting to segment io.
	// This is normally only set for testing.
	Endpoint string

	// TestDestination, when non-nil, is added as an event destination. It lets
	// tests record events and assert on them synchronously without spawning a
	// subprocess or hitting the network.
	TestDestination destination

	Metadata Meta
}

type Meta struct {
	Extra map[string]any

	Version string

	InstanceID uuid.UUID
	UserID     uuid.UUID

	// HostInfo is the host info to associate with events.
	HostInfo *host.InfoStat
}

func (m *Meta) toContext() *analytics.Context {
	var osInfo analytics.OSInfo
	device := analytics.DeviceInfo{Id: m.InstanceID.String()}
	// HostInfo is best-effort and may be nil when host detection fails
	// (e.g. gopsutil's ioreg lookup under a restricted PATH).
	if m.HostInfo != nil {
		osInfo = analytics.OSInfo{
			Name:    m.HostInfo.OS,
			Version: m.HostInfo.PlatformVersion,
		}
		device.Model = m.HostInfo.KernelArch
		device.Type = m.HostInfo.PlatformFamily
	}
	return &analytics.Context{
		App: analytics.AppInfo{
			Name:    "circleci-cli",
			Version: m.Version,
		},
		OS:     osInfo,
		Device: device,
		Traits: m.Extra,
	}
}

// NewSender creates a new telemetry sender
func NewSender(ctx context.Context, cfg Config) (_ *Sender, err error) {
	dest := &multiDestination{}

	if cfg.TestDestination != nil {
		dest.Add(cfg.TestDestination)
	}

	if cfg.Log {
		dest.Add(&loggingDestination{
			ctx: ctx,
		})
	}

	if cfg.Send {
		if cfg.Binary == "" {
			return nil, errors.New("binary is required")
		}

		dest.Add(&delegateDestination{
			bin:      cfg.Binary,
			writeKey: cfg.WriteKey,
			endpoint: cfg.Endpoint,
		})
	}

	if cfg.Metadata.UserID == uuid.Nil {
		cfg.Metadata.UserID = AnonymousID
	}

	return &Sender{
		dest: dest,
		meta: cfg.Metadata,
	}, nil
}

func (c *Sender) Close() error {
	if c == nil {
		return nil
	}

	closed := c.closed.Swap(true)
	if closed {
		return nil
	}

	return c.dest.Close()
}

// AnonymousID is hard-coded to a well-known value for unknown users.
// Callers should provide a real user id where possible.
var AnonymousID = uuid.MustParse("66f35d3e-40f6-4ade-909b-a6314990de53")

// Track sends an analytics event.
func (c *Sender) Track(eventName string, props map[string]any) error {
	if c == nil {
		return nil
	}

	p := analytics.NewProperties()
	for key, val := range props {
		p.Set(key, val)
	}

	return c.dest.Enqueue(analytics.Track{
		Event:      eventName,
		Timestamp:  time.Now(),
		Properties: p,

		UserId:       c.meta.UserID.String(),
		Context:      c.meta.toContext(),
		Integrations: analytics.NewIntegrations().Enable("Amplitude"),
	})
}
