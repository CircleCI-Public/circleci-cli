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

package acceptance_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/analytics-go/v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

func TestSettingsListJSON_Defaults(t *testing.T) {
	env := testenv.New(t)
	env.Telemetry = true

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["token_set"], false))
	assert.Check(t, cmp.Equal(out["host"], "https://circleci.com"))
	assert.Check(t, cmp.Equal(out["telemetry"], true))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_WithToken(t *testing.T) {
	env := testenv.New(t)
	env.Telemetry = true
	env.Token = "testtoken123"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["token_set"], true))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_WithCustomHost(t *testing.T) {
	env := testenv.New(t)
	env.Telemetry = true
	dir := t.TempDir()

	set := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "set", "host", "https://circleci.example.com"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, set.ExitCode, 0, "stderr: %s", set.Stderr)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["host"], "https://circleci.example.com"))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_TelemetryEnvVarOverride(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_NO_TELEMETRY"] = "1"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["telemetry"], false))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsList_TextOutput(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	env := testenv.New(t)
	env.Telemetry = true

	fs := fakesegment.New(ctx, telemetry.SegmentKey)
	fsSrv := httptest.NewServer(fs)
	t.Cleanup(fsSrv.Close)

	env.Extra["CIRCLECI_TELEMETRY_ENDPOINT"] = fsSrv.URL

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// Strip the path line since it contains a temp directory that changes each run.
	var stable []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if !strings.HasPrefix(line, "- Path:") {
			stable = append(stable, line)
		}
	}
	assert.Check(t, golden.String(strings.Join(stable, "\n"), t.Name()+".txt"))

	t.Run("telemetry", func(t *testing.T) {
		cfg, err := config.LoadFrom(ctx, filepath.Join(env.ConfigDir(), "circleci", "config.yml"), false)
		assert.NilError(t, err)

		batches := fs.Batches()
		now := time.Now()
		assert.Check(t, cmp.DeepEqual(batches, []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
						Timestamp: now,
						UserId:    telemetry.AnonymousID.String(),
						Event:     "command_invocation",
						Properties: analytics.Properties{
							"command": "circleci settings list",
							"flags":   "debug,insecure-storage,theme",
						},
						Context: &analytics.Context{
							App: analytics.AppInfo{Name: "circleci-cli", Version: "dev"},
							Device: analytics.DeviceInfo{
								Id:           cfg.DeviceID().String(),
								Manufacturer: "CircleCI Ltd",
								Name:         "circleci-cli",
							},
							OS: analytics.OSInfo{Name: runtime.GOOS},
						},
						Integrations: analytics.NewIntegrations().Enable("Amplitude"),
					},
				},
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})
}
