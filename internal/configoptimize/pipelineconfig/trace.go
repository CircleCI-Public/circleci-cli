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

package pipelineconfig

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineValue says how a `<< pipeline.* >>` value behaves across
// pipelines.
type PipelineValue int

// Pipeline values: NotPipeline is not a pipeline value; PerPipeline changes
// with every pipeline (number, id); PerCommit changes with every commit but
// is shared by pipelines on the same commit (revision); StablePipeline is
// the same for every pipeline on the same branch or project.
const (
	NotPipeline PipelineValue = iota
	PerPipeline
	PerCommit
	StablePipeline
)

// TraceCause says why a trace is broken, so an uninspectable result says
// which kind of unknown it is.
type TraceCause int

// Causes: CauseProvenance means the authored origin of the value could not
// be located; CauseExternal means the value is set outside the config (a
// trigger, a context, the project); CauseUnsupported means the config uses a
// construct the tracer does not follow yet.
const (
	CauseNone TraceCause = iota
	CauseProvenance
	CauseExternal
	CauseUnsupported
)

func (c TraceCause) String() string {
	switch c {
	case CauseProvenance:
		return "trace failed"
	case CauseExternal:
		return "value not available"
	case CauseUnsupported:
		return "not supported yet"
	case CauseNone:
	}
	return ""
}

// causeOf classifies a Problem. Every message the tracer produces is listed
// here; TestTraceCauses checks that each class stays classified.
func causeOf(problem string) TraceCause {
	for _, c := range []struct {
		cause TraceCause
		parts []string
	}{
		{CauseExternal, []string{"can be overridden when the pipeline is triggered", "has no enum list", "setup (dynamic) config"}},
		{CauseUnsupported, []string{
			"anchor", "merge key", "pre-steps or post-steps", "comes from an orb", "is not a value the tracer knows",
			"is itself a reference", "a reference that cannot be traced", "parameter expression", "is not a string",
			"the same for sibling commits",
			"is not a list", "is not a mapping or list", "no plain authored steps list", "only values inside a job's steps",
		}},
		{CauseProvenance, []string{
			"no authored config", "more than one workflow invocation", "no authored job produces", "is not defined in the authored config",
			"cannot be matched to one authored step", "is not in the authored config", "does not give the compiled text",
			"has no call-site value and no default", "is not declared", "is not a number",
		}},
	} {
		for _, part := range c.parts {
			if strings.Contains(problem, part) {
				return c.cause
			}
		}
	}
	return CauseProvenance
}

// TraceRef is one `<< … >>` reference in an authored value, and what it can
// be.
type TraceRef struct {
	// Expr is the reference, e.g. "parameters.cache_key".
	Expr string
	// Values are every value the reference can take, as text. Nil for a
	// pipeline value, whose text is not in the config.
	Values []string
	// Pipeline is set when the reference is, or resolves to, a pipeline
	// value.
	Pipeline PipelineValue
	// Via is the pipeline value a parameter resolves to, e.g.
	// "pipeline.git.revision" for a call site passing
	// "<< pipeline.git.revision >>". Empty otherwise.
	Via string
	// Source is the authored path of the node that supplies the value: a
	// workflow call site, a parameter default, or a matrix list.
	Source []string
	// matrix is set for a matrix parameter: the compiled job is one variant,
	// so Values are narrowed to the ones that reproduce it.
	matrix bool
}

// Trace is where a compiled scalar came from in the authored config, with
// every substitution in it resolved. A Trace with a Problem is broken: the
// value must be treated as uninspectable, never as stable.
type Trace struct {
	// AuthoredPath is the authored node holding the template.
	AuthoredPath []string
	// Template is the authored text, before substitution.
	Template string
	Refs     []TraceRef
	Problem  string
}

// Cause classifies the Problem; CauseNone for a complete trace.
func (t Trace) Cause() TraceCause {
	if t.Problem == "" {
		return CauseNone
	}
	return causeOf(t.Problem)
}

// refPattern matches one `<< … >>` reference.
var refPattern = regexp.MustCompile(`<<\s*([^<>]*?)\s*>>`)

