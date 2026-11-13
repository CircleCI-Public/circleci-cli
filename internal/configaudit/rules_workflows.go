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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// stdinLabel is the file label of a config read from standard input, which has
// no place on disk to look next to.
const stdinLabel = "<stdin>"

const (
	contextFix = "add filters.branches.only, an approval job, or a restricted context"
	cacheFix   = "add {{ .Branch }} to the cache key so a branch cannot fill a cache that other branches restore"
	timeFix    = "build the key from {{ .Branch }} and a checksum of the dependency lockfile"
	dynamicFix = "continue with a config checked into the repository, or check what the step generates before continuing"
)

var (
	branchTemplateRE = regexp.MustCompile(`\{\{\s*\.Branch\s*\}\}`)
	templateRE       = regexp.MustCompile(`\{\{\s*([^}]*?)\s*\}\}`)
)

// wfEntry is one item of a workflow's jobs list, in the bare form
// (`- build`) or the single-key-map form (`- build: {...}`).
type wfEntry struct {
	workflow *yaml.Node // the workflow mapping the entry belongs to
	job      string     // the job or orb job it runs
	name     string     // what "requires" calls it: its name key, else the job
	body     *yaml.Node // the entry's parameters; nil when there are none
}

// workflowEntries lists every job entry of every workflow.
func (d *Doc) workflowEntries() []wfEntry {
	var out []wfEntry
	for _, wf := range mapPairs(d.workflows) {
		if wf[1] == nil || wf[1].Kind != yaml.MappingNode {
			continue
		}
		for _, item := range seqItems(mapGet(wf[1], "jobs")) {
			e := wfEntry{workflow: wf[1]}
			if item.Kind == yaml.ScalarNode {
				e.job = item.Value
			} else {
				pairs := mapPairs(item)
				if item.Kind != yaml.MappingNode || len(pairs) != 1 {
					continue
				}
				e.job = pairs[0][0].Value
				if b := pairs[0][1]; b != nil && b.Kind == yaml.MappingNode {
					e.body = b
				}
			}
			e.name = e.job
			if n := mapGet(e.body, "name"); n != nil && n.Kind == yaml.ScalarNode && n.Value != "" {
				e.name = n.Value
			}
			out = append(out, e)
		}
	}
	return out
}

// contexts returns the scalar nodes naming the contexts of the entry, whether
// context is a string or a list.
func (e wfEntry) contexts() []*yaml.Node {
	c := mapGet(e.body, "context")
	if c == nil {
		return nil
	}
	nodes := seqItems(c)
	if c.Kind == yaml.ScalarNode {
		nodes = []*yaml.Node{c}
	}
	return slices.DeleteFunc(nodes, func(n *yaml.Node) bool {
		return n.Kind != yaml.ScalarNode || n.Value == ""
	})
}

// requires returns the names the entry waits for.
func (e wfEntry) requires() []string {
	var names []string
	for _, item := range seqItems(mapGet(e.body, "requires")) {
		if item.Kind == yaml.ScalarNode {
			names = append(names, item.Value)
			continue
		}
		// requires: - build: [success]
		for _, kv := range mapPairs(item) {
			names = append(names, kv[0].Value)
		}
	}
	return names
}

// isApproval reports whether the entry is a manual approval gate.
func (e wfEntry) isApproval() bool {
	t := mapGet(e.body, "type")
	return t != nil && t.Kind == yaml.ScalarNode && t.Value == "approval"
}

// branchList returns the values of filters.branches.<key>, whether it is a
// string or a list.
func (e wfEntry) branchList(key string) []string {
	n := mapGet(mapGet(mapGet(e.body, "filters"), "branches"), key)
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		return []string{n.Value}
	}
	var out []string
	for _, item := range seqItems(n) {
		if item.Kind == yaml.ScalarNode {
			out = append(out, item.Value)
		}
	}
	return out
}

