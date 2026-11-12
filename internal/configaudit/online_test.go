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
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// safeProject and safeOrg trip no settings rule.
func safeProject() *apiclient.ProjectSettingsAttributes {
	return &apiclient.ProjectSettingsAttributes{DisableSSH: true, IsAdminRequiredForWriting: true}
}

func safeOrg() *apiclient.OrgSettingsAttributes {
	return &apiclient.OrgSettingsAttributes{UserCheckoutKeysDisabled: true}
}

type settingsRow struct {
	ID       string
	Severity Severity
	Location string
}

func rowsOf(findings []Finding) []settingsRow {
	got := make([]settingsRow, 0, len(findings))
	for _, f := range findings {
		got = append(got, settingsRow{ID: f.RuleID, Severity: f.Severity, Location: f.Location})
	}
	return got
}

func restriction(kind, pattern string) apiclient.ContextRestriction {
	return apiclient.ContextRestriction{ID: uuid.New(), RestrictionType: kind, MatchPattern: pattern}
}

func TestSettingsFindings(t *testing.T) {
	tests := []struct {
		name    string
		s       Settings
		offline []Finding
		want    []settingsRow
	}{
		{
			name: "nothing read emits nothing",
			s:    Settings{},
		},
		{
			name: "safe settings emit nothing",
			s:    Settings{Proj: safeProject(), Org: safeOrg()},
		},
		{
			name: "fork pr secrets needs both settings",
			s: Settings{Proj: &apiclient.ProjectSettingsAttributes{
				DisableSSH: true, IsAdminRequiredForWriting: true,
				BuildForkPRs: true, CanPassSecretsToForkPR: true,
			}},
			want: []settingsRow{{ID: "fork-pr-secrets", Severity: SeverityHigh, Location: "project settings"}},
		},
		{
			name: "fork prs built without secrets passed",
			s: Settings{Proj: &apiclient.ProjectSettingsAttributes{
				DisableSSH: true, IsAdminRequiredForWriting: true, BuildForkPRs: true,
			}},
		},
		{
			name: "secrets allowed but fork prs not built",
			s: Settings{Proj: &apiclient.ProjectSettingsAttributes{
				DisableSSH: true, IsAdminRequiredForWriting: true, CanPassSecretsToForkPR: true,
			}},
		},
		{
			name: "ssh reruns allowed",
			s:    Settings{Proj: &apiclient.ProjectSettingsAttributes{IsAdminRequiredForWriting: true}},
			want: []settingsRow{{ID: "ssh-rerun-allowed", Severity: SeverityMedium, Location: "project settings"}},
		},
		{
			name: "settings writable by non-admins",
			s:    Settings{Proj: &apiclient.ProjectSettingsAttributes{DisableSSH: true}},
			want: []settingsRow{{ID: "settings-not-admin-only", Severity: SeverityLow, Location: "project settings"}},
		},
		{
			name: "user checkout keys allowed",
			s:    Settings{Org: &apiclient.OrgSettingsAttributes{}},
			want: []settingsRow{{ID: "user-checkout-keys-allowed", Severity: SeverityLow, Location: "org settings"}},
		},
		{
			name: "uncertified orbs allowed is low alone",
			s:    Settings{Org: &apiclient.OrgSettingsAttributes{UserCheckoutKeysDisabled: true, UncertifiedPublicOrbs: true}},
			want: []settingsRow{{ID: "uncertified-orbs-allowed", Severity: SeverityLow, Location: "org settings"}},
		},
		{
			name:    "uncertified orbs allowed is medium with an unpinned orb",
			s:       Settings{Org: &apiclient.OrgSettingsAttributes{UserCheckoutKeysDisabled: true, UncertifiedPublicOrbs: true}},
			offline: []Finding{{RuleID: "unpinned-orb"}},
			want:    []settingsRow{{ID: "uncertified-orbs-allowed", Severity: SeverityMedium, Location: "org settings"}},
		},
		{
			name:    "an unpinned orb alone does not raise anything",
			s:       Settings{Org: safeOrg()},
			offline: []Finding{{RuleID: "unpinned-orb"}},
		},
		{
			name: "a context with no restrictions is unrestricted",
			s:    Settings{Contexts: map[string][]apiclient.ContextRestriction{"deploy": {}}},
			want: []settingsRow{{ID: "context-unrestricted", Severity: SeverityMedium, Location: "context deploy"}},
		},
		{
			name: "a group restriction alone is unrestricted",
			s: Settings{Contexts: map[string][]apiclient.ContextRestriction{
				"deploy": {restriction("group", "00000000-0000-0000-0000-000000000001")},
			}},
			want: []settingsRow{{ID: "context-unrestricted", Severity: SeverityMedium, Location: "context deploy"}},
		},
		{
			name: "a project restriction is restricted",
			s: Settings{Contexts: map[string][]apiclient.ContextRestriction{
				"deploy": {restriction("project", "00000000-0000-0000-0000-000000000002")},
			}},
		},
		{
			name: "an expression restriction is restricted",
			s: Settings{Contexts: map[string][]apiclient.ContextRestriction{
				"deploy": {restriction("group", "x"), restriction("expression", "pipeline.git.branch == \"main\"")},
			}},
		},
		{
			name: "unread contexts emit nothing and read ones are sorted",
			s: Settings{Contexts: map[string][]apiclient.ContextRestriction{
				"zeta": {}, "alpha": {},
			}},
			want: []settingsRow{
				{ID: "context-unrestricted", Severity: SeverityMedium, Location: "context alpha"},
				{ID: "context-unrestricted", Severity: SeverityMedium, Location: "context zeta"},
			},
		},
		{
			name: "every rule at once, each from its own input",
			s: Settings{
				Proj: &apiclient.ProjectSettingsAttributes{BuildForkPRs: true, CanPassSecretsToForkPR: true},
				Org:  &apiclient.OrgSettingsAttributes{UncertifiedPublicOrbs: true},
				Contexts: map[string][]apiclient.ContextRestriction{
					"deploy": {},
				},
			},
			want: []settingsRow{
				{ID: "fork-pr-secrets", Severity: SeverityHigh, Location: "project settings"},
				{ID: "ssh-rerun-allowed", Severity: SeverityMedium, Location: "project settings"},
				{ID: "settings-not-admin-only", Severity: SeverityLow, Location: "project settings"},
				{ID: "user-checkout-keys-allowed", Severity: SeverityLow, Location: "org settings"},
				{ID: "uncertified-orbs-allowed", Severity: SeverityLow, Location: "org settings"},
				{ID: "context-unrestricted", Severity: SeverityMedium, Location: "context deploy"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rowsOf(settingsFindings(&tc.s, tc.offline))
			want := tc.want
			if want == nil {
				want = []settingsRow{}
			}
			assert.Check(t, cmp.DeepEqual(got, want))
		})
	}

	t.Run("settings findings carry the catalog fix and confidence", func(t *testing.T) {
		got := settingsFindings(&Settings{Proj: &apiclient.ProjectSettingsAttributes{}}, nil)
		assert.Assert(t, cmp.Len(got, 2))
		rule, ok := ruleByID(got[0].RuleID)
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(got[0].Fix, rule.Fix))
		assert.Check(t, cmp.Equal(got[0].Confidence, rule.Confidence))
		assert.Check(t, got[0].Position == nil, "settings findings have no position")
	})
}

