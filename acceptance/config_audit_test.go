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
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/configaudit"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	// language=yaml
	auditOneOrbYAML = "orbs:\n  node: circleci/node@5\njobs: {}\n"
	auditConfigPath = ".circleci/config.yml"
)

// auditFixture copies acceptance/testdata/config-audit/<name> to dir/.circleci/config.yml.
func auditFixture(t *testing.T, dir, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "config-audit", name))
	assert.NilError(t, err)
	writeConfig(t, dir, string(b))
}

func runAudit(t *testing.T, dir string, stdin io.Reader, args ...string) binary.CLIResult {
	t.Helper()
	env := testenv.New(t)
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"config", "audit"}, args...),
		Env:     env.Environ(),
		WorkDir: dir,
		Stdin:   stdin,
	})
}

func TestConfigAudit_Clean(t *testing.T) {
	dir := t.TempDir()
	auditFixture(t, dir, "clean.yml")
	result := runAudit(t, dir, nil)
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigAudit_Findings(t *testing.T) {
	dir := t.TempDir()
	auditFixture(t, dir, "all-rules.yml")
	result := runAudit(t, dir, nil)
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// auditPos is one finding's rule id and start position.
type auditPos struct {
	RuleID string
	Line   int
	Column int
}

// auditPositions returns the findings' positions from --json output, in order.
func auditPositions(t *testing.T, stdout string) []auditPos {
	t.Helper()
	var out struct {
		Findings []struct {
			RuleID string `json:"rule_id"`
			Line   int    `json:"line"`
			Column int    `json:"column"`
		} `json:"findings"`
	}
	err := json.Unmarshal([]byte(stdout), &out)
	assert.NilError(t, err)
	got := make([]auditPos, 0, len(out.Findings))
	for _, f := range out.Findings {
		got = append(got, auditPos{RuleID: f.RuleID, Line: f.Line, Column: f.Column})
	}
	return got
}

func TestConfigAudit_AllRules_JSON(t *testing.T) {
	dir := t.TempDir()
	auditFixture(t, dir, "all-rules.yml")
	result := runAudit(t, dir, nil, "--json")
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	got := auditPositions(t, result.Stdout)
	assert.Check(t, cmp.DeepEqual(got, []auditPos{
		{RuleID: "unpinned-orb", Line: 4, Column: 9},
		{RuleID: "unpinned-orb", Line: 5, Column: 12},
		{RuleID: "unpinned-image", Line: 10, Column: 16},
		{RuleID: "hardcoded-secret", Line: 13, Column: 21},
		{RuleID: "pipeline-value-injection", Line: 21, Column: 26},
		{RuleID: "hardcoded-secret", Line: 26, Column: 21},
		{RuleID: "ssh-keys-unrestricted", Line: 29, Column: 9},
		{RuleID: "curl-pipe-shell", Line: 30, Column: 14},
		{RuleID: "pipeline-value-injection", Line: 33, Column: 26},
		{RuleID: "cache-poisoning", Line: 37, Column: 16},
		{RuleID: "dynamic-config-untrusted", Line: 42, Column: 31},
		{RuleID: "unpinned-image", Line: 45, Column: 16},
		{RuleID: "context-on-any-branch", Line: 52, Column: 20},
	}))

	t.Run("summary", func(t *testing.T) {
		var out struct {
			Summary map[string]any `json:"summary"`
		}
		err := json.Unmarshal([]byte(result.Stdout), &out)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(out.Summary, map[string]any{
			"high": float64(5), "medium": float64(7), "low": float64(1), "total": float64(13), "suppressed": float64(0),
		}))
	})

	t.Run("secrets are redacted", func(t *testing.T) {
		assert.Check(t, !strings.Contains(result.Stdout, "hunter2hunter2"),
			"the docker auth password must not appear in the report")
		assert.Check(t, !strings.Contains(result.Stdout, "ghp_abcdefghijklmnopqrstuvwxyz0123456789"),
			"the GitHub token must not appear in the report")
		assert.Check(t, cmp.Contains(result.Stdout, "ghp_…"))
	})
}

// auditSARIF is the part of a SARIF log the acceptance tests read.
type auditSARIF struct {
	Version string `json:"version"`
	Runs    []struct {
		Tool struct {
			Driver struct {
				Name  string            `json:"name"`
				Rules []json.RawMessage `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID    string `json:"ruleId"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine   int `json:"startLine"`
						StartColumn int `json:"startColumn"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
			Suppressions []struct {
				Kind string `json:"kind"`
			} `json:"suppressions"`
		} `json:"results"`
	} `json:"runs"`
}

// parseAuditSARIF unmarshals stdout and gates on there being exactly one run.
func parseAuditSARIF(t *testing.T, stdout string) auditSARIF {
	t.Helper()
	var log auditSARIF
	err := json.Unmarshal([]byte(stdout), &log)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(log.Runs, 1))
	return log
}

func TestConfigAudit_SARIF(t *testing.T) {
	dir := t.TempDir()
	auditFixture(t, dir, "all-rules.yml")
	result := runAudit(t, dir, nil, "--format", "sarif")
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	log := parseAuditSARIF(t, result.Stdout)
	run := log.Runs[0]

	t.Run("header", func(t *testing.T) {
		assert.Check(t, cmp.Equal(log.Version, "2.1.0"))
		assert.Check(t, cmp.Equal(run.Tool.Driver.Name, "circleci-config-audit"))
		assert.Check(t, cmp.Len(run.Tool.Driver.Rules, len(configaudit.Catalog())))
	})

	t.Run("results", func(t *testing.T) {
		got := make([]auditPos, 0, len(run.Results))
		for _, r := range run.Results {
			assert.Assert(t, cmp.Len(r.Locations, 1))
			loc := r.Locations[0].PhysicalLocation
			got = append(got, auditPos{RuleID: r.RuleID, Line: loc.Region.StartLine, Column: loc.Region.StartColumn})
			assert.Check(t, cmp.Equal(loc.ArtifactLocation.URI, auditConfigPath), "%s", r.RuleID)
			assert.Check(t, cmp.Len(r.Suppressions, 0), "%s", r.RuleID)
		}
		assert.Check(t, cmp.DeepEqual(got, []auditPos{
			{RuleID: "unpinned-orb", Line: 4, Column: 9},
			{RuleID: "unpinned-orb", Line: 5, Column: 12},
			{RuleID: "unpinned-image", Line: 10, Column: 16},
			{RuleID: "hardcoded-secret", Line: 13, Column: 21},
			{RuleID: "pipeline-value-injection", Line: 21, Column: 26},
			{RuleID: "hardcoded-secret", Line: 26, Column: 21},
			{RuleID: "ssh-keys-unrestricted", Line: 29, Column: 9},
			{RuleID: "curl-pipe-shell", Line: 30, Column: 14},
			{RuleID: "pipeline-value-injection", Line: 33, Column: 26},
			{RuleID: "cache-poisoning", Line: 37, Column: 16},
			{RuleID: "dynamic-config-untrusted", Line: 42, Column: 31},
			{RuleID: "unpinned-image", Line: 45, Column: 16},
			{RuleID: "context-on-any-branch", Line: 52, Column: 20},
		}))
	})

	t.Run("secrets are redacted", func(t *testing.T) {
		assert.Check(t, !strings.Contains(result.Stdout, "ghp_abcdefghijklmnopqrstuvwxyz0123456789"),
			"the GitHub token must not appear in the log")
		assert.Check(t, !strings.Contains(result.Stdout, "hunter2hunter2"),
			"the docker auth password must not appear in the log")
	})

	t.Run("stderr is quiet", func(t *testing.T) {
		assert.Check(t, !strings.Contains(result.Stderr, "warning:"))
	})
}

func TestConfigAudit_NotAPipeline(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "sequence",
			yaml: "- 1\n- 2\n",
		},
		{
			name: "no sections",
			yaml: "version: 2.1\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, tc.yaml)
			result := runAudit(t, dir, nil)
			assert.Check(t, cmp.Equal(result.ExitCode, 0))
			assert.Check(t, cmp.Equal(result.Stdout, "No findings at or above low.\n"))
			assert.Check(t, cmp.Equal(result.Stderr, ""))
		})
	}
}

func TestConfigAudit_JSON(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, auditOneOrbYAML)
	result := runAudit(t, dir, nil, "--json")
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, cmp.Contains(result.Stderr, `"exit_code": 7`))

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"file": auditConfigPath,
		"findings": []any{
			map[string]any{
				"rule_id":    "unpinned-orb",
				"severity":   "medium",
				"confidence": "high",
				"message":    "orb circleci/node@5 is a major-version alias",
				"fix":        "pin the orb to an exact version (name@X.Y.Z)",
				"file":       auditConfigPath,
				"line":       float64(2),
				"column":     float64(9),
				"end_line":   float64(2),
				"end_column": float64(24),
				"snippet":    "  node: circleci/node@5",
			},
		},
		"summary": map[string]any{
			"high":       float64(0),
			"medium":     float64(1),
			"low":        float64(0),
			"total":      float64(1),
			"suppressed": float64(0),
		},
		"warnings": []any{},
	}))
}

func TestConfigAudit_MinSeverity(t *testing.T) {
	rank := map[string]int{"low": 0, "medium": 1, "high": 2}

	t.Run("findings at or above the threshold", func(t *testing.T) {
		for _, threshold := range []string{"low", "medium", "high"} {
			t.Run(threshold, func(t *testing.T) {
				dir := t.TempDir()
				auditFixture(t, dir, "all-rules.yml")
				result := runAudit(t, dir, nil, "--min-severity", threshold, "--json")
				assert.Check(t, cmp.Equal(result.ExitCode, 7))

				var out struct {
					Findings []struct {
						RuleID   string `json:"rule_id"`
						Severity string `json:"severity"`
						Line     int    `json:"line"`
						Column   int    `json:"column"`
					} `json:"findings"`
				}
				err := json.Unmarshal([]byte(result.Stdout), &out)
				assert.NilError(t, err)
				assert.Assert(t, len(out.Findings) != 0)
				for _, f := range out.Findings {
					assert.Check(t, rank[f.Severity] >= rank[threshold],
						"%s at %d:%d is %s, below %s", f.RuleID, f.Line, f.Column, f.Severity, threshold)
				}
				if threshold == "high" {
					found := false
					for _, f := range out.Findings {
						if f.RuleID == "unpinned-image" && f.Line == 10 && f.Column == 16 {
							found = true
						}
					}
					assert.Check(t, found, "unpinned-image at 10:16 is high and must be reported")
				}
			})
		}
	})

	t.Run("nothing at or above high", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, auditOneOrbYAML)
		result := runAudit(t, dir, nil, "--min-severity", "high")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(result.Stdout, "No findings at or above high.\n"))
		assert.Check(t, cmp.Equal(result.Stderr, ""))
	})
}

func TestConfigAudit_Suppressed(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantExit int
	}{
		{
			name: "above and end of line",
			yaml: "orbs:\n" +
				"  # circleci-audit: ignore[unpinned-orb]\n" +
				"  node: circleci/node@5\n" +
				"  aws: circleci/aws-cli@volatile # circleci-audit: ignore[unpinned-orb]\n" +
				"jobs: {}\n",
			wantExit: 0,
		},
		{
			name: "inside a run block",
			yaml: "jobs:\n" +
				"  build:\n" +
				"    docker:\n" +
				"      - image: cimg/base:1@sha256:0123abcd\n" +
				"    steps:\n" +
				"      - run: |\n" +
				"          echo start\n" +
				"          # circleci-audit: ignore[curl-pipe-shell]\n" +
				"          curl -fsSL https://example.com/install.sh | sh\n",
			wantExit: 0,
		},
		{
			name: "unknown rule id",
			yaml: "orbs:\n" +
				"  # circleci-audit: ignore[no-such-rule]\n" +
				"  node: circleci/node@5\n" +
				"jobs: {}\n",
			wantExit: 7,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, tc.yaml)
			result := runAudit(t, dir, nil)
			assert.Check(t, cmp.Equal(result.ExitCode, tc.wantExit))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
			if tc.wantExit == 7 {
				assert.Check(t, cmp.Contains(result.Stderr, "no-such-rule"))
			}
		})
	}

	t.Run("sarif keeps the suppressed finding", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, "orbs:\n"+
			"  # circleci-audit: ignore[unpinned-orb]\n"+
			"  node: circleci/node@5\n"+
			"jobs: {}\n")
		result := runAudit(t, dir, nil, "--format", "sarif")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		log := parseAuditSARIF(t, result.Stdout)
		assert.Assert(t, cmp.Len(log.Runs[0].Results, 1))
		res := log.Runs[0].Results[0]
		assert.Check(t, cmp.Equal(res.RuleID, "unpinned-orb"))
		assert.Assert(t, cmp.Len(res.Suppressions, 1))
		assert.Check(t, cmp.Equal(res.Suppressions[0].Kind, "inSource"))
	})
}

func TestConfigAudit_Stdin(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		result := runAudit(t, t.TempDir(), strings.NewReader(auditOneOrbYAML), "-")
		assert.Check(t, cmp.Equal(result.ExitCode, 7))
		assert.Check(t, cmp.Contains(result.Stdout, "<stdin>:2:9"))
	})

	t.Run("sarif", func(t *testing.T) {
		result := runAudit(t, t.TempDir(), strings.NewReader(auditOneOrbYAML), "-", "--format", "sarif")
		assert.Check(t, cmp.Equal(result.ExitCode, 7))
		log := parseAuditSARIF(t, result.Stdout)
		assert.Assert(t, cmp.Len(log.Runs[0].Results, 1))
		locs := log.Runs[0].Results[0].Locations
		assert.Assert(t, cmp.Len(locs, 1))
		assert.Check(t, cmp.Equal(locs[0].PhysicalLocation.ArtifactLocation.URI, "%3Cstdin%3E"))
	})
}

func TestConfigAudit_Errors(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		args     []string
		wantExit int
	}{
		{
			name:     "missing file",
			args:     []string{"nope.yml"},
			wantExit: 2,
		},
		{
			name:     "unparseable yaml",
			yaml:     "jobs: [\n",
			wantExit: 2,
		},
		{
			name:     "bad min severity",
			yaml:     auditOneOrbYAML,
			args:     []string{"--min-severity", "extreme"},
			wantExit: 2,
		},
		{
			name:     "bad format",
			yaml:     auditOneOrbYAML,
			args:     []string{"--format", "xml"},
			wantExit: 2,
		},
		{
			name:     "project without online",
			yaml:     auditOneOrbYAML,
			args:     []string{"--project", "gh/acme/web"},
			wantExit: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.yaml != "" {
				writeConfig(t, dir, tc.yaml)
			}
			result := runAudit(t, dir, nil, tc.args...)
			assert.Check(t, cmp.Equal(result.ExitCode, tc.wantExit))
			assert.Check(t, cmp.Equal(result.Stdout, ""))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}

	// The exit code for mutually exclusive flags comes from cobra's flag
	// validation, not from this command.
	t.Run("json with format", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, auditOneOrbYAML)
		result := runAudit(t, dir, nil, "--json", "--format", "sarif")
		assert.Check(t, result.ExitCode != 0)
		assert.Check(t, cmp.Equal(result.Stdout, ""))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	// The exit code for too many arguments comes from cobra's argument
	// validation, not from this command.
	t.Run("too many args", func(t *testing.T) {
		result := runAudit(t, t.TempDir(), nil, "a.yml", "b.yml")
		assert.Check(t, result.ExitCode != 0)
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})
}

const (
	auditOrgID     = "00000000-0000-0000-0000-00000000a0d1"
	auditProjectID = "00000000-0000-0000-0000-00000000a0d2"
	auditContextID = "00000000-0000-0000-0000-00000000a0d3"
	auditSlug      = "gh/acme/web"
)

// onlineConfig uses one context so --online reads its restrictions.
const onlineConfig = `version: 2.1
jobs:
  build:
    docker:
      - image: cimg/base:2024.01@sha256:0000000000000000000000000000000000000000000000000000000000000000
    steps:
      - checkout
workflows:
  main:
    jobs:
      - build:
          context: org-secrets
`

// onlineFake is the default fake for --online tests: a project with the given
// settings, a safe org, and a context restricted only to All members (which
// counts as unrestricted).
func onlineFake(t *testing.T, project map[string]any) *fakes.CircleCI {
	t.Helper()
	return onlineFakeWithRestriction(t, project, fakes.ContextRestriction{
		ID: "00000000-0000-0000-0000-00000000a0d4", RestrictionType: "group",
		MatchPattern: auditOrgID, Name: "All members",
	})
}

func onlineFakeWithRestriction(t *testing.T, project map[string]any, restriction fakes.ContextRestriction) *fakes.CircleCI {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddOrg(auditOrgID, "gh/acme", "acme", "github")
	fake.AddProjectBySlug(auditSlug, auditProjectID, "web", auditOrgID)
	if project != nil {
		fake.SetProjectSettings(auditProjectID, project)
	}
	fake.SetOrgSettings(auditOrgID, map[string]any{
		"is_user_checkout_keys_disabled": true,
		"enable_uncertified_public_orbs": false,
	})
	fake.AddContext(auditOrgID, fakes.Context{ID: auditContextID, Name: "org-secrets", CreatedAt: "2020-01-01T12:00:00Z"})
	fake.AddContextRestriction(auditContextID, restriction)
	return fake
}

var riskyProject = map[string]any{
	"enable_building_fork_prs":               true,
	"can_pass_secrets_to_fork_pr_jobs":       true,
	"is_ssh_disabled":                        false,
	"is_admin_required_for_writing_settings": true,
}

// onlineEnv points the CLI at fake, with a token when withToken is set.
func onlineEnv(t *testing.T, fake *fakes.CircleCI, withToken bool) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	if withToken {
		env.Token = testToken
	}
	env.CircleCIURL = fake.URL()
	return env
}

func runOnlineAudit(t *testing.T, env *testenv.TestEnv, dir string, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"config", "audit"}, args...),
		Env:     env.Environ(),
		WorkDir: dir,
	})
}

// auditOnlineOut is the part of --json output the --online tests read.
type auditOnlineOut struct {
	Findings []struct {
		RuleID   string `json:"rule_id"`
		Severity string `json:"severity"`
		Location string `json:"location"`
	} `json:"findings"`
	Online struct {
		Checked  bool     `json:"checked"`
		Project  string   `json:"project"`
		Requests []string `json:"requests"`
	} `json:"online"`
	Warnings []string `json:"warnings"`
}

type auditOnlineRow struct{ RuleID, Severity, Location string }

func (o auditOnlineOut) rows() []auditOnlineRow {
	got := make([]auditOnlineRow, 0, len(o.Findings))
	for _, f := range o.Findings {
		got = append(got, auditOnlineRow{RuleID: f.RuleID, Severity: f.Severity, Location: f.Location})
	}
	return got
}

func TestConfigAudit_Online(t *testing.T) {
	fake := onlineFake(t, riskyProject)
	env := onlineEnv(t, fake, true)
	dir := t.TempDir()
	writeConfig(t, dir, onlineConfig)

	run := func(args ...string) binary.CLIResult {
		return runOnlineAudit(t, env, dir, append([]string{"--online", "--project", auditSlug}, args...)...)
	}

	t.Run("text", func(t *testing.T) {
		result := run()
		assert.Check(t, cmp.Equal(result.ExitCode, 7))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})

	t.Run("json", func(t *testing.T) {
		result := run("--json")
		assert.Check(t, cmp.Equal(result.ExitCode, 7))
		var out auditOnlineOut
		err := json.Unmarshal([]byte(result.Stdout), &out)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(out.rows(), []auditOnlineRow{
			{RuleID: "context-on-any-branch", Severity: "medium", Location: ""},
			{RuleID: "fork-pr-secrets", Severity: "high", Location: "project settings"},
			{RuleID: "context-unrestricted", Severity: "medium", Location: "context org-secrets"},
			{RuleID: "ssh-rerun-allowed", Severity: "medium", Location: "project settings"},
		}))
		assert.Check(t, out.Online.Checked)
		assert.Check(t, cmp.Equal(out.Online.Project, auditSlug))
		assert.Check(t, cmp.Len(out.Warnings, 0))
		assert.Check(t, cmp.DeepEqual(out.Online.Requests, []string{
			"GET /api/v3/projects?filter[slug]=" + auditSlug,
			"GET /api/v3/projects/" + auditProjectID + "/settings",
			"GET /api/v3/orgs/" + auditOrgID + "/settings",
			"GET /api/v3/contexts?filter[org_id]=" + auditOrgID + "&filter[name]=org-secrets",
			"GET /api/v3/context-restrictions?filter[context_id]=" + auditContextID,
		}))
	})

	t.Run("sarif", func(t *testing.T) {
		result := run("--format", "sarif")
		assert.Check(t, cmp.Equal(result.ExitCode, 7))
		log := parseAuditSARIF(t, result.Stdout)
		got := make([]string, 0, len(log.Runs[0].Results))
		for _, r := range log.Runs[0].Results {
			got = append(got, r.RuleID)
		}
		assert.Check(t, cmp.DeepEqual(got, []string{
			"context-on-any-branch", "fork-pr-secrets", "context-unrestricted", "ssh-rerun-allowed",
		}))
	})

	t.Run("only GET requests", func(t *testing.T) {
		requests := fake.AllRequests()
		assert.Check(t, len(requests) > 0, "the audit made no request")
		for _, req := range requests {
			assert.Check(t, cmp.Equal(req.Method, http.MethodGet), "request %s", req.URL.Path)
		}
	})

	t.Run("a project-restricted context is downgraded", func(t *testing.T) {
		restricted := onlineFakeWithRestriction(t, riskyProject, fakes.ContextRestriction{
			ID: "00000000-0000-0000-0000-00000000a0d5", RestrictionType: "project",
			MatchPattern: auditProjectID, Name: "web",
		})
		env := onlineEnv(t, restricted, true)
		args := []string{"--online", "--project", auditSlug, "--json"}

		t.Run("shown at low", func(t *testing.T) {
			result := runOnlineAudit(t, env, dir, args...)
			assert.Check(t, cmp.Equal(result.ExitCode, 7))
			var out auditOnlineOut
			err := json.Unmarshal([]byte(result.Stdout), &out)
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(out.rows(), []auditOnlineRow{
				{RuleID: "context-on-any-branch", Severity: "low", Location: ""},
				{RuleID: "fork-pr-secrets", Severity: "high", Location: "project settings"},
				{RuleID: "ssh-rerun-allowed", Severity: "medium", Location: "project settings"},
			}))
		})

		t.Run("hidden by --min-severity medium, still exit 7", func(t *testing.T) {
			result := runOnlineAudit(t, env, dir, append(args, "--min-severity", "medium")...)
			assert.Check(t, cmp.Equal(result.ExitCode, 7), "fork-pr-secrets still fails the run")
			var out auditOnlineOut
			err := json.Unmarshal([]byte(result.Stdout), &out)
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(out.rows(), []auditOnlineRow{
				{RuleID: "fork-pr-secrets", Severity: "high", Location: "project settings"},
				{RuleID: "ssh-rerun-allowed", Severity: "medium", Location: "project settings"},
			}))
		})
	})
}

const (
	// language=yaml
	onlineNopeConfig = `version: 2.1
jobs:
  build:
    docker:
      - image: cimg/base:2024.01@sha256:0000000000000000000000000000000000000000000000000000000000000000
    steps:
      - checkout
workflows:
  main:
    jobs:
      - build:
          context: nope
`
	// language=yaml
	onlineParameterisedConfig = `version: 2.1
parameters:
  ctx:
    type: string
    default: org-secrets
jobs:
  build:
    docker:
      - image: cimg/base:2024.01@sha256:0000000000000000000000000000000000000000000000000000000000000000
    steps:
      - checkout
workflows:
  main:
    jobs:
      - build:
          context: << pipeline.parameters.ctx >>
`
)

// An --online failure is a warning: the offline findings still print and the
// exit code stays 7, from the offline context-on-any-branch finding.
func TestConfigAudit_OnlineFailures(t *testing.T) {
	tests := []struct {
		name string
		// fake builds the server; the default is onlineFake with risky settings.
		fake      func(t *testing.T) *fakes.CircleCI
		noToken   bool
		yaml      string
		args      []string
		stderrHas string
		check     func(t *testing.T, result binary.CLIResult, fake *fakes.CircleCI)
	}{
		{
			name:    "no token",
			noToken: true,
			args:    []string{"--online", "--project", auditSlug},
			check: func(t *testing.T, _ binary.CLIResult, fake *fakes.CircleCI) {
				assert.Check(t, cmp.Len(fake.AllRequests(), 0))
			},
		},
		{
			name: "settings call fails",
			fake: func(t *testing.T) *fakes.CircleCI { return onlineFake(t, nil) },
			args: []string{"--online", "--project", auditSlug},
		},
		{
			name: "unknown project",
			args: []string{"--online", "--project", "gh/acme/missing"},
		},
		{
			name: "context not found",
			yaml: onlineNopeConfig,
			args: []string{"--online", "--project", auditSlug},
		},
		{
			name:    "json warnings",
			noToken: true,
			args:    []string{"--online", "--project", auditSlug, "--json"},
			check: func(t *testing.T, result binary.CLIResult, _ *fakes.CircleCI) {
				var out auditOnlineOut
				err := json.Unmarshal([]byte(result.Stdout), &out)
				assert.NilError(t, err)
				assert.Check(t, cmp.DeepEqual(out.Warnings, []string{
					"no CircleCI API token; skipping --online checks, offline results only",
				}))
				var stderr map[string]any
				err = json.Unmarshal([]byte(result.Stderr), &stderr)
				assert.Check(t, err, "stderr is one JSON object")
			},
		},
		{
			name:    "sarif warnings",
			noToken: true,
			args:    []string{"--online", "--project", auditSlug, "--format", "sarif"},
			check: func(t *testing.T, result binary.CLIResult, _ *fakes.CircleCI) {
				var log struct {
					Runs []struct {
						Invocations []struct {
							ToolExecutionNotifications []struct {
								Message struct {
									Text string `json:"text"`
								} `json:"message"`
							} `json:"toolExecutionNotifications"`
						} `json:"invocations"`
					} `json:"runs"`
				}
				err := json.Unmarshal([]byte(result.Stdout), &log)
				assert.NilError(t, err)
				assert.Assert(t, cmp.Len(log.Runs, 1))
				assert.Assert(t, cmp.Len(log.Runs[0].Invocations, 1))
				notes := log.Runs[0].Invocations[0].ToolExecutionNotifications
				assert.Assert(t, cmp.Len(notes, 1))
				assert.Check(t, cmp.Equal(notes[0].Message.Text,
					"no CircleCI API token; skipping --online checks, offline results only"))
				assert.Check(t, !strings.Contains(result.Stderr, "warning:"))
			},
		},
		{
			// The temp dir has no git remote and no .circleci/info.yml.
			name:      "no project resolvable",
			args:      []string{"--online"},
			stderrHas: "could not read git remote",
			check: func(t *testing.T, _ binary.CLIResult, fake *fakes.CircleCI) {
				assert.Check(t, cmp.Len(fake.AllRequests(), 0))
			},
		},
		{
			name: "parameterised context",
			yaml: onlineParameterisedConfig,
			args: []string{"--online", "--project", auditSlug},
			check: func(t *testing.T, _ binary.CLIResult, fake *fakes.CircleCI) {
				for _, req := range fake.AllRequests() {
					assert.Check(t, req.URL.Path != "/api/v3/contexts", "request %s", req.URL)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := onlineFake(t, riskyProject)
			if tc.fake != nil {
				fake = tc.fake(t)
			}
			env := onlineEnv(t, fake, !tc.noToken)
			dir := t.TempDir()
			yaml := onlineConfig
			if tc.yaml != "" {
				yaml = tc.yaml
			}
			writeConfig(t, dir, yaml)

			result := runOnlineAudit(t, env, dir, tc.args...)
			assert.Check(t, cmp.Equal(result.ExitCode, 7))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			if tc.stderrHas != "" {
				// git's own message is version- and platform-dependent, so match
				// a stable substring rather than golden the whole stderr.
				assert.Check(t, cmp.Contains(result.Stderr, tc.stderrHas))
				assert.Check(t, cmp.Contains(result.Stderr, "warning: "))
			} else {
				assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
			}
			if tc.check != nil {
				tc.check(t, result, fake)
			}
		})
	}
}

func TestConfigAudit_HelpTopic(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"help", "config-audit"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	for _, rule := range configaudit.Catalog() {
		assert.Check(t, cmp.Contains(result.Stdout, rule.ID), "help topic must document rule %s", rule.ID)
	}
	for _, want := range []string{"circleci-audit: ignore[", "--format sarif", "not expanded"} {
		assert.Check(t, cmp.Contains(result.Stdout, want))
	}
}
