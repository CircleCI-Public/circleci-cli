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
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// KeyVerdict is one restore key, scoped from its trace.
type KeyVerdict struct {
	Key Key
	// Scope is the key's narrowest scoped part; Unscoped for a stable key.
	Scope Scope
	// Inspectable is false when some part of the key could not be judged;
	// Reason then says why.
	Inspectable bool
	// Reason says what gives the key its scope, e.g. `<< parameters.cache_key
	// >> is "{{ epoch }}" (workflows.baseline.jobs.0...)`.
	Reason string
	// Site is the authored node that makes the key what it is: the call site
	// supplying a parameter, or the authored key itself.
	Site []string
}

// part is one piece of an authored key: its literal text, or one reference.
type part struct {
	scope       Scope
	inspectable bool
	reason      string
	site        []string
}

// judgeKey scopes a key from its authored template and every value
// substituted into it (X1-static-v2). Built-in pipeline values are kept
// symbolic from the authored config: the compiler fills them in, so the
// compiled text alone cannot show them. A broken trace is uninspectable,
// never unscoped.
//
// The verdict is about the config as compiled with the given parameters. A
// value a trigger or setting can change, or known values that disagree (a
// matrix, an enum), make the key uninspectable.
func judgeKey(cfg *pipelineconfig.Effective, job string, k Key) KeyVerdict {
	v := KeyVerdict{Key: k}
	tr := cfg.Trace(job, k.Path)
	if tr.Problem != "" {
		v.Reason = tr.Problem
		return v
	}
	v.Site = tr.AuthoredPath
	ls, lok := textScope(stripRefs(tr.Template))
	parts := []part{{ls, lok, "the authored key " + strconv.Quote(tr.Template), tr.AuthoredPath}}
	for _, r := range tr.Refs {
		p := part{site: r.Source, inspectable: true}
		if len(p.site) == 0 {
			p.site = tr.AuthoredPath
		}
		src := strings.Join(r.Source, ".")
		via := "<< " + r.Expr + " >>"
		if r.Via != "" {
			via += " is << " + r.Via + " >>"
		}
		switch r.Pipeline {
		case pipelineconfig.PerPipeline:
			p.scope, p.reason = ScopePipeline, via+", the pipeline"
		case pipelineconfig.PerCommit:
			p.scope, p.reason = ScopeRevision, via+", the revision"
		case pipelineconfig.StablePipeline:
			p.scope, p.reason = Unscoped, via+", which is stable"
		case pipelineconfig.NotPipeline:
			p.scope, p.inspectable = valuesScope(r.Values)
			p.reason = via + " is " + strings.Join(quoteAll(r.Values), " or ")
		}
		if r.Via != "" || r.Pipeline == pipelineconfig.NotPipeline {
			p.reason += " (" + src + ")"
		}
		parts = append(parts, p)
	}
	// Any unknown part makes the key uninspectable (it could cancel a scoped
	// part); otherwise the narrowest scoped part decides.
	for _, p := range parts {
		if !p.inspectable {
			v.Reason = p.reason
			return v
		}
	}
	if why := boundaryCollision(tr); why != "" {
		v.Reason = why
		return v
	}
	v.Inspectable, v.Reason = true, "no part of the key is scoped"
	for _, p := range parts {
		if p.scope != Unscoped && (v.Scope == Unscoped || p.scope < v.Scope) {
			v.Scope, v.Reason, v.Site = p.scope, p.reason, p.site
		}
	}
	return v
}

// valuesScope scopes the values a reference can take. All must have the
// same scope, or all be unscoped; values that disagree depend on which one
// is used. Distinct scoped alternatives can also collide across runs
// ("{{ .BuildNum }}" in build 12 and "1{{ .BuildNum }}" in build 2 are both
// "12"), so they depend on the scenario too.
func valuesScope(values []string) (Scope, bool) {
	scopes := map[Scope]bool{}
	distinct := map[string]bool{}
	for _, val := range values {
		s, ok := textScope(val)
		if !ok {
			return Unscoped, false
		}
		scopes[s] = true
		distinct[val] = true
	}
	switch {
	case len(scopes) > 1, len(distinct) > 1 && !scopes[Unscoped]:
		return Unscoped, false
	}
	for s := range scopes {
		return s, true
	}
	return Unscoped, true
}