func TestSettingsFindingsDowngradeContextOnAnyBranch(t *testing.T) {
	offline := func() []Finding {
		return []Finding{
			{RuleID: "context-on-any-branch", Severity: SeverityMedium, Message: "job a gets context deploy on every branch", context: "deploy"},
			{RuleID: "context-on-any-branch", Severity: SeverityMedium, Message: "job a gets context other on every branch", context: "other"},
			{RuleID: "unpinned-orb", Severity: SeverityMedium, Message: "orb"},
		}
	}
	type result struct {
		Severity Severity
		Message  string
	}
	resultsOf := func(fs []Finding) []result {
		out := make([]result, 0, len(fs))
		for _, f := range fs {
			out = append(out, result{f.Severity, f.Message})
		}
		return out
	}
	untouched := []result{
		{SeverityMedium, "job a gets context deploy on every branch"},
		{SeverityMedium, "job a gets context other on every branch"},
		{SeverityMedium, "orb"},
	}

	t.Run("a project restriction lowers that context only", func(t *testing.T) {
		f := offline()
		got := settingsFindings(&Settings{Contexts: map[string][]apiclient.ContextRestriction{
			"deploy": {restriction("project", "p")},
		}}, f)
		assert.Check(t, cmp.Len(got, 0), "a restricted context is not unrestricted")
		assert.Check(t, cmp.DeepEqual(resultsOf(f), []result{
			{SeverityLow, "job a gets context deploy on every branch (context is restricted: project)"},
			{SeverityMedium, "job a gets context other on every branch"},
			{SeverityMedium, "orb"},
		}))
	})

	t.Run("the note lists each restriction type once", func(t *testing.T) {
		f := offline()
		settingsFindings(&Settings{Contexts: map[string][]apiclient.ContextRestriction{
			"deploy": {restriction("project", "p"), restriction("expression", "e"), restriction("project", "q"), restriction("group", "g")},
		}}, f)
		assert.Check(t, cmp.Equal(f[0].Message,
			"job a gets context deploy on every branch (context is restricted: expression, project)"))
	})

	t.Run("a group-only context is not lowered", func(t *testing.T) {
		f := offline()
		got := settingsFindings(&Settings{Contexts: map[string][]apiclient.ContextRestriction{
			"deploy": {restriction("group", "g")},
		}}, f)
		assert.Check(t, cmp.Len(got, 1))
		assert.Check(t, cmp.DeepEqual(resultsOf(f), untouched))
	})

	t.Run("a context that was not read is not lowered", func(t *testing.T) {
		f := offline()
		settingsFindings(&Settings{Proj: safeProject(), Org: safeOrg()}, f)
		assert.Check(t, cmp.DeepEqual(resultsOf(f), untouched))
	})
}