// perPipeline and stablePipeline are the pipeline values whose behaviour is
// known. Anything else is untraceable.
var (
	perPipeline = []string{"pipeline.number", "pipeline.id"}
	perCommit   = []string{"pipeline.git.revision"}
	// sharedBase can be the same for sibling commits (two branches from one
	// base), so it proves neither per-commit change nor stability.
	sharedBase     = "pipeline.git.base_revision"
	stablePipeline = []string{"pipeline.git.branch", "pipeline.project.git_url", "pipeline.project.type"}
)

// Trace follows the compiled scalar at effPath of effective job job back to
// the authored config. It resolves call-site parameters,
// parameter defaults, matrix values, enum pipeline parameters and pipeline
// values, and checks that substituting them reproduces the compiled text.
// Only builtin steps in a job's own steps list are followed; anything else is
// a Problem.
func (m *Mutability) Trace(job string, effPath []string, effValue *yaml.Node) Trace {
	broken := func(why string) Trace { return Trace{Problem: why} }
	if m == nil || m.root == nil {
		return broken("no authored config")
	}
	if setup := MapGet(m.root, "setup"); setup != nil && IsTrue(setup) {
		return broken("the config is a setup (dynamic) config: the pipeline's real config is generated at runtime")
	}
	if len(effPath) < 5 || effPath[0] != "jobs" || effPath[1] != job || effPath[2] != "steps" {
		return broken("only values inside a job's steps are traced")
	}
	ref, inv, why := m.invocationOf(job)
	if why != "" {
		return broken(why)
	}
	jobNode := MapGet(m.jobs, ref)
	if jobNode == nil {
		return broken("job " + ref + " is not defined in the authored config")
	}
	if m.shared(m.jobValueNode(ref)) || hasMergeKey(jobNode) {
		return broken("job " + ref + " is shared through an anchor, alias or merge key")
	}
	if inv != nil && inv.wraps {
		return broken("job " + ref + " is invoked with pre-steps or post-steps, which shift step indexes")
	}
	n, path, why := m.authoredStepValue(job, ref, jobNode, effPath)
	if why != "" {
		return broken(why)
	}
	if m.shared(n) {
		return broken(strings.Join(path, ".") + " is an anchor or alias")
	}
	n = deref(n)
	if n.Kind != yaml.ScalarNode || effValue == nil || effValue.Kind != yaml.ScalarNode {
		return broken(strings.Join(path, ".") + " is not a string")
	}
	t := Trace{AuthoredPath: path, Template: n.Value}
	for _, match := range refPattern.FindAllStringSubmatch(n.Value, -1) {
		r, why := m.resolveRef(match[1], ref, inv)
		if why != "" {
			return broken(why)
		}
		t.Refs = append(t.Refs, r)
	}
	narrowMatrix(&t, effValue.Value)
	if !reproduces(t, effValue.Value) {
		return broken(strings.Join(path, ".") + ": substituting the traced values does not give the compiled text")
	}
	return t
}

// invocationOf finds the authored job and the workflow invocation that
// produce effective job job. The invocation is nil for a job no workflow
// renames or parameterizes.
func (m *Mutability) invocationOf(job string) (string, *invocation, string) {
	var found []invocation
	for _, ref := range slices.Sorted(maps.Keys(m.invocations)) {
		for _, inv := range m.invocations[ref] {
			if (!inv.matrix && inv.name == job) || (inv.matrix && m.matrixProduces(inv, job)) {
				found = append(found, inv)
			}
		}
	}
	switch {
	case len(found) > 1:
		return "", nil, "more than one workflow invocation could produce job " + job
	case len(found) == 1:
		if m.isOrbRef(found[0].ref) {
			return "", nil, "job " + job + " comes from an orb"
		}
		return found[0].ref, &found[0], ""
	case m.isOrbRef(job):
		return "", nil, "job " + job + " comes from an orb"
	case MapGet(m.jobs, job) != nil:
		return job, nil, ""
	}
	for _, invs := range m.invocations {
		for _, inv := range invs {
			if inv.body != nil && hasMergeKey(inv.body) {
				return "", nil, "no authored job produces effective job " + job +
					" that the tracer can see: a workflow entry uses a YAML merge key, which it does not follow"
			}
		}
	}
	return "", nil, "no authored job produces effective job " + job
}

