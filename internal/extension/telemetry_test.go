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

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/extschema"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
)

// recordingDestination keeps the events the sender receives.
type recordingDestination struct {
	mu     sync.Mutex
	tracks []analytics.Track
}

func (r *recordingDestination) Enqueue(track analytics.Track) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracks = append(r.tracks, track)
	return nil
}

func (r *recordingDestination) Close() error { return nil }

// recordedProps runs fn with a telemetry context and returns the properties of
// the command_invocation event, less the CLI's own command, flags, exit_code
// and app_name.
func recordedProps(t *testing.T, fn func(ctx context.Context)) map[string]any {
	t.Helper()

	recorder := &recordingDestination{}
	client, err := telemetry.NewSender(iostream.Testing(context.Background()), telemetry.Config{
		Send:            false,
		TestDestination: recorder,
		Metadata: telemetry.Meta{
			Version:    "1.2.3",
			InstanceID: uuid.New(),
			UserID:     uuid.New(),
			HostInfo:   &host.InfoStat{OS: "linux", KernelArch: "x86_64"},
		},
	})
	assert.NilError(t, err)

	cmd := &cobra.Command{Use: "ext"}
	cmd.SetContext(cmdutil.WithTelemetry(context.Background(), client))
	fn(cmd.Context())
	cmdutil.RecordTelemetryNow(cmd)
	assert.NilError(t, client.Close())

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	assert.Assert(t, cmp.Len(recorder.tracks, 1))
	props := map[string]any{}
	for k, v := range recorder.tracks[0].Properties {
		switch k {
		case "command", "flags", "exit_code", "app_name":
		default:
			props[k] = v
		}
	}
	return props
}

func attrs(t *testing.T, doc string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	err := json.Unmarshal([]byte(doc), &m)
	assert.NilError(t, err)
	return m
}

func TestAcceptedProperties(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ext     string
		version string
		attrs   string
		want    map[string]any
	}{
		{
			name:    "a name in the schema",
			ext:     "configoptimizer",
			version: "1.2.3",
			attrs:   `{"suggestions":3}`,
			want: map[string]any{
				"extension_version":               "1.2.3",
				"extension_attributes_status":     "accepted",
				"ext_configoptimizer_suggestions": 3,
			},
		},
		{
			name:    "a name not in the schema",
			ext:     "configoptimizer",
			version: "1.2.3",
			attrs:   `{"nope":3}`,
			want: map[string]any{
				"extension_version":           "1.2.3",
				"extension_attributes_status": "rejected",
			},
		},
		{
			name:    "a name not in the schema beside a valid one",
			ext:     "configoptimizer",
			version: "1.2.3",
			attrs:   `{"nope":3,"suggestions":0}`,
			want: map[string]any{
				"extension_version":               "1.2.3",
				"extension_attributes_status":     "partially_accepted",
				"ext_configoptimizer_suggestions": 0,
			},
		},
		{
			name:    "an extension with no schema",
			ext:     "other",
			version: "1.2.3",
			attrs:   `{"suggestions":3}`,
			want: map[string]any{
				"extension_version":           "1.2.3",
				"extension_attributes_status": "not_supported",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := extschema.Check(tc.ext, 1, attrs(t, tc.attrs))
			got := recordedProps(t, func(ctx context.Context) { record(ctx, tc.version, res) })
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}

	// An invalid version is left out and the status is still set.
	for _, tc := range []struct {
		version string
		valid   bool
	}{
		{
			version: "1.2.3",
			valid:   true,
		},
		{
			version: "1.2.3-rc.1",
			valid:   true,
		},
		{
			version: "1.2.3+build",
			valid:   true,
		},
		{
			version: "1.0",
			valid:   false,
		},
		{
			version: "v1",
			valid:   false,
		},
		{
			version: "v1.2.3",
			valid:   false,
		},
		{
			version: "1.2.3.4",
			valid:   false,
		},
		{
			version: "abc",
			valid:   false,
		},
		{
			version: "",
			valid:   false,
		},
	} {
		t.Run(fmt.Sprintf("version %q", tc.version), func(t *testing.T) {
			got := recordedProps(t, func(ctx context.Context) {
				record(ctx, tc.version, extschema.Result{Status: extschema.NotSupported})
			})
			want := map[string]any{"extension_attributes_status": "not_supported"}
			if tc.valid {
				want["extension_version"] = tc.version
			}
			assert.Check(t, cmp.DeepEqual(got, want))
		})
	}
}

// schemaDoc is the part of the exported schema these tests read.
type schemaDoc struct {
	Extensions []struct {
		Name       string `json:"name"`
		Version    int    `json:"version"`
		Attributes []struct {
			ShortName string   `json:"short_name"`
			Property  string   `json:"property"`
			Type      string   `json:"type"`
			Max       int      `json:"max"`
			Values    []string `json:"values"`
		} `json:"attributes"`
	} `json:"extensions"`
}

func exportedSchema(t *testing.T) schemaDoc {
	t.Helper()
	out, err := extschema.Export()
	assert.NilError(t, err)
	var doc schemaDoc
	err = json.Unmarshal(out, &doc)
	assert.NilError(t, err)
	assert.Assert(t, len(doc.Extensions) > 0)
	return doc
}

// TestPropertyNamesMatchSchema fails when the schema gains an attribute the
// helper has no case for, or names a property differently from the literal.
func TestPropertyNamesMatchSchema(t *testing.T) {
	for _, ext := range exportedSchema(t).Extensions {
		for _, a := range ext.Attributes {
			t.Run(ext.Name+"/"+a.ShortName, func(t *testing.T) {
				var value, doc string
				switch a.Type {
				case "int":
					value, doc = "0", "0"
				case "enum":
					value = a.Values[0]
					doc = `"` + value + `"`
				case "bool":
					value, doc = "true", "true"
				default:
					t.Fatalf("unknown attribute type %q", a.Type)
				}
				payload := `{"` + a.ShortName + `":` + doc + `}`
				if a.ShortName == "no_suggestion_reason" {
					// The schema keeps a reason only beside zero suggestions.
					payload = `{"suggestions":0,"` + a.ShortName + `":` + doc + `}`
				}

				res := extschema.Check(ext.Name, ext.Version, attrs(t, payload))
				assert.Assert(t, cmp.Equal(res.Status, extschema.Accepted), "payload %s", payload)
				got := recordedProps(t, func(ctx context.Context) { record(ctx, "1.2.3", res) })

				v, ok := got[a.Property]
				assert.Check(t, ok, "no property %q set; got %v", a.Property, got)
				if a.Type == "int" {
					assert.Check(t, cmp.Equal(v, 0))
				} else {
					assert.Check(t, cmp.Equal(v, value))
				}
			})
		}
	}
}

func TestSchemaPropertiesUseExtPrefix(t *testing.T) {
	for _, ext := range exportedSchema(t).Extensions {
		for _, a := range ext.Attributes {
			assert.Check(t, strings.HasPrefix(a.Property, "ext_"), "%s: property %q", ext.Name, a.Property)
		}
	}
}
