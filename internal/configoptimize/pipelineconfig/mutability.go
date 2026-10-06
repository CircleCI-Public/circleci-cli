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

// Label says whether a node of the effective config can safely be edited in
// the authored file. Only LabelWritable is ever edited; every other
// label makes a finding report-only.
type Label string

const (
	// LabelWritable is a directly authored node owned by exactly one job.
	LabelWritable Label = "writable"
	// LabelOrbSourced came from an orb.
	LabelOrbSourced Label = "orb-sourced"
	// LabelSharedExecutor is part of an executor used by more than one job.
	LabelSharedExecutor Label = "shared-executor"
	// LabelSharedSource is reached through a matrix, an anchor or alias, a
	// merge key, a reusable command, a reusable job invoked more than once, or
	// a parameter — shapes where one authored node feeds several effective
	// ones, so editing it would change more than the target.
	LabelSharedSource Label = "shared-source"
	// LabelUnmatched means no authored node could be matched to the target
	// safely. The careful match can fail for shapes it does not
	// understand, and guessing would edit the wrong line.
	LabelUnmatched Label = "unmatched"
)

// Resolution is where an effective node lives in the authored file.
type Resolution struct {
	Label Label
	// AuthoredPath is the node's path in the authored file. Set only for
	// LabelWritable; it can differ from the effective path, e.g. when the
	// value lives on a named executor used by one job.
	AuthoredPath []string
	// Value is the authored node's decoded value, for writable resolutions.
	// An edit uses it to check the node still holds what was analysed.
	Value any
	// Reason explains any label other than writable.
	Reason string
	// Override, on a resolution that is not writable, says the job itself
	// can hold a value of its own that wins over the inherited one, leaving
	// the shared source untouched. Set only for a key directly under a
	// job that Resolve accepts: not an orb job, a matrix instance, a job
	// invoked more than once, or a shared job mapping.
	Override *Override
}

// Override is where a job-level value would go.
type Override struct {
	// Path is the authored path of the job's own key.
	Path []string
	// Insert is true when the job has no such key: it inherits the value
	// from a merge key or a shared executor, and the key is added. False
	// means the job has the key beside a merge key: replacing it is safe,
	// although deleting it would expose the merged value.
	Insert bool
	// Value is the authored value, when the key exists.
	Value any
}

// Mutability answers "is this effective node safe to edit?" once, for every
// module. Only reconcile's feasibility stage reads it.
// It matches paths between the two copies, so anything reached through a
// matrix, an alias, a merge key, a reusable command or a parameter is
// report-only.
type Mutability struct {
	root        *yaml.Node
	jobs        *yaml.Node
	executors   *yaml.Node
	orbs        map[string]bool
	commands    map[string]bool
	aliased     map[*yaml.Node]bool
	invocations map[string][]invocation
	effSteps    map[string]int
}

// invocation is one use of an authored job in a workflow.
type invocation struct {
	ref    string // the job it runs, e.g. "build" or "docker/publish"
	name   string // the effective job name it produces, "" for a matrix
	matrix bool
	params bool // passes parameters to the job
	// wraps is true when the invocation adds pre-steps or post-steps, which
	// shift every step index.
	wraps bool
	// body is the invocation's mapping (parameters, pre-steps, post-steps).
	body *yaml.Node
}

// pipelineRef matches a pipeline value reference however it is spaced.
var pipelineRef = regexp.MustCompile(`<<\s*pipeline\.`)

// builtinSteps are the step types that expand to exactly one effective step.
var builtinSteps = []string{
	"add_ssh_keys", "attach_workspace", "checkout", "persist_to_workspace",
	"restore_cache", "run", "save_cache", "setup_remote_docker",
	"store_artifacts", "store_test_results",
}

// IsBuiltinStep reports whether name is a builtin step type such as run or
// checkout.
func IsBuiltinStep(name string) bool {
	return slices.Contains(builtinSteps, name)
}

// workflowJobKeys are the invocation keys that are not job parameters.
var workflowJobKeys = []string{
	"context", "filters", "matrix", "name", "post-steps", "pre-steps",
	"requires", "serial-group", "type",
}