// stripRefs removes `<< … >>` references, leaving the authored literal text
// and its `{{ … }}` tokens.
func stripRefs(template string) string {
	var b strings.Builder
	for template != "" {
		before, rest, found := strings.Cut(template, "<<")
		b.WriteString(before)
		if !found {
			break
		}
		_, after, closed := strings.Cut(rest, ">>")
		if !closed {
			b.WriteString("{{ unclosed }}")
			break
		}
		template = after
	}
	return b.String()
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strconv.Quote(v)
	}
	return out
}

// PolicyVerdict is one restore policy, judged under RuleX1.
type PolicyVerdict struct {
	Policy *Step
	Keys   []KeyVerdict
	// Scope is the policy's broadest key: any fallback can provide that
	// reuse. Unscoped when some key is unscoped.
	Scope Scope
	// Volatile: every key is scoped, and the scope's claim holds. This is
	// the finding.
	Volatile bool
	// Uninspectable: some key could not be judged, or a key may match a save
	// of another shape. Reason says which.
	Uninspectable bool
	Reason        string
	// Saved: a save_cache sharing the key is in the config.
	Saved bool
	// Paths are what the matching saves store; PathClass classifies them.
	Paths     []string
	PathClass PathClass
}

func judgePolicy(cfg *pipelineconfig.Effective, m Model, p *Step) PolicyVerdict {
	pv := PolicyVerdict{Policy: p}
	for _, k := range p.Keys {
		kv := judgeKey(cfg, p.Job, k)
		pv.Keys = append(pv.Keys, kv)
	}
	for _, kv := range pv.Keys {
		switch {
		case !kv.Inspectable:
			pv.Uninspectable, pv.Reason = true, kv.Reason
			return pv
		case kv.Scope == Unscoped:
			// An unscoped fallback can reuse caches without limit.
			pv.Scope = Unscoped
			return pv
		case kv.Scope > pv.Scope:
			pv.Scope = kv.Scope
		}
	}
	// A scoped key proves only that the key is scoped, not that it cannot
	// match a save of another shape: restore_cache matches by prefix, and
	// compiled text can coincide ("deps-<< pipeline.number >>" is "deps-42",
	// like a hard-coded save "deps-42"). An edge keeps the claim only when
	// the scoped part is provably shared by the save.
	var saves []*Step
	for _, e := range m.EdgesOf(p) {
		if ok, why := correlated(cfg, p, e, pv.Keys[e.Key]); !ok {
			pv.Uninspectable, pv.Reason = true, why
			return pv
		}
		pv.Saved = true
		saves = append(saves, e.Save)
	}
	pv.Paths, pv.PathClass = savedPaths(cfg, saves)
	if pv.Scope.needsDependencyPaths() && pv.PathClass != DependencyPaths {
		pv.Reason = pv.Scope.Code() + " is reported only for dependency caches; the paths are " + pv.PathClass.String()
		return pv
	}
	pv.Volatile = true
	return pv
}

