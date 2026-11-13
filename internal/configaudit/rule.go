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

// Rule is one catalog entry.
type Rule struct {
	ID         string
	Severity   Severity // default
	Confidence Confidence
	Short      string // one line: what it catches
	Why        string
	Fix        string
	// Online marks a rule that audits settings read with --online rather than
	// the config file.
	Online bool
}

// checker finds one rule's problems in a Doc and reports each through emit.
// A checker sets a Finding's RuleID, Message and Position; Audit fills in the
// rest from the catalog unless the checker has already set it.
type checker func(d *Doc, emit func(Finding))

// catalog lists the rules in the order they run and are documented.
var catalog = []Rule{
	{
		ID:         "pipeline-value-injection",
		Severity:   SeverityHigh,
		Confidence: ConfidenceHigh,
		Short:      "Pipeline value an outsider controls is expanded into a shell command",
		Why:        "Values such as the branch name, tag or commit message are chosen by whoever pushes or opens a pull request. Expanded into a run step before the shell reads it, they become shell syntax, so a crafted branch name runs commands with the job's secrets.",
		Fix:        `pass it through environment: (BRANCH: << pipeline.git.branch >>) and use "$BRANCH"`,
	},
	{
		ID:         "unpinned-orb",
		Severity:   SeverityMedium,
		Confidence: ConfidenceHigh,
		Short:      "Orb reference is not pinned to an exact version",
		Why:        "A major-version alias, no version or @volatile lets the orb publisher change the code your jobs run, with your secrets, without a change to your config.",
		Fix:        "pin the orb to an exact version (name@X.Y.Z)",
	},
	{
		ID:         "unpinned-image",
		Severity:   SeverityMedium,
		Confidence: ConfidenceHigh,
		Short:      "Docker image is not pinned to a digest",
		Why:        "A tag can be moved to different content at any time, so a job can silently run an image you never reviewed. :latest and a missing tag are the most exposed.",
		Fix:        "pin the image to a digest (image:tag@sha256:<digest>)",
	},
	{
		ID:         "cache-poisoning",
		Severity:   SeverityMedium,
		Confidence: ConfidenceLow,
		Short:      "A cache any branch can fill is restored by a job with secrets or on the default branch",
		Why:        "Without {{ .Branch }} in the key, a build of any branch, including one from an outside contributor, can save a cache that the default branch or a job with a context later restores, so planted files run with its secrets.",
		Fix:        cacheFix,
	},
	{
		ID:         "ssh-keys-unrestricted",
		Severity:   SeverityMedium,
		Confidence: ConfidenceHigh,
		Short:      "add_ssh_keys does not name the keys to add",
		Why:        "Without fingerprints, every SSH key added to the project is loaded into the job, so a compromised step can use keys the job never needed.",
		Fix:        sshFix,
	},
	{
		ID:         "curl-pipe-shell",
		Severity:   SeverityMedium,
		Confidence: ConfidenceMedium,
		Short:      "A downloaded script is piped straight into a shell",
		Why:        "The script runs unreviewed and unverified, so whoever controls the server, or a network position in between, runs code in your job with its secrets.",
		Fix:        curlFix,
	},
	{
		ID:         "hardcoded-secret",
		Severity:   SeverityHigh,
		Confidence: ConfidenceMedium,
		Short:      "A secret is written into the config as a literal value",
		Why:        "The config is readable by everyone with access to the repository and lives forever in its history, so a literal token or password there has to be treated as leaked.",
		Fix:        secretFix,
	},
	{
		ID:         "context-on-any-branch",
		Severity:   SeverityMedium,
		Confidence: ConfidenceMedium,
		Short:      "A workflow job gets a context on every branch",
		Why:        "A context's secrets are handed to the job on whichever branch runs it, so anyone who can push a branch can write a config change that reads them.",
		Fix:        contextFix,
	},
	{
		ID:         "dynamic-config-untrusted",
		Severity:   SeverityLow,
		Confidence: ConfidenceLow,
		Short:      "A setup workflow continues with a config that is generated, or not checked in",
		Why:        "The config reviewed is a setup config for a dynamic pipeline. Review the config used to continue the pipeline.",
		Fix:        dynamicFix,
	},
	{
		ID:         "fork-pr-secrets",
		Severity:   SeverityHigh,
		Confidence: ConfidenceHigh,
		Short:      "Fork pull requests are built and receive project secrets",
		Why:        "With both \"Build forked pull requests\" and \"Pass secrets to builds from forked pull requests\" on, anyone who can open a pull request from a fork can change the config of their copy and read the project's environment variables and contexts. This applies to OAuth pipelines; GitHub App pipelines never build fork pull requests.",
		Fix:        `turn off "Pass secrets to builds from forked pull requests" in Project Settings > Advanced`,
		Online:     true,
	},
	{
		ID:         "ssh-rerun-allowed",
		Severity:   SeverityMedium,
		Confidence: ConfidenceHigh,
		Short:      "Jobs can be rerun with SSH",
		Why:        "Anyone who can rerun a job can open a shell in it, with every secret the job holds, so SSH reruns widen who can read the project's secrets.",
		Fix:        "disable SSH reruns for this project",
		Online:     true,
	},
	{
		ID:         "settings-not-admin-only",
		Severity:   SeverityLow,
		Confidence: ConfidenceHigh,
		Short:      "Project settings can be changed by non-admins",
		Why:        "Settings such as the fork pull request options and SSH reruns decide who can reach the project's secrets. Letting any member change them lets any member loosen them.",
		Fix:        "require admin access to change this project's settings",
		Online:     true,
	},
	{
		ID:         "user-checkout-keys-allowed",
		Severity:   SeverityLow,
		Confidence: ConfidenceHigh,
		Short:      "User checkout keys are allowed in the org",
		Why:        "A user key acts as the person who created it, so a job using one can reach every repository that person can, well beyond the project it runs for.",
		Fix:        "disable user checkout keys in the org's settings",
		Online:     true,
	},
	{
		ID:         "uncertified-orbs-allowed",
		Severity:   SeverityLow,
		Confidence: ConfidenceHigh,
		Short:      "Uncertified public orbs are allowed in the org",
		Why:        "Any published orb can then run in your jobs, with their secrets, although CircleCI has not reviewed it. The finding is medium when the config also has an unpinned orb.",
		Fix:        "turn off uncertified public orbs in the org's settings, and pin every orb to an exact version",
		Online:     true,
	},
	{
		ID:         "context-unrestricted",
		Severity:   SeverityMedium,
		Confidence: ConfidenceHigh,
		Short:      "A context the config uses has no project or expression restriction",
		Why:        "A context with no restriction, or only one for a group such as All members, is open to every branch of every project in the org, and does not stop fork pull requests from receiving it.",
		Fix:        "restrict the context to this project or to an expression such as the default branch",
		Online:     true,
	},
}

// checkers holds each rule's implementation by id.
var checkers = map[string]checker{
	"pipeline-value-injection": checkInjection,
	"unpinned-orb":             checkUnpinnedOrb,
	"unpinned-image":           checkUnpinnedImage,
	"ssh-keys-unrestricted":    checkSSHKeys,
	"curl-pipe-shell":          checkCurlPipeShell,
	"hardcoded-secret":         checkHardcodedSecret,
	"cache-poisoning":          checkCachePoisoning,
	"context-on-any-branch":    checkContextOnAnyBranch,
	"dynamic-config-untrusted": checkDynamicConfig,
}

// Catalog returns every rule, in catalog order.
func Catalog() []Rule {
	return append([]Rule(nil), catalog...)
}

func ruleByID(id string) (Rule, bool) {
	for _, r := range catalog {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}