// NewMutability indexes the authored config against the effective one.
func NewMutability(a *Authored, eff *Effective) *Mutability {
	m := &Mutability{
		orbs:        map[string]bool{},
		commands:    map[string]bool{},
		aliased:     map[*yaml.Node]bool{},
		invocations: map[string][]invocation{},
		effSteps:    map[string]int{},
	}
	for _, j := range eff.Jobs {
		m.effSteps[j.Name] = len(j.Steps())
	}
	eff.tracer = m
	m.root = a.Root()
	if m.root == nil {
		return m
	}
	eff.Setup = IsTrue(MapGet(m.root, "setup"))
	for _, d := range a.Documents() {
		collectAliased(d, m.aliased)
	}
	// An anchored key that an alias refers to makes its whole entry shared:
	// removing the entry would break the alias elsewhere.
	for _, d := range a.Documents() {
		markAliasedKeys(d, m.aliased)
	}
	m.jobs = MapGet(m.root, "jobs")
	m.executors = MapGet(m.root, "executors")
	for _, k := range mapKeys(MapGet(m.root, "orbs")) {
		m.orbs[k] = true
	}
	for _, k := range mapKeys(MapGet(m.root, "commands")) {
		m.commands[k] = true
	}
	m.indexWorkflows()
	return m
}

func (m *Mutability) indexWorkflows() {
	workflows := MapGet(m.root, "workflows")
	if workflows == nil || workflows.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(workflows.Content); i += 2 {
		jobs := MapGet(workflows.Content[i+1], "jobs")
		if jobs == nil || jobs.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range jobs.Content {
			inv, ok := parseInvocation(deref(item))
			if ok {
				m.invocations[inv.ref] = append(m.invocations[inv.ref], inv)
			}
		}
	}
}

func parseInvocation(n *yaml.Node) (invocation, bool) {
	switch {
	case n == nil:
		return invocation{}, false
	case n.Kind == yaml.ScalarNode:
		return invocation{ref: n.Value, name: n.Value}, true
	case n.Kind == yaml.MappingNode && len(n.Content) >= 2:
		inv := invocation{ref: n.Content[0].Value, name: n.Content[0].Value}
		body := deref(n.Content[1])
		if name := MapGet(body, "name"); name != nil {
			inv.name = name.Value
		}
		inv.body = body
		if hasMergeKey(body) {
			// Keys merged into an invocation (name, matrix, pre-steps) are
			// not indexed, so it is treated as shared.
			inv.matrix = true
			inv.name = ""
		}
		if MapGet(body, "matrix") != nil {
			inv.matrix = true
			inv.name = ""
		}
		for _, k := range mapKeys(body) {
			if !slices.Contains(workflowJobKeys, k) {
				inv.params = true
			}
			if k == "pre-steps" || k == "post-steps" {
				inv.wraps = true
			}
		}
		return inv, true
	}
	return invocation{}, false
}

// Resolve labels the node at effPath (e.g. ["jobs", "build", "machine",
// "docker_layer_caching"]) of effective job job. effValue is the effective
// node at that path, used to confirm the authored value is the same literal.
func (m *Mutability) Resolve(job string, effPath []string, effValue *yaml.Node) Resolution {
	if len(effPath) < 3 || effPath[0] != "jobs" || effPath[1] != job {
		return unmatched("only job-scoped paths are resolved")
	}
	ref, res, ok := m.authoredJob(job)
	if !ok {
		return res
	}
	jobNode := MapGet(m.jobs, ref)
	if jobNode == nil {
		return unmatched("job " + ref + " is not defined in the authored config")
	}
	if m.shared(m.jobValueNode(ref)) {
		return shared("job " + ref + " is an anchor or alias shared with other nodes")
	}
	return m.resolveInJob(job, ref, jobNode, effPath[2:], effValue)
}

// authoredJob finds the authored job an effective job came from.
func (m *Mutability) authoredJob(job string) (string, Resolution, bool) {
	refs := slices.Sorted(maps.Keys(m.invocations))
	// An invocation that renames the job, or runs it directly.
	for _, ref := range refs {
		for _, inv := range m.invocations[ref] {
			if inv.name == job {
				return m.checkInvocations(ref)
			}
		}
	}
	// A matrix instance: the compiler names it "<job>-<values>".
	for _, ref := range refs {
		for _, inv := range m.invocations[ref] {
			if inv.matrix && strings.HasPrefix(job, ref+"-") {
				return "", shared("job " + job + " is a matrix instance of " + ref), false
			}
		}
	}
	if m.isOrbRef(job) {
		return "", Resolution{Label: LabelOrbSourced, Reason: "job " + job + " comes from an orb"}, false
	}
	if MapGet(m.jobs, job) != nil {
		return m.checkInvocations(job)
	}
	return "", unmatched("no authored job produces effective job " + job), false
}