// correlated reports whether an edge still leaves the key's scope claim
// true. It holds when the compiled key has a scoped runtime token that the
// save has at the same place (the comparison is typed, so an unambiguous
// edge aligns it), or when the save's traced key starts with the restore
// key's authored template and the same traced values, so the scoped part is
// the same variable on both sides. Equal template text is not enough:
// "deps-<< parameters.k >>" with k = pipeline.git.revision on one side and
// pipeline.git.base_revision on the other are different variables.
func correlated(cfg *pipelineconfig.Effective, p *Step, e Edge, kv KeyVerdict) (bool, string) {
	save := e.Save.Keys[0]
	// The shortcut holds only when a runtime token gives the key its scope:
	// in "deps-<< pipeline.number >>-{{ .Revision }}" the shared revision
	// token does not prove the pipeline number is shared too.
	compiledScope, _ := textScope(p.Keys[e.Key].Raw)
	switch {
	case e.Ambiguous:
		return false, "key " + strconv.Itoa(e.Key) + " may match " + strconv.Quote(save.Raw) + " saved by job " + e.Save.Job
	case p.Keys[e.Key].changingToken() && compiledScope == kv.Scope:
		return true, ""
	}
	rt, st := cfg.Trace(p.Job, p.Keys[e.Key].Path), cfg.Trace(e.Save.Job, save.Path)
	if rt.Problem == "" && st.Problem == "" && strings.HasPrefix(normalRefs(st.Template), normalRefs(rt.Template)) &&
		len(st.Refs) >= len(rt.Refs) && sameRefs(rt.Refs, st.Refs[:len(rt.Refs)]) {
		return true, ""
	}
	return false, "key " + strconv.Itoa(e.Key) + " may match " + strconv.Quote(save.Raw) + " saved by job " + e.Save.Job +
		", whose key is not built from the same scoped value"
}

// refSpacing matches a reference however it is spaced.
var refSpacing = regexp.MustCompile(`<<\s*(.*?)\s*>>`)

// normalRefs spells every reference as "<< expr >>", so "<<parameters.k>>"
// and "<< parameters.k >>" compare equal.
func normalRefs(template string) string { return refSpacing.ReplaceAllString(template, "<< $1 >>") }

// sameRefs reports whether two traced reference lists resolve to the same
// values: the same expression, the same pipeline value, and the same
// literal values.
func sameRefs(a, b []pipelineconfig.TraceRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Expr != b[i].Expr || a[i].Pipeline != b[i].Pipeline || a[i].Via != b[i].Via || !slices.Equal(a[i].Values, b[i].Values) {
			return false
		}
	}
	return true
}

// JobVerdict is one job, judged under RuleF4.
type JobVerdict struct {
	Job string
	// Installs are "step N: npm ci" entries.
	Installs []string
	HasCache bool
	// Blocked says why an install job cannot be a candidate: a step that
	// might cache but cannot be inspected.
	Blocked []string
}

// Candidate reports an install, no visible cache, and nothing uninspectable.
func (j JobVerdict) Candidate() bool {
	return len(j.Installs) > 0 && !j.HasCache && len(j.Blocked) == 0
}

func judgeJob(cfg *pipelineconfig.Effective, job pipelineconfig.Job, mj Job) JobVerdict {
	jv := JobVerdict{Job: job.Name, HasCache: len(mj.Steps) > 0}
	if cfg.Setup {
		jv.Blocked = append(jv.Blocked, "the config is a setup (dynamic) config: the pipeline's real config is generated at runtime")
	}
	for _, i := range mj.UnknownSteps {
		jv.Blocked = append(jv.Blocked, fmt.Sprintf("step %d is of a type the model does not know, which might cache", i))
	}
	for _, st := range job.Steps() {
		if st.Type != "run" {
			continue
		}
		if w := pipelineconfig.MapGet(st.Body, "when"); w != nil && w.Value == "on_fail" {
			continue // runs only when the job fails
		}
		cmds, ok := commandsOf(st)
		if !ok {
			continue // unparseable: no install is claimed from it
		}
		for _, words := range cmds {
			if isFunctionRun(words) {
				// The function registry is empty, so every
				// function is uninspectable.
				jv.Blocked = append(jv.Blocked, fmt.Sprintf("step %d runs CircleCI function %q, which the registry has not reviewed", st.Index, strings.Join(words, " ")))
			}
			if name := installOf(words); name != "" {
				jv.Installs = append(jv.Installs, fmt.Sprintf("step %d: %s", st.Index, name))
			}
		}
	}
	return jv
}

// Analysis is the module's result for one config: the graph, every
// verdict, and the findings.
type Analysis struct {
	Model    Model
	Policies []PolicyVerdict
	Jobs     []JobVerdict
}

