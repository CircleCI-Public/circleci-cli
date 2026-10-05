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

package cache

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// The cache key fix: replace {{ epoch }} with a checksum of the
// install's lockfile, edited once at the key's source so the restore and the
// save change together.

// Fix is the edit for one volatile restore policy, or why there is none.
type Fix struct {
	// Source is the authored node to edit, Old its value, New the value.
	Source   []string
	Old, New string
	// Lockfile is the file the new key checksums, relative to the
	// working directory.
	Lockfile string
	// Compiled are the compiled keys the edit changes, with their new text.
	Compiled map[string]string
	// Suggested is the key to use when no edit is made, and Why says why
	// there is none ("needs a lockfile", …). Empty when Source is set.
	Suggested, Why string
}

var epochToken = regexp.MustCompile(`\{\{\s*epoch\s*\}\}`)

// planFix decides the edit for a TIME_SCOPED policy. Every condition the
// owner set must hold, or there is no edit:
//   - the key's {{ epoch }} comes from a parameter at a workflow call site,
//     so one edit at that source changes the restore and the save together;
//   - the cache stores package-manager dependencies (the dependency list);
//   - exactly one lockfile is read by an install command in the job, is
//     present at that path in the repo relative to working_directory, and
//     is rewritten neither by the install nor by an earlier step.
func planFix(cfg *pipelineconfig.Effective, m Model, pv PolicyVerdict, repo string) Fix {
	var f Fix
	kv := pv.Keys[0]
	if len(pv.Keys) != 1 || pv.Scope != ScopeTime {
		f.Why = "only a single {{ epoch }} key is fixed"
		return f
	}
	if pv.PathClass != DependencyPaths {
		f.Why = "the cache stores " + pv.PathClass.String() + "; only dependency caches are keyed on a lockfile"
		return f
	}
	lock, why := lockfileOf(cfg, pv.Policy.Job, repo)
	f.Suggested = epochToken.ReplaceAllString(kv.Key.Raw, `{{ checksum "`+orPlaceholder(lock)+`" }}`)
	if why != "" {
		f.Why = why
		return f
	}
	tr := cfg.Trace(pv.Policy.Job, kv.Key.Path)
	src, old := epochSource(tr)
	if src == nil {
		f.Why = "the {{ epoch }} is not set by one parameter at a workflow call site, so it is not edited in one place"
		return f
	}
	f.Source, f.Old, f.Lockfile = src, old, lock
	f.New = epochToken.ReplaceAllString(old, `{{ checksum "`+lock+`" }}`)
	// Every compiled key built from that source changes the same way.
	f.Compiled = map[string]string{}
	for _, s := range append(append([]*Step{}, m.Policies...), m.Saves...) {
		if s.Job != pv.Policy.Job {
			continue
		}
		for _, k := range s.Keys {
			if t := cfg.Trace(s.Job, k.Path); t.Problem == "" && usesSource(t, src) {
				f.Compiled[strings.Join(k.Path, "\x00")] = epochToken.ReplaceAllString(k.Raw, `{{ checksum "`+lock+`" }}`)
			}
		}
	}
	return f
}

func orPlaceholder(lock string) string {
	if lock == "" {
		return "<lockfile>"
	}
	return lock
}

// epochSource returns the call-site parameter whose value holds the key's
// {{ epoch }}, and that value.
func epochSource(tr pipelineconfig.Trace) ([]string, string) {
	for _, r := range tr.Refs {
		if r.Pipeline != pipelineconfig.NotPipeline || len(r.Values) != 1 || !epochToken.MatchString(r.Values[0]) {
			continue
		}
		if len(r.Source) > 0 && r.Source[0] == "workflows" {
			return r.Source, r.Values[0]
		}
	}
	return nil, ""
}

func usesSource(t pipelineconfig.Trace, src []string) bool {
	for _, r := range t.Refs {
		if slices.Equal(r.Source, src) {
			return true
		}
	}
	return false
}