// mayRunOnDefaultBranch reports whether the entry can run on main or master:
// it has no branches.only, or that list names one, directly or by a regular
// expression.
func (e wfEntry) mayRunOnDefaultBranch() bool {
	only := e.branchList("only")
	if len(only) == 0 {
		return true
	}
	for _, pattern := range only {
		for _, branch := range []string{"main", "master"} {
			if branchMatches(pattern, branch) {
				return true
			}
		}
	}
	return false
}

// branchMatches reports whether a branch filter value matches branch. A value
// wrapped in slashes is a regular expression that must match the whole name; a
// pattern that does not compile is taken to match, so the rule errs towards
// flagging.
func branchMatches(pattern, branch string) bool {
	if len(pattern) >= 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		re, err := regexp.Compile(`^(?:` + pattern[1:len(pattern)-1] + `)$`)
		return err != nil || re.MatchString(branch)
	}
	return pattern == branch
}

// tagOnly reports whether branches.ignore keeps the entry off every branch.
func (e wfEntry) tagOnly() bool {
	return slices.Contains(e.branchList("ignore"), "/.*/")
}

// approvalUpstream reports whether an approval job is reachable from e through
// requires, transitively, within its workflow.
func approvalUpstream(e wfEntry, group []wfEntry) bool {
	byName := map[string]wfEntry{}
	for _, other := range group {
		byName[other.name] = other
	}
	visited := map[string]bool{}
	queue := e.requires()
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		up, ok := byName[name]
		if !ok {
			continue
		}
		if up.isApproval() {
			return true
		}
		queue = append(queue, up.requires()...)
	}
	return false
}

// checkContextOnAnyBranch flags workflow jobs that get a context on every
// branch, so a context's secrets reach any branch's build.
func checkContextOnAnyBranch(d *Doc, emit func(Finding)) {
	entries := d.workflowEntries()
	groups := map[*yaml.Node][]wfEntry{}
	for _, e := range entries {
		groups[e.workflow] = append(groups[e.workflow], e)
	}
	for _, e := range entries {
		contexts := e.contexts()
		if len(contexts) == 0 || len(e.branchList("only")) > 0 || e.tagOnly() ||
			mapGet(e.workflow, "when") != nil || mapGet(e.workflow, "unless") != nil ||
			approvalUpstream(e, groups[e.workflow]) {
			continue
		}
		for _, c := range contexts {
			pos := locate(c, d.lines, 0, len(c.Value))
			emit(Finding{
				RuleID:   "context-on-any-branch",
				Message:  "job " + e.job + " gets context " + c.Value + " on every branch",
				Fix:      contextFix,
				Position: &pos,
				context:  c.Value,
			})
		}
	}
}

// walkExpanded is walkSteps that also walks into the commands the steps invoke.
func (d *Doc) walkExpanded(n *yaml.Node, fn func(step)) {
	d.walkExpandedVisiting(n, fn, map[string]bool{})
}

func (d *Doc) walkExpandedVisiting(n *yaml.Node, fn func(step), visiting map[string]bool) {
	walkSteps(n, func(s step) {
		fn(s)
		if cmd := mapGet(d.commands, s.name); cmd != nil && !visiting[s.name] {
			visiting[s.name] = true
			d.walkExpandedVisiting(mapGet(cmd, "steps"), fn, visiting)
			delete(visiting, s.name)
		}
	})
}

// cacheFamily is the literal part of a cache key before its first template.
func cacheFamily(key string) string {
	prefix, _, _ := strings.Cut(key, "{{")
	return prefix
}

// cacheKeys returns the key scalars of a save_cache or restore_cache step.
func cacheKeys(s step) []*yaml.Node {
	if s.body == nil || s.body.Kind != yaml.MappingNode {
		return nil
	}
	var out []*yaml.Node
	if k := mapGet(s.body, "key"); k != nil && k.Kind == yaml.ScalarNode {
		out = append(out, k)
	}
	for _, k := range seqItems(mapGet(s.body, "keys")) {
		if k.Kind == yaml.ScalarNode {
			out = append(out, k)
		}
	}
	return out
}