func (m *Mutability) checkInvocations(ref string) (string, Resolution, bool) {
	if m.isOrbRef(ref) {
		return "", Resolution{Label: LabelOrbSourced, Reason: "job " + ref + " comes from an orb"}, false
	}
	invs := m.invocations[ref]
	if len(invs) > 1 {
		return "", shared("job " + ref + " is invoked " + strconv.Itoa(len(invs)) + " times"), false
	}
	for _, inv := range invs {
		if inv.matrix {
			return "", shared("job " + ref + " is expanded by a matrix"), false
		}
	}
	return ref, Resolution{}, true
}

func (m *Mutability) resolveInJob(job, ref string, jobNode *yaml.Node, rest []string, effValue *yaml.Node) Resolution {
	path := []string{"jobs", ref}
	if rest[0] == "steps" {
		return m.resolveStep(job, ref, jobNode, rest, effValue)
	}
	own := len(rest) == 1 // a key directly under the job, which an override can hold
	if key, present := directKey(jobNode, rest[0]); present {
		if hasMergeKey(jobNode) {
			res := shared(rest[0] + " of job " + ref + " sits beside a merge key, which could supply the same value")
			if own {
				// The job's own key always wins over the merge, so a
				// replace is safe where a delete is not.
				if w := m.resolveNode(key, append(path, rest[0]), nil, effValue, ref); w.Label == LabelWritable {
					res.Override = &Override{Path: w.AuthoredPath, Value: w.Value}
				}
			}
			return res
		}
		return m.resolveNode(key, append(path, rest[0]), rest[1:], effValue, ref)
	}
	if hasMergeKey(jobNode) {
		res := shared(rest[0] + " of job " + ref + " comes from a merge key")
		if own {
			res.Override = &Override{Path: append(path, rest[0]), Insert: true}
		}
		return res
	}
	executor := MapGet(jobNode, "executor")
	if executor == nil {
		return unmatched(rest[0] + " is not set on authored job " + ref)
	}
	res := m.resolveExecutor(ref, executor, rest, effValue)
	if own && (res.Label == LabelSharedExecutor || res.Label == LabelSharedSource) {
		// Only an authored executor: resolveExecutor labels an orb's as
		// orb-sourced, and an unresolvable one as unmatched.
		res.Override = &Override{Path: append(path, rest[0]), Insert: true}
	}
	return res
}

func (m *Mutability) resolveExecutor(ref string, executor *yaml.Node, rest []string, effValue *yaml.Node) Resolution {
	name := executor.Value
	if executor.Kind == yaml.MappingNode {
		n := MapGet(executor, "name")
		if n == nil {
			return unmatched("executor of job " + ref + " has no name")
		}
		name = n.Value
	}
	if m.isOrbRef(name) {
		return Resolution{Label: LabelOrbSourced, Reason: "executor " + name + " comes from an orb"}
	}
	execNode := MapGet(m.executors, name)
	if execNode == nil {
		return unmatched("executor " + name + " is not defined in the authored config")
	}
	if users := m.executorUsers(name); users > 1 {
		return Resolution{
			Label:  LabelSharedExecutor,
			Reason: "executor " + name + " is used by " + strconv.Itoa(users) + " jobs",
		}
	}
	if m.shared(m.executorValueNode(name)) {
		return shared("executor " + name + " is an anchor or alias shared with other nodes")
	}
	if hasMergeKey(execNode) {
		return shared("executor " + name + " has a merge key")
	}
	key, present := directKey(execNode, rest[0])
	if !present {
		return unmatched(rest[0] + " is not set on executor " + name)
	}
	return m.resolveNode(key, []string{"executors", name, rest[0]}, rest[1:], effValue, ref)
}