// resolveRef resolves one reference used in authored job ref.
func (m *Mutability) resolveRef(expr, ref string, inv *invocation) (TraceRef, string) {
	r := TraceRef{Expr: expr}
	switch {
	case slices.Contains(perPipeline, expr):
		r.Pipeline = PerPipeline
		return r, ""
	case slices.Contains(perCommit, expr):
		r.Pipeline = PerCommit
		return r, ""
	case expr == sharedBase:
		return r, "<< " + expr + " >> can be the same for sibling commits, which the tracer does not model"
	case slices.Contains(stablePipeline, expr):
		r.Pipeline = StablePipeline
		return r, ""
	case strings.HasPrefix(expr, "pipeline.parameters."):
		return m.pipelineParameter(r, strings.TrimPrefix(expr, "pipeline.parameters."))
	case strings.HasPrefix(expr, "parameters."):
		return m.jobParameter(r, ref, inv, strings.TrimPrefix(expr, "parameters."))
	}
	return r, "<< " + expr + " >> is not a value the tracer knows"
}

// jobParameter resolves << parameters.name >> for authored job ref: the call
// site's value, else the matrix list, else the job's default.
func (m *Mutability) jobParameter(r TraceRef, ref string, inv *invocation, name string) (TraceRef, string) {
	if strings.ContainsAny(name, " \"'()") {
		return r, "<< parameters." + name + " >> is a parameter expression, which the tracer does not evaluate"
	}
	if inv != nil && inv.body != nil {
		if matrix := MapGet(inv.body, "matrix"); matrix != nil {
			if list := MapGet(MapGet(matrix, "parameters"), name); list != nil {
				if list.Kind != yaml.SequenceNode {
					return r, "matrix parameter " + name + " is not a list"
				}
				for _, v := range list.Content {
					if deref(v).Kind != yaml.ScalarNode {
						return r, "matrix parameter " + name + " has a value that is not a string"
					}
					r.Values = append(r.Values, deref(v).Value)
				}
				r.Source, r.matrix = m.invocationPath(inv, "matrix", "parameters", name), true
				return m.expandValues(r)
			}
		}
		if v := MapGet(inv.body, name); v != nil {
			if v.Kind != yaml.ScalarNode {
				return r, "parameter " + name + " at the call site is not a string"
			}
			r.Values, r.Source = []string{v.Value}, m.invocationPath(inv, name)
			return m.expandValues(r)
		}
	}
	def := MapGet(MapGet(MapGet(MapGet(m.jobs, ref), "parameters"), name), "default")
	if def == nil || def.Kind != yaml.ScalarNode {
		return r, "parameter " + name + " of job " + ref + " has no call-site value and no default"
	}
	r.Values, r.Source = []string{def.Value}, []string{"jobs", ref, "parameters", name, "default"}
	return m.expandValues(r)
}

// pipelineParameter resolves << pipeline.parameters.name >>. A trigger can
// override any default, so only an enum, whose every value is in the config,
// is traceable.
func (m *Mutability) pipelineParameter(r TraceRef, name string) (TraceRef, string) {
	p := MapGet(MapGet(m.root, "parameters"), name)
	if p == nil {
		return r, "pipeline parameter " + name + " is not declared"
	}
	if t := MapGet(p, "type"); t == nil || t.Value != "enum" {
		return r, "pipeline parameter " + name + " can be overridden when the pipeline is triggered"
	}
	enum := MapGet(p, "enum")
	if enum == nil || enum.Kind != yaml.SequenceNode {
		return r, "pipeline parameter " + name + " has no enum list"
	}
	for _, v := range enum.Content {
		r.Values = append(r.Values, deref(v).Value)
	}
	r.Source = []string{"parameters", name, "enum"}
	return m.expandValues(r)
}