// exposure says why a job's workflow entries put its cache at risk: the job
// gets a context, or may run on the default branch. It is empty when neither.
func exposure(entries []wfEntry) string {
	for _, e := range entries {
		if len(e.contexts()) > 0 {
			return "gets a context"
		}
	}
	for _, e := range entries {
		if e.mayRunOnDefaultBranch() {
			return "may run on the default branch"
		}
	}
	return ""
}

// checkCachePoisoning flags a save_cache key with no {{ .Branch }} whose family
// is restored by a job that has secrets or runs on the default branch, so a
// build of any branch could plant files in that job's cache. It also notes
// keys built on a predictable time window.
func checkCachePoisoning(d *Doc, emit func(Finding)) {
	byJob := map[string][]wfEntry{}
	for _, e := range d.workflowEntries() {
		byJob[e.job] = append(byJob[e.job], e)
	}

	// restoredBy maps a key family to the exposed jobs that restore it, with why.
	type restorer struct{ job, why string }
	restoredBy := map[string][]restorer{}
	for _, o := range d.owners("job") {
		why := exposure(byJob[o.name])
		if why == "" {
			continue
		}
		d.walkExpanded(mapGet(o.node, "steps"), func(s step) {
			if s.name != "restore_cache" {
				return
			}
			for _, k := range cacheKeys(s) {
				fam := cacheFamily(k.Value)
				if !slices.ContainsFunc(restoredBy[fam], func(r restorer) bool { return r.job == o.name }) {
					restoredBy[fam] = append(restoredBy[fam], restorer{job: o.name, why: why})
				}
			}
		})
	}

	for _, o := range d.owners("job", "command") {
		walkSteps(mapGet(o.node, "steps"), func(s step) {
			if s.name != "save_cache" || s.body == nil || s.body.Kind != yaml.MappingNode {
				return
			}
			key := mapGet(s.body, "key")
			if key == nil || key.Kind != yaml.ScalarNode || branchTemplateRE.MatchString(key.Value) {
				return
			}
			if rs := restoredBy[cacheFamily(key.Value)]; len(rs) > 0 {
				pos := locate(key, d.lines, 0, len(key.Value))
				emit(Finding{
					RuleID: "cache-poisoning",
					Message: "cache key " + key.Value + " has no {{ .Branch }} and is restored by job " +
						rs[0].job + ", which " + rs[0].why,
					Fix:      cacheFix,
					Position: &pos,
				})
			}
			if off, length, ok := predictableTemplate(key.Value); ok {
				pos := locate(key, d.lines, off, length)
				emit(Finding{
					RuleID:     "cache-poisoning",
					Severity:   SeverityLow,
					Confidence: ConfidenceLow,
					Message:    "cache key " + key.Value + " is built on a predictable time window",
					Fix:        timeFix,
					Position:   &pos,
				})
			}
		})
	}
}

// predictableTemplate finds the template that makes a key predictable: the
// first {{ epoch }}, or the first {{ arch }} when every template of the key is
// {{ arch }}. off and length are a byte range of key.
func predictableTemplate(key string) (off, length int, ok bool) {
	matches := templateRE.FindAllStringSubmatchIndex(key, -1)
	if len(matches) == 0 {
		return 0, 0, false
	}
	for _, m := range matches {
		if key[m[2]:m[3]] == "epoch" {
			return m[0], m[1] - m[0], true
		}
	}
	for _, m := range matches {
		if key[m[2]:m[3]] != "arch" {
			return 0, 0, false
		}
	}
	return matches[0][0], matches[0][1] - matches[0][0], true
}

// pathParams are the parameters of an orb step or job that name a config file
// the pipeline continues with.
func pathParams(name string) []string {
	switch {
	case strings.HasPrefix(name, "path-filtering/"):
		return []string{"configuration_path", "mapping", "config-path"}
	case strings.Contains(name, "/"):
		return []string{"configuration_path"}
	}
	return nil
}

