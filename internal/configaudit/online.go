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
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// Settings is what --online read from the CircleCI API. A part that could not
// be read is nil or absent, and its rules stay silent.
type Settings struct {
	Project  string                                    // slug, for messages
	Proj     *apiclient.ProjectSettingsAttributes      // nil when the call failed
	Org      *apiclient.OrgSettingsAttributes          // nil when the call failed
	Contexts map[string][]apiclient.ContextRestriction // by context name; missing key = not read
	Requests []string                                  // "GET <path>" for each call made
}

// Online reports what --online did, in the JSON report.
type Online struct {
	Checked  bool     `json:"checked"` // false when no settings call succeeded
	Project  string   `json:"project"`
	Requests []string `json:"requests"`
}

// Fetch reads the settings of project slug, of its org, and the restrictions of
// each named context, with GET requests only. Each call fails on its own into a
// warning and leaves its part of Settings empty; nothing here returns an error,
// so the offline rules always run.
func Fetch(ctx context.Context, client *apiclient.Client, slug string, contexts []string) (Settings, []string) {
	s := Settings{Project: slug, Contexts: map[string][]apiclient.ContextRestriction{}, Requests: []string{}}
	var warnings []string
	warn := func(what string, err error) {
		warnings = append(warnings, fmt.Sprintf("could not read %s: %v; offline results only", what, err))
	}

	s.Requests = append(s.Requests, "GET /api/v3/projects?filter[slug]="+slug)
	ref, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"could not find project %s: %v; skipping --online checks, offline results only", slug, err))
		return s, warnings
	}

	s.Requests = append(s.Requests, "GET /api/v3/projects/"+ref.ID.String()+"/settings")
	if proj, err := client.GetProjectSettings(ctx, ref.ID); err != nil {
		warn("project settings", err)
	} else {
		s.Proj = proj
	}

	s.Requests = append(s.Requests, "GET /api/v3/orgs/"+ref.OrgID.String()+"/settings")
	if org, err := client.GetOrgSettings(ctx, ref.OrgID); err != nil {
		warn("org settings", err)
	} else {
		s.Org = org
	}

	for _, name := range contexts {
		s.Requests = append(s.Requests, fmt.Sprintf("GET /api/v3/contexts?filter[org_id]=%s&filter[name]=%s",
			ref.OrgID, url.QueryEscape(name)))
		found, err := client.ListContexts(ctx, ref.OrgID, name)
		if err != nil {
			warn("context "+name, err)
			continue
		}
		// The API filters by case-insensitive substring; the config's name is exact.
		i := slices.IndexFunc(found, func(c apiclient.Context) bool { return c.Name == name })
		if i < 0 {
			warnings = append(warnings, fmt.Sprintf("context %s not found in the org", name))
			continue
		}
		s.Requests = append(s.Requests, "GET /api/v3/context-restrictions?filter[context_id]="+found[i].ID.String())
		restrictions, err := client.ListContextRestrictions(ctx, found[i].ID)
		if err != nil {
			warn("restrictions of context "+name, err)
			continue
		}
		s.Contexts[name] = restrictions
	}
	return s, warnings
}

// checked reports whether at least one settings call succeeded.
func (s *Settings) checked() bool {
	return s.Proj != nil || s.Org != nil || len(s.Contexts) > 0
}

// contextScope returns the restriction types of a context that limit it to a
// project or an expression, sorted and without duplicates. A group restriction
// does not count: one to the org's own ID means "All members".
func contextScope(restrictions []apiclient.ContextRestriction) []string {
	var types []string
	for _, r := range restrictions {
		scoped := r.RestrictionType == "project" || r.RestrictionType == "expression"
		if scoped && !slices.Contains(types, r.RestrictionType) {
			types = append(types, r.RestrictionType)
		}
	}
	slices.Sort(types)
	return types
}

// settingsFindings returns the findings the settings give rise to, each only
// when the setting it needs was read. It also lowers, in place, every offline
// context-on-any-branch finding for a context that is restricted to a project
// or an expression to low, noting why in the message: such a context is not
// handed to every branch of every project.
func settingsFindings(s *Settings, offline []Finding) []Finding {
	var out []Finding
	add := func(id, location, message string, sev Severity) {
		rule, _ := ruleByID(id)
		if sev == 0 {
			sev = rule.Severity
		}
		out = append(out, Finding{
			RuleID:     id,
			Severity:   sev,
			Confidence: rule.Confidence,
			Message:    message,
			Fix:        rule.Fix,
			Location:   location,
		})
	}

	if p := s.Proj; p != nil {
		if p.BuildForkPRs && p.CanPassSecretsToForkPR {
			add("fork-pr-secrets", "project settings",
				"fork PRs are built and receive project secrets and contexts (OAuth pipelines)", 0)
		}
		if !p.DisableSSH {
			add("ssh-rerun-allowed", "project settings",
				"jobs can be rerun with SSH, exposing their secrets to anyone who can rerun", 0)
		}
		if !p.IsAdminRequiredForWriting {
			add("settings-not-admin-only", "project settings",
				"project settings can be changed by members who are not admins", 0)
		}
	}

	if o := s.Org; o != nil {
		if !o.UserCheckoutKeysDisabled {
			add("user-checkout-keys-allowed", "org settings",
				"jobs can use a member's personal checkout key, which reaches every repository that member can", 0)
		}
		if o.UncertifiedPublicOrbs {
			sev := SeverityLow
			if slices.ContainsFunc(offline, func(f Finding) bool { return f.RuleID == "unpinned-orb" }) {
				sev = SeverityMedium
			}
			add("uncertified-orbs-allowed", "org settings",
				"uncertified public orbs can be used by jobs in this org", sev)
		}
	}

	names := make([]string, 0, len(s.Contexts))
	for name := range s.Contexts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		scope := contextScope(s.Contexts[name])
		if len(scope) == 0 {
			add("context-unrestricted", "context "+name,
				"context "+name+" has no project or expression restriction", 0)
			continue
		}
		for i := range offline {
			f := &offline[i]
			if f.RuleID == "context-on-any-branch" && f.context == name {
				f.Severity = SeverityLow
				f.Message += " (context is restricted: " + strings.Join(scope, ", ") + ")"
			}
		}
	}
	return out
}