// expandValues handles a value that is itself one pipeline reference, such as
// a call site passing "<< pipeline.git.revision >>". Any other nested
// reference is untraceable.
func (m *Mutability) expandValues(r TraceRef) (TraceRef, string) {
	for _, v := range r.Values {
		if !strings.Contains(v, "<<") {
			continue
		}
		inner := refPattern.FindStringSubmatch(v)
		if len(r.Values) != 1 || inner == nil || strings.TrimSpace(v) != inner[0] {
			return r, "a value of << " + r.Expr + " >> is itself a reference"
		}
		nested, why := m.resolveRef(inner[1], "", nil)
		if why != "" || !strings.HasPrefix(inner[1], "pipeline.") {
			return r, "a value of << " + r.Expr + " >> is a reference that cannot be traced"
		}
		nested.Via, nested.Expr, nested.Source = nested.Expr, r.Expr, r.Source
		return nested, ""
	}
	return r, ""
}

func (m *Mutability) invocationPath(inv *invocation, rest ...string) []string {
	workflows := MapGet(m.root, "workflows")
	if workflows == nil {
		return rest
	}
	for i := 0; i+1 < len(workflows.Content); i += 2 {
		jobs := MapGet(workflows.Content[i+1], "jobs")
		if jobs == nil {
			continue
		}
		for j, item := range jobs.Content {
			if n := deref(item); n.Kind == yaml.MappingNode && len(n.Content) >= 2 && deref(n.Content[1]) == inv.body {
				return append([]string{"workflows", workflows.Content[i].Value, "jobs", strconv.Itoa(j), inv.ref}, rest...)
			}
		}
	}
	return rest
}

// narrowMatrix keeps, for each matrix reference, only the values that
// reproduce the compiled text: the compiled job is one variant, judged with
// its own values, never with its siblings'.
func narrowMatrix(t *Trace, compiled string) {
	for i, r := range t.Refs {
		if !r.matrix || len(r.Values) < len([]string{"a", "b"}) {
			continue
		}
		var kept []string
		for _, v := range r.Values {
			t.Refs[i].Values = []string{v}
			if reproduces(*t, compiled) {
				kept = append(kept, v)
			}
		}
		t.Refs[i].Values = kept
		if len(kept) == 0 {
			t.Refs[i].Values = r.Values // reproduces then fails and reports it
		}
	}
}

// reproduces checks that some substitution of the traced values gives the
// compiled text. A pipeline value's text is unknown, so it matches anything.
func reproduces(t Trace, compiled string) bool {
	var b strings.Builder
	b.WriteString("^")
	prev := 0
	for i, loc := range refPattern.FindAllStringIndex(t.Template, -1) {
		b.WriteString(regexp.QuoteMeta(t.Template[prev:loc[0]]))
		if r := t.Refs[i]; r.Values == nil {
			b.WriteString(".*?")
		} else {
			quoted := make([]string, len(r.Values))
			for j, v := range r.Values {
				quoted[j] = regexp.QuoteMeta(v)
			}
			b.WriteString("(?:" + strings.Join(quoted, "|") + ")")
		}
		prev = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(t.Template[prev:]) + "$")
	re, err := regexp.Compile(b.String())
	return err == nil && re.MatchString(compiled)
}

// authoredStepValue finds the authored node for effPath inside job ref's
// own steps list.
func (m *Mutability) authoredStepValue(job, ref string, jobNode *yaml.Node, effPath []string) (*yaml.Node, []string, string) {
	steps, present := directKey(jobNode, "steps")
	if !present || m.shared(steps) || deref(steps).Kind != yaml.SequenceNode {
		return nil, nil, ("job " + ref + " has no plain authored steps list")
	}
	effIndex, err := strconv.Atoi(effPath[3])
	if err != nil {
		return nil, nil, ("step index " + effPath[3] + " is not a number")
	}
	authIndex, ok := m.alignStep(deref(steps).Content, effIndex, m.effSteps[job])
	if !ok {
		return nil, nil, ("the step cannot be matched to one authored step (" + m.expandingSteps(deref(steps).Content) + ")")
	}
	path := []string{"jobs", ref, "steps", strconv.Itoa(authIndex)}
	n := deref(steps).Content[authIndex]
	for _, part := range effPath[4:] {
		n = deref(n)
		switch {
		case m.shared(n) || hasMergeKey(n):
			return nil, nil, (strings.Join(path, ".") + " is shared through an anchor, alias or merge key")
		case n.Kind == yaml.MappingNode:
			child, ok := directKey(n, part)
			if !ok {
				return nil, nil, (strings.Join(append(path, part), ".") + " is not in the authored config")
			}
			n = child
		case n.Kind == yaml.SequenceNode:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(n.Content) {
				return nil, nil, (strings.Join(append(path, part), ".") + " is not in the authored config")
			}
			n = n.Content[i]
		default:
			return nil, nil, (strings.Join(path, ".") + " is not a mapping or list")
		}
		path = append(path, part)
	}
	return n, path, ""
}