func TestAuditOnline(t *testing.T) {
	src := "version: 2.1\n" +
		"jobs:\n  build:\n    docker:\n      - image: cimg/base@sha256:abc\n    steps:\n      - checkout\n" +
		"workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n"
	restricted := &Settings{
		Project: "gh/acme/web",
		Proj:    &apiclient.ProjectSettingsAttributes{BuildForkPRs: true, CanPassSecretsToForkPR: true, DisableSSH: true, IsAdminRequiredForWriting: true},
		Contexts: map[string][]apiclient.ContextRestriction{
			"deploy": {restriction("project", "p")},
		},
		Requests: []string{"GET /a"},
	}

	t.Run("without online there is no online block and no settings finding", func(t *testing.T) {
		report := parseAudit(t, src, Options{})
		assert.Check(t, report.Online == nil)
		assert.Check(t, cmp.Equal(report.Summary.Total, 1))
	})

	t.Run("the downgrade happens before the minimum severity filter", func(t *testing.T) {
		report := parseAudit(t, src, Options{MinSeverity: SeverityMedium, Online: restricted})
		assert.Check(t, cmp.DeepEqual(rowsOf(report.Findings), []settingsRow{
			{ID: "fork-pr-secrets", Severity: SeverityHigh, Location: "project settings"},
		}))
		assert.Check(t, cmp.Equal(report.Summary.Total, 1))
	})

	t.Run("the lowered finding is kept at low", func(t *testing.T) {
		report := parseAudit(t, src, Options{Online: restricted})
		assert.Check(t, cmp.DeepEqual(rowsOf(report.Findings), []settingsRow{
			{ID: "context-on-any-branch", Severity: SeverityLow},
			{ID: "fork-pr-secrets", Severity: SeverityHigh, Location: "project settings"},
		}))
	})

	t.Run("the online block", func(t *testing.T) {
		report := parseAudit(t, src, Options{Online: restricted})
		assert.Assert(t, report.Online != nil)
		assert.Check(t, cmp.DeepEqual(*report.Online, Online{Checked: true, Project: "gh/acme/web", Requests: []string{"GET /a"}}))
	})

	t.Run("checked is false when nothing was read", func(t *testing.T) {
		report := parseAudit(t, src, Options{Online: &Settings{Project: "gh/acme/web"}})
		assert.Assert(t, report.Online != nil)
		assert.Check(t, !report.Online.Checked)
		assert.Check(t, cmp.Len(report.Online.Requests, 0))
	})

	t.Run("a parameterised context name is warned about", func(t *testing.T) {
		param := "version: 2.1\nworkflows:\n  main:\n    jobs:\n      - build:\n          context: << pipeline.parameters.ctx >>\n"
		report := parseAudit(t, param, Options{Online: &Settings{}})
		assert.Check(t, cmp.DeepEqual(report.Warnings, []string{
			"context << pipeline.parameters.ctx >> is parameterised; its restrictions were not checked",
		}))
		offline := parseAudit(t, param, Options{})
		assert.Check(t, cmp.Len(offline.Warnings, 0), "no warning without --online")
	})
}