// resolveStep maps an effective step index to an authored one. Builtin steps
// expand to exactly one effective step, so indexes line up for a prefix of
// builtin steps and, counted from the end, for a suffix of them. A step
// between two expanded commands cannot be located safely.
func (m *Mutability) resolveStep(job, ref string, jobNode *yaml.Node, rest []string, effValue *yaml.Node) Resolution {
	if len(rest) < 2 {
		return unmatched("the steps list itself is not resolved")
	}
	effIndex, err := strconv.Atoi(rest[1])
	if err != nil {
		return unmatched("step index " + rest[1] + " is not a number")
	}
	steps, present := directKey(jobNode, "steps")
	if !present {
		if hasMergeKey(jobNode) {
			return shared("steps of job " + ref + " come from a merge key")
		}
		return unmatched("job " + ref + " has no authored steps")
	}
	if m.shared(steps) {
		return shared("steps of job " + ref + " are an anchor or alias")
	}
	stepsNode := deref(steps)
	if stepsNode.Kind != yaml.SequenceNode {
		return unmatched("steps of job " + ref + " is not a sequence")
	}
	for _, inv := range m.invocations[ref] {
		if inv.wraps {
			return shared("job " + ref + " is invoked with pre-steps or post-steps, which shift step indexes")
		}
	}
	effCount, known := m.effSteps[job]
	if !known {
		return unmatched("effective job " + job + " was not indexed")
	}
	authIndex, ok := m.alignStep(stepsNode.Content, effIndex, effCount)
	if !ok {
		return shared("effective step " + rest[1] + " of job " + job +
			" cannot be matched to one authored step: reusable commands and orb commands expand into several")
	}
	path := []string{"jobs", ref, "steps", strconv.Itoa(authIndex)}
	return m.resolveNode(stepsNode.Content[authIndex], path, rest[2:], effValue, ref)
}

func (m *Mutability) alignStep(authored []*yaml.Node, effIndex, effCount int) (int, bool) {
	prefix := 0
	for prefix < len(authored) && m.isBuiltinStep(authored[prefix]) {
		prefix++
	}
	if prefix == len(authored) {
		// Every step is builtin: counts must match exactly.
		if effCount != len(authored) {
			return 0, false
		}
		return effIndex, effIndex < len(authored)
	}
	if effIndex < prefix {
		return effIndex, true
	}
	suffix := 0
	for suffix < len(authored)-prefix && m.isBuiltinStep(authored[len(authored)-1-suffix]) {
		suffix++
	}
	fromEnd := effCount - effIndex
	if fromEnd >= 1 && fromEnd <= suffix {
		return len(authored) - fromEnd, true
	}
	return 0, false
}

func (m *Mutability) isBuiltinStep(n *yaml.Node) bool {
	n = deref(n)
	var name string
	switch n.Kind {
	case yaml.ScalarNode:
		name = n.Value
	case yaml.MappingNode:
		if len(n.Content) != 2 {
			return false
		}
		name = n.Content[0].Value
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return false
	}
	// Only a builtin name that no local command shadows expands to one step.
	return IsBuiltinStep(name) && !m.commands[name]
}

// resolveNode walks the remaining mapping keys from n, refusing anything
// shared, and finally checks the authored value is the same literal as the
// effective one.
func (m *Mutability) resolveNode(n *yaml.Node, path, rest []string, effValue *yaml.Node, ref string) Resolution {
	for _, key := range rest {
		if m.shared(n) {
			return shared(strings.Join(path, ".") + " is an anchor or alias")
		}
		parent := deref(n)
		if hasMergeKey(parent) {
			// Editing a direct key beside a merge can expose the merged
			// value instead of changing the result.
			return shared(strings.Join(path, ".") + " has a merge key, which could supply " + key)
		}
		child, present := directKey(parent, key)
		if !present {
			if hasMergeKey(parent) {
				return shared(strings.Join(append(path, key), ".") + " comes from a merge key")
			}
			return unmatched(strings.Join(append(path, key), ".") + " is not in the authored config")
		}
		n = child
		path = append(path, key)
	}
	if m.shared(n) {
		return shared(strings.Join(path, ".") + " is an anchor or alias")
	}
	if v := deref(n); v.Kind == yaml.ScalarNode && strings.Contains(v.Value, "<<") {
		return shared(strings.Join(path, ".") + " is set by a parameter")
	}
	if effValue == nil {
		// Fail closed: without the effective node there is nothing to prove
		// the authored one produced.
		return unmatched(strings.Join(path, ".") + " has no effective counterpart")
	}
	if !sameScalar(deref(n), effValue) {
		if len(m.invocations[ref]) == 1 && m.invocations[ref][0].params {
			return shared(strings.Join(path, ".") + " differs after parameter substitution")
		}
		return unmatched(strings.Join(path, ".") + " does not match the compiled value")
	}
	var value any
	if err := deref(n).Decode(&value); err != nil {
		return unmatched(strings.Join(path, ".") + " could not be decoded")
	}
	return Resolution{Label: LabelWritable, AuthoredPath: slices.Clone(path), Value: value}
}