// lockfileOf finds the one lockfile an install in the job reads. It returns
// the path relative to working_directory, or why there is none.
func lockfileOf(cfg *pipelineconfig.Effective, job, repo string) (string, string) {
	var j pipelineconfig.Job
	for _, x := range cfg.Jobs {
		if x.Name == job {
			j = x
		}
	}
	if wd := j.Get("working_directory"); wd != nil && wd.Value != "~/project" && wd.Value != "." {
		return "", "needs a lockfile: the job's working_directory is " + wd.Value + ", which config optimize does not map to the repo"
	}
	if repo == "" {
		return "", "needs a lockfile: the repo is not available to check it"
	}
	var found []string
	var reasons []string
	var earlier []string // commands of steps before the install
	for _, st := range j.Steps() {
		if st.Type != "run" {
			continue
		}
		cmds, ok := commandsOf(st)
		if !ok {
			continue
		}
		dir := "."
		for _, words := range cmds {
			w := stripWrappers(words)
			if startsWith(w, "cd") && len(w) > len([]string{"cd"}) {
				dir = path.Clean(path.Join(dir, w[len(w)-1]))
				continue
			}
			lock, why := installLockfile(w)
			switch {
			case why != "":
				reasons = append(reasons, why)
			case lock != "":
				p := path.Clean(path.Join(dir, lock))
				if slices.ContainsFunc(earlier, func(c string) bool { return strings.Contains(c, path.Base(p)) }) {
					reasons = append(reasons, "an earlier step mentions "+path.Base(p)+", so it may rewrite it")
					continue
				}
				if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(p))); err != nil {
					reasons = append(reasons, p+" is not in the repo")
					continue
				}
				if !slices.Contains(found, p) {
					found = append(found, p)
				}
			}
		}
		earlier = append(earlier, commandText(st))
	}
	switch {
	case len(found) == 1 && len(reasons) == 0:
		return found[0], ""
	case len(found) > 1:
		return "", "needs a lockfile: the job reads more than one (" + strings.Join(found, ", ") + ")"
	case len(reasons) > 0:
		return "", "needs a lockfile: " + reasons[0]
	}
	return "", "needs a lockfile: no install in the job reads one"
}

// installLockfile names the lockfile an install command reads, or why it
// cannot be used: the install may rewrite it, or the package manager has none.
func installLockfile(w []string) (string, string) {
	has := func(flags ...string) bool {
		return slices.ContainsFunc(w, func(x string) bool { return slices.Contains(flags, x) })
	}
	switch installOf(w) {
	case "pip install -r":
		for i, x := range w {
			if x == "-r" && i+1 < len(w) {
				return w[i+1], ""
			}
			if v, ok := strings.CutPrefix(x, "--requirement="); ok {
				return v, ""
			}
		}
	case "npm ci":
		return "package-lock.json", ""
	case "yarn install":
		if has("--frozen-lockfile", "--immutable") {
			return "yarn.lock", ""
		}
		return "", "yarn install without --frozen-lockfile can rewrite yarn.lock"
	case "pnpm install":
		if has("--frozen-lockfile") {
			return "pnpm-lock.yaml", ""
		}
		return "", "pnpm install without --frozen-lockfile can rewrite pnpm-lock.yaml"
	case "poetry install":
		return "poetry.lock", ""
	case "bundle install":
		if has("--frozen", "--deployment") {
			return "Gemfile.lock", ""
		}
		return "", "bundle install without --frozen can rewrite Gemfile.lock"
	case "go mod download":
		return "go.sum", ""
	case "cargo fetch", "cargo build":
		if has("--locked", "--frozen") {
			return "Cargo.lock", ""
		}
		return "", "cargo without --locked can write Cargo.lock"
	case "mvn":
		return "", "Maven has no lockfile"
	}
	return "", ""
}

func commandText(st pipelineconfig.Step) string {
	if st.Body == nil {
		return ""
	}
	if c := pipelineconfig.MapGet(st.Body, "command"); c != nil {
		return c.Value
	}
	return st.Body.Value
}
