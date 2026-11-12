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

package configaudit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// loadSARIFSchema reads the official SARIF 2.1.0 schema and resolves it.
// The file declares JSON Schema draft-04, which jsonschema-go does not accept;
// rewriting $schema to draft-07 and the top-level "id" to "$id" is enough for
// this file, which uses no exclusiveMinimum/exclusiveMaximum (the draft-04
// keywords whose meaning changed) and whose other "id"s are property names.
func loadSARIFSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "sarif-schema-2.1.0.json"))
	assert.NilError(t, err)
	s := strings.Replace(string(b), "http://json-schema.org/draft-04/schema#", "http://json-schema.org/draft-07/schema#", 1)
	s = strings.Replace(s, `"id": "https://docs.oasis-open.org/sarif/`, `"$id": "https://docs.oasis-open.org/sarif/`, 1)
	var schema jsonschema.Schema
	err = json.Unmarshal([]byte(s), &schema)
	assert.NilError(t, err)
	rs, err := schema.Resolve(nil)
	assert.NilError(t, err)
	return rs
}

// sampleReport has one positioned finding, one suppressed finding, one
// unpositioned finding and one warning.
func sampleReport() *Report {
	return &Report{
		File: ".circleci/config.yml",
		Findings: []Finding{
			{
				RuleID:     "unpinned-orb",
				Severity:   SeverityMedium,
				Confidence: ConfidenceHigh,
				Message:    "orb circleci/node@5 is a major-version alias",
				Fix:        "pin the orb to an exact version (name@X.Y.Z)",
				File:       ".circleci/config.yml",
				Position:   &Position{Line: 2, Column: 9, EndLine: 2, EndColumn: 24},
				Snippet:    "  node: circleci/node@5",
			},
			{
				RuleID:     "fork-pr-secrets",
				Severity:   SeverityHigh,
				Confidence: ConfidenceHigh,
				Message:    "fork pull requests can read project secrets",
				Fix:        "turn off passing secrets to fork pull requests",
				Location:   "project settings",
			},
		},
		Suppressed: []Finding{
			{
				RuleID:     "unpinned-image",
				Severity:   SeverityHigh,
				Confidence: ConfidenceHigh,
				Message:    "image cimg/base:latest uses the latest tag",
				Fix:        "pin the image to a digest (image:tag@sha256:<digest>)",
				File:       ".circleci/config.yml",
				Position:   &Position{Line: 7, Column: 16, EndLine: 7, EndColumn: 32},
				Snippet:    "      - image: cimg/base:latest",
			},
		},
		Warnings: []string{"could not read project settings: no token; offline results only"},
	}
}

func TestRenderSARIF_ValidatesAgainstSchema(t *testing.T) {
	rs := loadSARIFSchema(t)
	tests := []struct {
		name   string
		report *Report
	}{
		{
			name:   "empty",
			report: &Report{File: ".circleci/config.yml", Warnings: []string{}},
		},
		{
			name:   "findings, suppressed and a warning",
			report: sampleReport(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderSARIF(&buf, tc.report, "1.2.3")
			assert.NilError(t, err)
			var doc any
			err = json.Unmarshal(buf.Bytes(), &doc)
			assert.NilError(t, err)
			err = rs.Validate(doc)
			assert.Check(t, err)
		})
	}
	t.Run("schema rejects a bad log", func(t *testing.T) {
		err := rs.Validate(map[string]any{"version": "9", "runs": []any{}})
		assert.Check(t, err != nil, "a log with version 9 must not validate")
	})
}