// shared reports whether n is an alias, or an anchored node some alias points
// at: editing it would change every other place it appears.
func (m *Mutability) shared(n *yaml.Node) bool {
	return n != nil && (n.Kind == yaml.AliasNode || m.aliased[n])
}

func (m *Mutability) isOrbRef(name string) bool {
	prefix, _, ok := strings.Cut(name, "/")
	return ok && m.orbs[prefix]
}

// executorUsers counts the authored jobs that may run on executor name. A
// job whose executor is a parameter (`<< parameters.ex >>`) may run on any
// executor, so it counts for every one.
func (m *Mutability) executorUsers(name string) int {
	users := 0
	for i := 0; m.jobs != nil && i+1 < len(m.jobs.Content); i += 2 {
		ex := MapGet(m.jobs.Content[i+1], "executor")
		if ex == nil {
			continue
		}
		ref := ex.Value
		if n := MapGet(ex, "name"); n != nil {
			ref = n.Value
		}
		if ref == name || strings.Contains(ref, "<<") {
			users++
		}
	}
	return users
}

func (m *Mutability) jobValueNode(ref string) *yaml.Node {
	n, _ := directKey(m.jobs, ref)
	return n
}

func (m *Mutability) executorValueNode(name string) *yaml.Node {
	n, _ := directKey(m.executors, name)
	return n
}

// directKey returns the raw (not dereferenced) value node for key, looking
// only at the mapping's own keys, never through a merge key.
func directKey(n *yaml.Node, key string) (*yaml.Node, bool) {
	v := mapValue(n, key, false)
	return v, v != nil
}

func hasMergeKey(n *yaml.Node) bool {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Tag == "!!merge" {
			return true
		}
	}
	return false
}

func sameScalar(a, b *yaml.Node) bool {
	if a.Kind != yaml.ScalarNode || b.Kind != yaml.ScalarNode {
		return a.Kind == b.Kind
	}
	var av, bv any
	if a.Decode(&av) != nil || b.Decode(&bv) != nil {
		return a.Value == b.Value
	}
	return av == bv
}

func markAliasedKeys(n *yaml.Node, aliased map[*yaml.Node]bool) {
	if n == nil {
		return
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if aliased[n.Content[i]] || n.Content[i].Kind == yaml.AliasNode {
				aliased[n.Content[i+1]] = true
			}
		}
	}
	for _, c := range n.Content {
		markAliasedKeys(c, aliased)
	}
}

func collectAliased(n *yaml.Node, out map[*yaml.Node]bool) {
	if n == nil {
		return
	}
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		out[n.Alias] = true
	}
	for _, c := range n.Content {
		collectAliased(c, out)
	}
}

