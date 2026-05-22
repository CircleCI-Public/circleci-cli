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
)

type Client struct {
	client analytics.Client
	user   User
}

type Mode int

const (
	// ModeNOOP disables telemetry.
	ModeNOOP Mode = iota
	// ModeSend sends events to Segment.
	ModeSend
	// ModeLog logs events to stderr.
	ModeLog
)

type Config struct {
	Mode Mode

	// WriteKey is the Segment write key, and if not provided, will disable telemtry.
	WriteKey string
	// Endpoint is the Segment endpoint, and is optional, defaulting to segment io.
	// This is normally only set for testing.
	Endpoint string
	// Specifies the number of events to batch together before sending. If zero the client will use a default.
	// This is likely only useful for testing.
	BatchSize int

	// User is the user to associate with events.
	User User
}

type User struct {
	// InstanceID allows manually specifying the client instance ID. Not meant to bs used in production, but
	// useful for deterministic tests.
	InstanceID uuid.UUID
	// UserID is the user ID to associate with events.
	UserID uuid.UUID

	IsSelfHosted bool
	OS           string
	Version      string
}

func (u User) toContext() *analytics.Context {
	return &analytics.Context{
		App: analytics.AppInfo{
			Name:    "circleci-cli",
			Version: u.Version,
		},
		OS: analytics.OSInfo{
			Name: u.OS,
		},
		Device: analytics.DeviceInfo{
			Id:           u.InstanceID.String(),
			Manufacturer: "CircleCI Ltd",
			Name:         "circleci-cli",
		},
	}
}

// New creates a new segment client
func New(ctx context.Context, cfg Config) (_ *Client, err error) {
	var client analytics.Client
	switch cfg.Mode {
	case ModeNOOP:
		client = &noopClient{}
	case ModeSend:
		if cfg.WriteKey == "" {
			return nil, errors.New("write key is required")
		}
		client, err = analytics.NewWithConfig(cfg.WriteKey, analytics.Config{
			Endpoint:  cfg.Endpoint,
			BatchSize: cfg.BatchSize,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create segment client: %w", err)
		}
	case ModeLog:
		client = &loggingClient{
			ctx: ctx,
		}
	}

	if cfg.User.InstanceID == uuid.Nil {
		cfg.User.InstanceID = uuid.New()
	}

	if cfg.User.UserID == uuid.Nil {
		cfg.User.UserID = AnonymousID
	}

	return &Client{
		client: client,
		user:   cfg.User,
	}, nil
}

func (c *Client) Identify() error {
	return c.client.Enqueue(analytics.Identify{
		UserId:       c.user.UserID.String(),
		Context:      c.user.toContext(),
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

		UserId:       c.user.UserID.String(),
		Context:      c.user.toContext(),
		Integrations: analytics.NewIntegrations().Enable("Amplitude"),
	})
}
