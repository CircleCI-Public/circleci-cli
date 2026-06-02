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

package config

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsTelemetryEnabled(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name             string
		storedPreference *bool
		envVar           string
		want             bool
	}{
		{name: "default enabled when no preference set", storedPreference: nil, want: true},
		{name: "stored true respected", storedPreference: boolPtr(true), want: true},
		{name: "stored false respected", storedPreference: boolPtr(false), want: false},
		{name: "env var overrides nil preference", storedPreference: nil, envVar: "CIRCLE_NO_TELEMETRY", want: false},
		{name: "env var overrides stored true", storedPreference: boolPtr(true), envVar: "CIRCLE_NO_TELEMETRY", want: false},
		{name: "env var overrides stored false", storedPreference: boolPtr(false), envVar: "CIRCLE_NO_TELEMETRY", want: false},
		{name: "NO_ANALYTICS disables telemetry", storedPreference: boolPtr(true), envVar: "NO_ANALYTICS", want: false},
		{name: "DO_NOT_TRACK disables telemetry", storedPreference: boolPtr(true), envVar: "DO_NOT_TRACK", want: false},
		{name: "CI disables telemetry", storedPreference: boolPtr(true), envVar: "CI", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, env := range noTelemetryEnvVars {
				t.Setenv(env, "")
			}
			if tc.envVar != "" {
				t.Setenv(tc.envVar, "1")
			}

			cfg := &Config{state: state{Telemetry: tc.storedPreference}}
			assert.Equal(t, cfg.IsTelemetry(), tc.want)
		})
	}
}