func TestRenderSARIF_Content(t *testing.T) {
	var buf bytes.Buffer
	err := RenderSARIF(&buf, sampleReport(), "1.2.3")
	assert.NilError(t, err)
	var log struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Rules   []struct {
						ID                   string `json:"id"`
						DefaultConfiguration struct {
							Level string `json:"level"`
						} `json:"defaultConfiguration"`
						Properties struct {
							SecuritySeverity string   `json:"security-severity"`
							Tags             []string `json:"tags"`
						} `json:"properties"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Invocations []struct {
				ExecutionSuccessful bool `json:"executionSuccessful"`
				Notifications       []struct {
					Level   string `json:"level"`
					Message struct {
						Text string `json:"text"`
					} `json:"message"`
				} `json:"toolExecutionNotifications"`
			} `json:"invocations"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					Physical *struct {
						Artifact struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine   int `json:"startLine"`
							StartColumn int `json:"startColumn"`
							EndLine     int `json:"endLine"`
							EndColumn   int `json:"endColumn"`
							Snippet     struct {
								Text string `json:"text"`
							} `json:"snippet"`
						} `json:"region"`
					} `json:"physicalLocation"`
					Logical []struct {
						Name string `json:"name"`
						Kind string `json:"kind"`
					} `json:"logicalLocations"`
				} `json:"locations"`
				Suppressions []struct {
					Kind string `json:"kind"`
				} `json:"suppressions"`
			} `json:"results"`
		} `json:"runs"`
	}
	err = json.Unmarshal(buf.Bytes(), &log)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(log.Runs, 1))
	run := log.Runs[0]

	t.Run("header", func(t *testing.T) {
		assert.Check(t, cmp.Equal(log.Version, "2.1.0"))
		assert.Check(t, cmp.Equal(log.Schema, sarifSchemaURL))
		assert.Check(t, cmp.Equal(run.Tool.Driver.Name, "circleci-config-audit"))
		assert.Check(t, cmp.Equal(run.Tool.Driver.Version, "1.2.3"))
	})

	t.Run("rules are the catalog", func(t *testing.T) {
		assert.Assert(t, cmp.Len(run.Tool.Driver.Rules, len(Catalog())))
		for i, rule := range Catalog() {
			got := run.Tool.Driver.Rules[i]
			assert.Check(t, cmp.Equal(got.ID, rule.ID))
			assert.Check(t, cmp.Equal(got.DefaultConfiguration.Level, map[Severity]string{
				SeverityHigh: "error", SeverityMedium: "warning", SeverityLow: "note",
			}[rule.Severity]), "rule %s", rule.ID)
			assert.Check(t, cmp.Equal(got.Properties.SecuritySeverity, map[Severity]string{
				SeverityHigh: "8.0", SeverityMedium: "5.0", SeverityLow: "2.0",
			}[rule.Severity]), "rule %s", rule.ID)
			assert.Check(t, cmp.DeepEqual(got.Properties.Tags, []string{"security"}))
		}
	})

	t.Run("results", func(t *testing.T) {
		assert.Assert(t, cmp.Len(run.Results, 3))

		positioned := run.Results[0]
		assert.Check(t, cmp.Equal(positioned.RuleID, "unpinned-orb"))
		assert.Check(t, cmp.Equal(positioned.Level, "warning"))
		assert.Check(t, cmp.Len(positioned.Suppressions, 0))
		assert.Assert(t, cmp.Len(positioned.Locations, 1))
		phys := positioned.Locations[0].Physical
		assert.Assert(t, phys != nil)
		assert.Check(t, cmp.Equal(phys.Artifact.URI, ".circleci/config.yml"))
		assert.Check(t, cmp.Equal(phys.Region.StartLine, 2))
		assert.Check(t, cmp.Equal(phys.Region.StartColumn, 9))
		assert.Check(t, cmp.Equal(phys.Region.EndLine, 2))
		assert.Check(t, cmp.Equal(phys.Region.EndColumn, 24))
		assert.Check(t, cmp.Equal(phys.Region.Snippet.Text, "  node: circleci/node@5"))

		settings := run.Results[1]
		assert.Check(t, cmp.Equal(settings.RuleID, "fork-pr-secrets"))
		assert.Check(t, cmp.Equal(settings.Level, "error"))
		assert.Assert(t, cmp.Len(settings.Locations, 1))
		assert.Check(t, settings.Locations[0].Physical == nil)
		assert.Assert(t, cmp.Len(settings.Locations[0].Logical, 1))
		assert.Check(t, cmp.Equal(settings.Locations[0].Logical[0].Name, "project settings"))
		assert.Check(t, cmp.Equal(settings.Locations[0].Logical[0].Kind, "module"))

		suppressed := run.Results[2]
		assert.Check(t, cmp.Equal(suppressed.RuleID, "unpinned-image"))
		assert.Assert(t, cmp.Len(suppressed.Suppressions, 1))
		assert.Check(t, cmp.Equal(suppressed.Suppressions[0].Kind, "inSource"))
	})

	t.Run("warnings become notifications", func(t *testing.T) {
		assert.Assert(t, cmp.Len(run.Invocations, 1))
		inv := run.Invocations[0]
		assert.Check(t, inv.ExecutionSuccessful)
		assert.Assert(t, cmp.Len(inv.Notifications, 1))
		assert.Check(t, cmp.Equal(inv.Notifications[0].Level, "warning"))
		assert.Check(t, cmp.Equal(inv.Notifications[0].Message.Text, "could not read project settings: no token; offline results only"))
	})
}

func TestSarifURI(t *testing.T) {
	cwd := t.TempDir()
	inside := filepath.Join(cwd, ".circleci", "config.yml")
	outside := filepath.Join(filepath.Dir(cwd), "elsewhere", "config.yml")
	// A path outside cwd keeps its absolute form; Windows drive paths gain a
	// leading slash after file://.
	wantOutside := "file://" + filepath.ToSlash(outside)
	if runtime.GOOS == "windows" {
		wantOutside = "file:///" + filepath.ToSlash(outside)
	}
	tests := []struct {
		name string
		file string
		want string
	}{
		{
			name: "stdin",
			file: "<stdin>",
			want: "%3Cstdin%3E",
		},
		{
			name: "relative",
			file: ".circleci/config.yml",
			want: ".circleci/config.yml",
		},
		{
			name: "relative with dot segments",
			file: "../other/config.yml",
			want: "../other/config.yml",
		},
		{
			name: "absolute inside the working directory",
			file: inside,
			want: ".circleci/config.yml",
		},
		{
			name: "absolute outside the working directory",
			file: outside,
			want: wantOutside,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(sarifURI(tc.file, cwd), tc.want))
		})
	}
}
