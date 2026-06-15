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

package receiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/segmentio/analytics-go/v3"
)

const (
	// EnvWriteKey configures the write key for the telemetry client.
	EnvWriteKey = "__CIRCLE_TELEMETRY_WRITE_KEY"
	// EnvTelemetryEndpoint configures the endpoint for the telemetry client.
	EnvTelemetryEndpoint = "__CIRCLE_TELEMETRY_ENDPOINT"
)

func Receive(in io.Reader) (err error) {
	writeKey := os.Getenv(EnvWriteKey)
	endpoint := os.Getenv(EnvTelemetryEndpoint)

	var messages []analytics.Track
	err = json.NewDecoder(in).Decode(&messages)
	if err != nil {
		return err
	}

	if writeKey == "" {
		return errors.New("write key is required")
	}
	c, err := analytics.NewWithConfig(writeKey, analytics.Config{
		Endpoint: endpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create segment client: %w", err)
	}

	defer func() {
		cerr := c.Close()
		if err == nil {
			err = cerr
		}
	}()

	for _, m := range messages {
		err := c.Enqueue(m)
		if err != nil {
			return err
		}
	}

	return nil
}
