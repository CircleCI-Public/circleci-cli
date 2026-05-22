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

	"github.com/segmentio/analytics-go/v3"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

type loggingClient struct {
	ctx context.Context
}

func (l *loggingClient) Close() error {
	return nil
}

func (l *loggingClient) Enqueue(m analytics.Message) error {
	switch m := m.(type) {
	case analytics.Track:
		msg := "track " + m.Event
		args := make([]any, 0, 2+2*len(m.Properties))
		for k, v := range m.Properties {
			args = append(args, k, v)
		}
		args = append(args, "kind", "telemetry")
		iostream.DebugContext(l.ctx, msg, args...)
	case analytics.Identify:
		msg := "identify"
		iostream.DebugContext(l.ctx, msg,
			"kind", "telemetry",
		)
	}

	return nil
}