func mapKeys(n *yaml.Node) []string {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

// Dependence says why a job's compiled steps may not hold for other inputs.
type Dependence int

const (
	// DependenceNone means nothing reachable from the job reads a pipeline value.
	DependenceNone Dependence = iota
	// DependenceKnown means the job, a command or executor it uses, or its
	// invocation visibly reads a pipeline value.
	DependenceKnown
	// DependenceOpaqueOrb means the only doubt is a registry orb command whose
	// body is not in the authored file. Measured usage already includes what it
	// did, so it does not hold back usage evidence.
	DependenceOpaqueOrb
)

// InputDependence reports whether an effective job's steps could differ
// under other pipeline inputs, and why. The compiler resolves every parameter and
// condition for one set of inputs, so evidence read from its output
// holds for other inputs only if nothing reachable from the job reads a
// pipeline value: the job itself, the local commands and executor it uses,
// and the workflow invocation that runs it. Job and command parameters come
// from literals in the config, so they are fixed unless they chain to a
// pipeline value, which this walk sees.
// A registry orb command's body is not in the authored file, so a job that
// calls one is judged from its expansion under the analysed inputs.
func (m *Mutability) InputDependence(job string) (Dependence, string) {
	ref, _, ok := m.authoredJob(job)
	if !ok {
		return DependenceNone, ""
	}
	if m.pipelineAnywhere(MapGet(m.jobs, ref), map[string]bool{}) {
		return DependenceKnown, "job " + ref + " reads pipeline values (<< pipeline.* >>), directly or through a command or executor"
	}
	for _, inv := range m.invocations[ref] {
		if m.pipelineAnywhere(inv.body, map[string]bool{}) {
			return DependenceKnown, "job " + ref + " is invoked with parameters or pre/post-steps that read pipeline values"
		}
	}
	if orbCmd := m.registryOrbCommand(MapGet(m.jobs, ref), map[string]bool{}); orbCmd != "" {
		// Resolved conservatively: a registry orb's command body is
		// not in the authored file, so it may read pipeline values unseen.
		// Kept apart from known dependence: measured usage already includes
		// what the command did.
		return DependenceOpaqueOrb, "job " + ref + " calls the registry orb command " + orbCmd + ", whose body is not in the config"
	}
	return DependenceNone, ""
}

// pipelineAnywhere walks n, following local commands and executors it names.
func (m *Mutability) pipelineAnywhere(n *yaml.Node, seen map[string]bool) bool {
	n = deref(n)
	if n == nil {
		return false
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return pipelineRef.MatchString(n.Value)
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if m.pipelineAnywhere(key, seen) || m.pipelineAnywhere(val, seen) {
				return true
			}
			if key.Value == "executor" {
				name := deref(val).Value
				if nameNode := MapGet(val, "name"); nameNode != nil {
					name = nameNode.Value
				}
				if m.followName("executor:"+name, MapGet(m.executors, name), seen) {
					return true
				}
			}
			if m.commands[key.Value] && m.followName("command:"+key.Value, MapGet(MapGet(m.root, "commands"), key.Value), seen) {
				return true
			}
			if body := m.inlineOrbCommand(key.Value); body != nil && m.followName("orb:"+key.Value, body, seen) {
				return true
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			c = deref(c) // an alias step names the same command
			if c.Kind == yaml.ScalarNode && m.commands[c.Value] &&
				m.followName("command:"+c.Value, MapGet(MapGet(m.root, "commands"), c.Value), seen) {
				return true
			}
			if c.Kind == yaml.ScalarNode {
				if body := m.inlineOrbCommand(c.Value); body != nil && m.followName("orb:"+c.Value, body, seen) {
					return true
				}
			}
			if m.pipelineAnywhere(c, seen) {
				return true
			}
		}
	case yaml.DocumentNode, yaml.AliasNode:
	}
	return false
}

// inlineOrbCommand returns the body of `alias/name` when alias is an orb
// defined inline in the config, else nil.
func (m *Mutability) inlineOrbCommand(ref string) *yaml.Node {
	alias, name, ok := strings.Cut(ref, "/")
	if !ok {
		return nil
	}
	orb := MapGet(MapGet(m.root, "orbs"), alias)
	if orb == nil || orb.Kind != yaml.MappingNode {
		return nil
	}
	return MapGet(MapGet(orb, "commands"), name)
}

// registryOrbCommand returns the first `alias/name` step reachable from n
// whose orb comes from the registry (a version string, not an inline body).
func (m *Mutability) registryOrbCommand(n *yaml.Node, seen map[string]bool) string {
	n = deref(n)
	if n == nil {
		return ""
	}
	isRegistry := func(ref string) bool {
		alias, _, ok := strings.Cut(ref, "/")
		orb := MapGet(MapGet(m.root, "orbs"), alias)
		return ok && orb != nil && orb.Kind == yaml.ScalarNode
	}
	steps := MapGet(n, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return ""
	}
	for _, st := range steps.Content {
		st = deref(st)
		name := st.Value
		if st.Kind == yaml.MappingNode && len(st.Content) >= len([]string{"key", "value"}) {
			name = st.Content[0].Value
		}
		if isRegistry(name) {
			return name
		}
		if m.commands[name] && !seen[name] {
			seen[name] = true
			if found := m.registryOrbCommand(MapGet(MapGet(m.root, "commands"), name), seen); found != "" {
				return found
			}
		}
		if body := m.inlineOrbCommand(name); body != nil && !seen["orb:"+name] {
			seen["orb:"+name] = true
			if found := m.registryOrbCommand(body, seen); found != "" {
				return found
			}
		}
	}
	return ""
}

func (m *Mutability) followName(key string, n *yaml.Node, seen map[string]bool) bool {
	if n == nil || seen[key] {
		return false
	}
	seen[key] = true
	return m.pipelineAnywhere(n, seen)
}

func shared(reason string) Resolution {
	return Resolution{Label: LabelSharedSource, Reason: reason}
}

func unmatched(reason string) Resolution {
	return Resolution{Label: LabelUnmatched, Reason: reason}
}