// expandingSteps names what in an authored steps list can expand into a
// different number of effective steps, so a failed match says why.
func (m *Mutability) expandingSteps(steps []*yaml.Node) string {
	var kinds []string
	add := func(k string) {
		if !slices.Contains(kinds, k) {
			kinds = append(kinds, k)
		}
	}
	for _, s := range steps {
		if m.isBuiltinStep(s) {
			continue
		}
		name := deref(s).Value
		if n := deref(s); n.Kind == yaml.MappingNode && len(n.Content) > 0 {
			name = n.Content[0].Value
		}
		switch {
		case m.commands[name]:
			add("a reusable command")
		case strings.Contains(name, "/"):
			add("an orb command")
		case name == "when" || name == "unless":
			add("a when or unless block")
		default:
			add("a step of another kind")
		}
	}
	if len(kinds) == 0 {
		return "no expanding step found"
	}
	return "the job's steps include " + strings.Join(kinds, ", ")
}

// matrixProduces reports whether matrix invocation inv can produce effective
// job job. The compiler names each variant "<job>-<v1>-<v2>…", with the
// values in alphabetical order of the parameter names (measured with
// circleci 1.0.49536: os and arch give "<job>-<arch>-<os>"), and skips the
// tuples listed under exclude. (A matrix alias names the group for
// requires, not the jobs.) When the values cannot be enumerated, any name
// with the prefix is accepted.
func (m *Mutability) matrixProduces(inv invocation, job string) bool {
	if !strings.HasPrefix(job, inv.ref+"-") {
		return false
	}
	matrix := MapGet(inv.body, "matrix")
	params := MapGet(matrix, "parameters")
	if params == nil || params.Kind != yaml.MappingNode {
		return true
	}
	type tuple struct {
		name   string
		values map[string]string
	}
	tuples := []tuple{{name: inv.ref, values: map[string]string{}}}
	order := make([]int, 0, len(params.Content)/len([]string{"key", "value"}))
	for i := 0; i+1 < len(params.Content); i += 2 {
		order = append(order, i)
	}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(params.Content[a].Value, params.Content[b].Value) })
	for _, i := range order {
		key, list := params.Content[i].Value, deref(params.Content[i+1])
		if list.Kind != yaml.SequenceNode {
			return true
		}
		var next []tuple
		for _, t := range tuples {
			for _, v := range list.Content {
				if deref(v).Kind != yaml.ScalarNode {
					return true
				}
				values := maps.Clone(t.values)
				values[key] = deref(v).Value
				next = append(next, tuple{name: t.name + "-" + deref(v).Value, values: values})
			}
		}
		tuples = next
	}
	excluded := func(t tuple) bool {
		ex := MapGet(matrix, "exclude")
		if ex == nil || ex.Kind != yaml.SequenceNode {
			return false
		}
		for _, e := range ex.Content {
			e = deref(e)
			if e.Kind != yaml.MappingNode {
				continue
			}
			all := true
			for i := 0; i+1 < len(e.Content); i += 2 {
				if t.values[e.Content[i].Value] != deref(e.Content[i+1]).Value {
					all = false
				}
			}
			if all {
				return true
			}
		}
		return false
	}
	for _, t := range tuples {
		if t.name == job && !excluded(t) {
			return true
		}
	}
	return false
}