// setupTrue reports whether the config is a setup config: top-level setup: true.
func (d *Doc) setupTrue() bool {
	v := mapGet(d.root, "setup")
	return v != nil && v.Kind == yaml.ScalarNode && v.Value == "true"
}

// checkDynamicConfig flags, in a setup config, a continuation config path that
// a run step writes, or that is not a checked-in file next to the audited
// config: what the pipeline continues with is then not what was reviewed.
func checkDynamicConfig(d *Doc, emit func(Finding)) {
	if !d.setupTrue() {
		return
	}
	report := func(param string, val *yaml.Node, runs []*yaml.Node) {
		if val == nil || val.Kind != yaml.ScalarNode {
			return
		}
		path := val.Value
		if path == "" || strings.ContainsAny(path, "\n") || strings.Contains(path, "<<") {
			return
		}
		if param == "mapping" && strings.ContainsAny(path, " \t") {
			return // a mapping table of "path regex parameter value" lines, not a path
		}
		var msg string
		switch {
		case slices.ContainsFunc(runs, func(r *yaml.Node) bool { return writesPath(r.Value, path) }):
			msg = param + " " + path + " is written by a run step, so the config it continues with is generated"
		case !d.existsNextToConfig(path):
			msg = param + " " + path + " is not a file checked in next to the audited config"
		default:
			return
		}
		pos := locate(val, d.lines, 0, len(val.Value))
		emit(Finding{RuleID: "dynamic-config-untrusted", Message: msg, Fix: dynamicFix, Position: &pos})
	}

	for _, o := range d.owners("job", "command") {
		var runs []*yaml.Node
		d.walkExpanded(mapGet(o.node, "steps"), func(s step) {
			if cmd := runCommand(s); cmd != nil {
				runs = append(runs, cmd)
			}
		})
		walkSteps(mapGet(o.node, "steps"), func(s step) {
			if s.body == nil || s.body.Kind != yaml.MappingNode {
				return
			}
			for _, param := range pathParams(s.name) {
				report(param, mapGet(s.body, param), runs)
			}
		})
	}
	for _, e := range d.workflowEntries() {
		for _, param := range pathParams(e.job) {
			report(param, mapGet(e.body, param), nil)
		}
	}
}

// writesPath reports whether shell text writes to path: a redirect, tee or -o.
func writesPath(shell, path string) bool {
	clean := strings.TrimPrefix(path, "./")
	re := regexp.MustCompile(`(?m)(?:>>?\s*|\btee\s+(?:-\w+\s+)*|(?:^|\s)-o\s*)(?:\./)?['"]?` +
		regexp.QuoteMeta(clean) + `['"]?(?:$|[\s;&|)])`)
	return re.MatchString(shell)
}

// existsNextToConfig reports whether path is a file beside the audited config
// or at the repository root above it. It is true for a config read from stdin,
// which has no place on disk to look.
func (d *Doc) existsNextToConfig(path string) bool {
	if d.file == stdinLabel {
		return true
	}
	dir := filepath.Dir(d.file)
	for _, p := range []string{filepath.Join(dir, "..", path), filepath.Join(dir, path)} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// parameterisedContexts returns the context names the workflow jobs build from
// a parameter (<< ... >>), sorted and without duplicates: the names Contexts
// leaves out.
func (d *Doc) parameterisedContexts() []string {
	names := map[string]bool{}
	for _, e := range d.workflowEntries() {
		for _, c := range e.contexts() {
			if strings.Contains(c.Value, "<<") {
				names[c.Value] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(names))
}

// Contexts returns the context names the workflow jobs use, sorted and without
// duplicates. A name built from a parameter (<< ... >>) cannot be resolved
// offline and is left out.
func (d *Doc) Contexts() []string {
	names := map[string]bool{}
	for _, e := range d.workflowEntries() {
		for _, c := range e.contexts() {
			if !strings.Contains(c.Value, "<<") {
				names[c.Value] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(names))
}
