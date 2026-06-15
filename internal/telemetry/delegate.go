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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/segmentio/analytics-go/v3"

	"github.com/CircleCI-Public/circleci-cli/internal/telemetry/receiver"
)

type delegateDestination struct {
	bin      string
	writeKey string
	endpoint string

	mu       sync.RWMutex
	messages []analytics.Track
}

func (d *delegateDestination) Enqueue(message analytics.Track) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.messages = append(d.messages, message)
	return nil
}

func (d *delegateDestination) Close() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.messages) == 0 {
		return nil
	}

	buf, err := json.Marshal(d.messages)
	if err != nil {
		return err
	}

	_ = d.send(bytes.NewReader(buf))

	return nil
}

func (d *delegateDestination) send(in io.Reader) error {
	bin := d.bin
	if abs, err := filepath.Abs(bin); err == nil {
		bin = abs
	}

	//#nosec:G204 // This is the path of our own binary
	cmd := exec.Command(bin, "receive-telemetry")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = detachAttrs()
	cmd.Env = append(os.Environ(),
		receiver.EnvWriteKey+"="+d.writeKey,
		receiver.EnvTelemetryEndpoint+"="+d.endpoint,
	)
	if filepath.IsAbs(bin) {
		cmd.Dir = os.TempDir()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		_ = stdin.Close()
		return err
	}

	_, _ = io.Copy(stdin, in)
	_ = stdin.Close()

	return cmd.Process.Release()
}
