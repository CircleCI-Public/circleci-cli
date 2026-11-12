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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// sarifSchemaURL is the schema the log declares and that GitHub code scanning
// validates against.
const sarifSchemaURL = "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json"

// sarifToolName is the name the log gives its driver.
const sarifToolName = "circleci-config-audit"

// The structs below hold only the SARIF 2.1.0 fields the audit uses.

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Invocations []sarifInvocation `json:"invocations"`
	Results     []sarifResult     `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version,omitempty"`
	Rules   []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID                   string          `json:"id"`
	ShortDescription     sarifText       `json:"shortDescription"`
	FullDescription      sarifText       `json:"fullDescription"`
	Help                 sarifText       `json:"help"`
	DefaultConfiguration sarifRuleConfig `json:"defaultConfiguration"`
	Properties           sarifRuleProps  `json:"properties"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifRuleConfig struct {
	Level string `json:"level"`
}

type sarifRuleProps struct {
	SecuritySeverity string   `json:"security-severity"`
	Tags             []string `json:"tags"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications,omitempty"`
}

type sarifNotification struct {
	Level   string    `json:"level"`
	Message sarifText `json:"message"`
}

type sarifResult struct {
	RuleID       string             `json:"ruleId"`
	Level        string             `json:"level"`
	Message      sarifText          `json:"message"`
	Locations    []sarifLocation    `json:"locations"`
	Suppressions []sarifSuppression `json:"suppressions,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation *sarifPhysicalLocation `json:"physicalLocation,omitempty"`
	LogicalLocations []sarifLogicalLocation `json:"logicalLocations,omitempty"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int        `json:"startLine"`
	StartColumn int        `json:"startColumn"`
	EndLine     int        `json:"endLine"`
	EndColumn   int        `json:"endColumn"`
	Snippet     *sarifText `json:"snippet,omitempty"`
}

type sarifLogicalLocation struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type sarifSuppression struct {
	Kind string `json:"kind"`
}

// RenderSARIF writes the report as a SARIF 2.1.0 log: the whole rule catalog
// as the driver's rules, then a result for each finding followed by each
// suppressed finding (marked as suppressed in source). version is the CLI
// version, recorded on the driver. A finding's region snippet is its
// Finding.Snippet as given, when it has one.
func RenderSARIF(w io.Writer, r *Report, version string) error {
	cwd, _ := os.Getwd()

	rules := make([]sarifRule, 0, len(catalog))
	for _, rule := range catalog {
		rules = append(rules, sarifRule{
			ID:               rule.ID,
			ShortDescription: sarifText{Text: rule.Short},
			FullDescription:  sarifText{Text: rule.Why},
			Help:             sarifText{Text: rule.Fix},
			DefaultConfiguration: sarifRuleConfig{
				Level: sarifLevel(rule.Severity),
			},
			Properties: sarifRuleProps{
				SecuritySeverity: sarifSecuritySeverity(rule.Severity),
				Tags:             []string{"security"},
			},
		})
	}

	results := make([]sarifResult, 0, len(r.Findings)+len(r.Suppressed))
	for _, f := range r.Findings {
		results = append(results, sarifResultFor(f, r.File, cwd, false))
	}
	for _, f := range r.Suppressed {
		results = append(results, sarifResultFor(f, r.File, cwd, true))
	}

	inv := sarifInvocation{ExecutionSuccessful: true}
	for _, warning := range r.Warnings {
		inv.ToolExecutionNotifications = append(inv.ToolExecutionNotifications,
			sarifNotification{Level: "warning", Message: sarifText{Text: warning}})
	}

	log := sarifLog{
		Schema:  sarifSchemaURL,
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:        sarifTool{Driver: sarifDriver{Name: sarifToolName, Version: version, Rules: rules}},
			Invocations: []sarifInvocation{inv},
			Results:     results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func sarifResultFor(f Finding, reportFile, cwd string, suppressed bool) sarifResult {
	res := sarifResult{
		RuleID:  f.RuleID,
		Level:   sarifLevel(f.Severity),
		Message: sarifText{Text: f.Message},
	}
	if f.Position != nil {
		file := f.File
		if file == "" {
			file = reportFile
		}
		region := sarifRegion{
			StartLine:   f.Line,
			StartColumn: f.Column,
			EndLine:     f.EndLine,
			EndColumn:   f.EndColumn,
		}
		if f.Snippet != "" {
			region.Snippet = &sarifText{Text: f.Snippet}
		}
		res.Locations = []sarifLocation{{PhysicalLocation: &sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: sarifURI(file, cwd)},
			Region:           region,
		}}}
	} else {
		res.Locations = []sarifLocation{{LogicalLocations: []sarifLogicalLocation{
			{Name: f.Location, Kind: "module"},
		}}}
	}
	if suppressed {
		res.Suppressions = []sarifSuppression{{Kind: "inSource"}}
	}
	return res
}

// sarifLevel maps a severity to a SARIF level.
func sarifLevel(s Severity) string {
	switch s {
	case SeverityHigh:
		return "error"
	case SeverityMedium:
		return "warning"
	case SeverityLow:
		return "note"
	}
	return "note"
}

// sarifSecuritySeverity maps a severity to the 0-10 score GitHub ranks by.
func sarifSecuritySeverity(s Severity) string {
	switch s {
	case SeverityHigh:
		return "8.0"
	case SeverityMedium:
		return "5.0"
	case SeverityLow:
		return "2.0"
	}
	return "2.0"
}

// sarifURI returns the artifact URI for the audited file. Standard input is
// "<stdin>" percent-encoded, so it stays a valid URI reference; a relative path
// is kept as given, in slash form; an absolute path inside cwd becomes
// relative to it; one outside it is kept absolute as a file:// URI, which SARIF
// requires there.
func sarifURI(file, cwd string) string {
	if file == stdinLabel {
		return "%3Cstdin%3E"
	}
	if !filepath.IsAbs(file) {
		return filepath.ToSlash(file)
	}
	if cwd != "" {
		if rel, err := filepath.Rel(cwd, file); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	abs := filepath.ToSlash(file)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs // a Windows drive path: file:///C:/...
	}
	return "file://" + abs
}