// Analyze builds the model and judges every restore policy and job.
func Analyze(cfg *pipelineconfig.Effective) Analysis {
	a := Analysis{Model: Build(cfg)}
	for _, p := range a.Model.Policies {
		a.Policies = append(a.Policies, judgePolicy(cfg, a.Model, p))
	}
	for i, job := range cfg.Jobs {
		a.Jobs = append(a.Jobs, judgeJob(cfg, job, a.Model.Jobs[i]))
	}
	return a
}

// width is how long a part of a key can be.
type width int

const (
	fixedWidth    width = iota // a non-digit literal, or a value of one length
	digitLiteral               // a literal digit, which extends an adjacent number
	variableDigit              // a decimal of any length: {{ .BuildNum }}, pipeline.number
	variableOther              // any other text of any length: {{ .Branch }}, several values
)

// boundaryCollision reports a variable-width number directly next to another
// variable-width value, with no literal between them. Two pipelines can then
// give the same key, so no scope claim holds: in
// "deps-<< pipeline.number >>{{ .Branch }}", pipeline 1 on branch "2fix" and
// pipeline 12 on branch "fix" are both "deps-12fix". Fixed-width values
// ({{ epoch }}, the revision, checksums, the pipeline and workflow ids) and
// any literal delimiter keep the parts apart.
func boundaryCollision(tr pipelineconfig.Trace) string {
	var widths []width
	refs := refSpacing.FindAllStringIndex(tr.Template, -1)
	prev := 0
	for ri, loc := range refs {
		widths = appendWidths(widths, tr.Template[prev:loc[0]])
		if r := tr.Refs[ri]; r.Pipeline == pipelineconfig.NotPipeline && len(r.Values) == 1 {
			// One literal value: its text is part of the key, so its tokens
			// sit next to their neighbours ("<< parameters.k >>" with k =
			// "{{ .BuildNum }}" is a number).
			widths = appendWidths(widths, r.Values[0])
		} else {
			widths = append(widths, refWidth(r))
		}
		prev = loc[1]
	}
	widths = appendWidths(widths, tr.Template[prev:])
	for i, w := range widths {
		if w != variableDigit {
			continue
		}
		// Literal digits run on from the number ("<< pipeline.number >>1" is
		// still a number), so look past them to the first other part.
		left, right := i-1, i+1
		for left >= 0 && widths[left] == digitLiteral {
			left--
		}
		for right < len(widths) && widths[right] == digitLiteral {
			right++
		}
		if (left >= 0 && widths[left] != fixedWidth) || (right < len(widths) && widths[right] != fixedWidth) {
			return "a variable-length number sits next to another variable value with no non-digit delimiter, so " +
				"two pipelines can give the same key (pipeline 1 on branch \"2fix\" and pipeline 12 on \"fix\")"
		}
	}
	return ""
}

// appendWidths adds the widths of text with no references: one fixed width
// per literal character, one per token.
func appendWidths(widths []width, text string) []width {
	for _, a := range atoms(text) {
		if a.tok == nil {
			if a.char >= '0' && a.char <= '9' {
				widths = append(widths, digitLiteral)
			} else {
				widths = append(widths, fixedWidth)
			}
			continue
		}
		widths = append(widths, tokenWidth(*a.tok))
	}
	return widths
}

func tokenWidth(t Token) width {
	switch t.Kind {
	case TokBuildNum:
		return variableDigit
	case TokBranch, TokArch, TokOther, TokEnv:
		if t.Kind == TokEnv && t.Arg == "CIRCLE_WORKFLOW_ID" {
			return fixedWidth
		}
		return variableOther
	case TokEpoch, TokRevision, TokChecksum:
	}
	return fixedWidth
}

func refWidth(r pipelineconfig.TraceRef) width {
	name := r.Expr
	if r.Via != "" {
		name = r.Via
	}
	switch {
	case name == "pipeline.number":
		return variableDigit
	case name == "pipeline.id" || name == "pipeline.git.revision":
		return fixedWidth
	case r.Pipeline != pipelineconfig.NotPipeline:
		return variableOther // the branch and other pipeline values
	}
	return variableOther // several values of different lengths
}
