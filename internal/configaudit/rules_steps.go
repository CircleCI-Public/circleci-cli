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
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var (
	secretKeyRE         = regexp.MustCompile(`(?i)(TOKEN|SECRET|PASSWORD|PASSWD|API_KEY|_KEY$)`)
	secretPlaceholderRE = regexp.MustCompile(`(?i)^(changeme|placeholder|xxx+|\*+|<.*>)$`)
	// secretNotLiteralRE matches values that are plainly a flag or a number, not
	// a secret: YAML booleans and purely numeric values.
	secretNotLiteralRE = regexp.MustCompile(`(?i)^(true|false|yes|no|on|off|\d+)$`)
	secretPrefixes     = []string{"ghp_", "github_pat_", "AKIA", "xoxb-", "xoxp-", "CCIPAT_"}

	curlPipeREs = []*regexp.Regexp{
		regexp.MustCompile(`(?:curl|wget)\b[^|;&\n]*\|\s*(?:sudo\s+)?(?:ba|z)?sh\b`),
		regexp.MustCompile(`(?:ba|z)?sh\s+<\(\s*(?:curl|wget)\b`),
		regexp.MustCompile(`(?:ba|z)?sh\s+-c\s+["']?\$\(\s*(?:curl|wget)\b`),
	}
)

const (
	secretFix = "move the value to a context or project environment variable"
	sshFix    = "list the keys to add with fingerprints: [<fingerprint>]"
	curlFix   = "download the script, verify its checksum, then run it"
)

// walkSteps calls fn for each step of a steps list, including the steps nested
// in when and unless steps.
func walkSteps(n *yaml.Node, fn func(step)) {
	walkStepsDepth(n, fn, 0)
}

func walkStepsDepth(n *yaml.Node, fn func(step), depth int) {
	if depth > maxAliasDepth {
		return
	}
	for _, s := range steps(n) {
		fn(s)
		if (s.name == "when" || s.name == "unless") && s.body != nil {
			walkStepsDepth(mapGet(s.body, "steps"), fn, depth+1)
		}
	}
}

// owner is a job, executor or command: a mapping that can hold steps, an
// environment or docker entries.
type owner struct {
	kind string // "job", "executor" or "command"
	name string
	node *yaml.Node
}

// owners lists the entries of the named sections, in the order given.
func (d *Doc) owners(kinds ...string) []owner {
	var out []owner
	for _, kind := range kinds {
		section := map[string]*yaml.Node{"job": d.jobs, "executor": d.executors, "command": d.commands}[kind]
		for _, kv := range mapPairs(section) {
			if kv[1] != nil && kv[1].Kind == yaml.MappingNode {
				out = append(out, owner{kind: kind, name: kv[0].Value, node: kv[1]})
			}
		}
	}
	return out
}

// runCommand returns the scalar holding the shell text of a run step: the step
// itself in the string form, or its command key.
func runCommand(s step) *yaml.Node {
	if s.name != "run" || s.body == nil {
		return nil
	}
	cmd := s.body
	if cmd.Kind == yaml.MappingNode {
		cmd = mapGet(cmd, "command")
	}
	if cmd == nil || cmd.Kind != yaml.ScalarNode {
		return nil
	}
	return cmd
}

// eachRunCommand calls fn for every run step's command of every job and command.
func (d *Doc) eachRunCommand(fn func(o owner, cmd *yaml.Node)) {
	for _, o := range d.owners("job", "command") {
		walkSteps(mapGet(o.node, "steps"), func(s step) {
			if cmd := runCommand(s); cmd != nil {
				fn(o, cmd)
			}
		})
	}
}

// paramTaint says why a job or command parameter carries an outsider's value.
type paramTaint struct {
	ref    string // the tainted pipeline value
	passed bool   // handed in by a caller, as opposed to the parameter's default
}

// paramTaints finds, per job or command ("job:name", "command:name"), the
// parameters that get a tainted pipeline value: from their default, or from an
// invocation (a workflow job entry or a step) that passes one. It does not
// follow a value through a second parameter.
func (d *Doc) paramTaints() map[string]map[string]paramTaint {
	out := map[string]map[string]paramTaint{}
	set := func(key, param string, t paramTaint) {
		if out[key] == nil {
			out[key] = map[string]paramTaint{}
		}
		if _, ok := out[key][param]; !ok {
			out[key][param] = t
		}
	}
	for _, o := range d.owners("job", "command") {
		for _, kv := range mapPairs(mapGet(o.node, "parameters")) {
			def := mapGet(kv[1], "default")
			if def == nil || def.Kind != yaml.ScalarNode {
				continue
			}
			if ref, ok := taintedRef(def.Value); ok {
				set(o.kind+":"+o.name, kv[0].Value, paramTaint{ref: ref})
			}
		}
	}

	passArgs := func(key string, args *yaml.Node) {
		for _, kv := range mapPairs(args) {
			if kv[1] == nil || kv[1].Kind != yaml.ScalarNode {
				continue
			}
			if ref, ok := taintedRef(kv[1].Value); ok {
				set(key, kv[0].Value, paramTaint{ref: ref, passed: true})
			}
		}
	}
	invokeCommands := func(stepList *yaml.Node) {
		walkSteps(stepList, func(s step) {
			if s.body != nil && s.body.Kind == yaml.MappingNode && mapGet(d.commands, s.name) != nil {
				passArgs("command:"+s.name, s.body)
			}
		})
	}
	for _, o := range d.owners("job", "command") {
		invokeCommands(mapGet(o.node, "steps"))
	}
	for _, wf := range mapPairs(d.workflows) {
		for _, item := range seqItems(mapGet(wf[1], "jobs")) {
			pairs := mapPairs(item)
			if len(pairs) != 1 || pairs[0][1] == nil || pairs[0][1].Kind != yaml.MappingNode {
				continue
			}
			name, body := pairs[0][0].Value, pairs[0][1]
			if mapGet(d.jobs, name) != nil {
				passArgs("job:"+name, body)
			}
			invokeCommands(mapGet(body, "pre-steps"))
			invokeCommands(mapGet(body, "post-steps"))
		}
	}
	return out
}

// envName turns the last part of a value's name into an environment variable
// name: pipeline.git.branch becomes BRANCH.
func envName(ref string) string {
	name := ref[strings.LastIndex(ref, ".")+1:]
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// checkInjection flags pipeline values an outsider controls that are expanded
// into the text of a run step, where they become shell syntax.
func checkInjection(d *Doc, emit func(Finding)) {
	taints := d.paramTaints()
	d.eachRunCommand(func(o owner, cmd *yaml.Node) {
		for _, m := range valueRefRE.FindAllStringSubmatchIndex(cmd.Value, -1) {
			name := cmd.Value[m[2]:m[3]]
			var msg, ref string
			switch {
			case strings.HasPrefix(name, "pipeline."):
				if !isTainted(name) {
					continue
				}
				ref = name
				msg = name + " is expanded into a shell command"
			default:
				param := strings.TrimPrefix(name, "parameters.")
				t, ok := taints[o.kind+":"+o.name][param]
				if !ok {
					continue
				}
				ref = name
				verb := "defaults to "
				if t.passed {
					verb = "is passed "
				}
				msg = "parameter " + param + " " + verb + t.ref + " and is expanded into a shell command"
			}
			pos := locate(cmd, d.lines, m[0], m[1]-m[0])
			emit(Finding{
				RuleID:   "pipeline-value-injection",
				Message:  msg,
				Fix:      "pass it through environment: (" + envName(ref) + ": << " + ref + " >>) and use \"$" + envName(ref) + "\"",
				Position: &pos,
			})
		}
	})
}

// checkHardcodedSecret flags literal secrets in environment mappings and docker
// registry passwords.
func checkHardcodedSecret(d *Doc, emit func(Finding)) {
	checkEnv := func(env *yaml.Node) {
		for _, kv := range mapPairs(env) {
			checkSecretValue(d, kv[0].Value, secretKeyRE.MatchString(kv[0].Value), kv[1], emit)
		}
	}
	for _, o := range d.owners("job", "executor", "command") {
		checkEnv(mapGet(o.node, "environment"))
		for _, entry := range seqItems(mapGet(o.node, "docker")) {
			checkEnv(mapGet(entry, "environment"))
			checkSecretValue(d, "docker auth password", true, mapGet(mapGet(entry, "auth"), "password"), emit)
		}
		walkSteps(mapGet(o.node, "steps"), func(s step) {
			if s.name == "run" && s.body != nil && s.body.Kind == yaml.MappingNode {
				checkEnv(mapGet(s.body, "environment"))
			}
		})
	}
}

// checkSecretValue emits a finding when val is a literal that looks like a
// secret: its key matches (keyMatches) or its value starts with a known token
// prefix. label names the value in the message.
func checkSecretValue(d *Doc, label string, keyMatches bool, val *yaml.Node, emit func(Finding)) {
	if val == nil || val.Kind != yaml.ScalarNode {
		return
	}
	v := strings.TrimSpace(val.Value)
	if v == "" || strings.HasPrefix(v, "$") || strings.Contains(v, "<<") || secretPlaceholderRE.MatchString(v) {
		return
	}
	prefixed := slices.ContainsFunc(secretPrefixes, func(p string) bool { return strings.HasPrefix(v, p) })
	if !keyMatches && !prefixed {
		return
	}
	if !prefixed && secretNotLiteralRE.MatchString(v) {
		return
	}
	confidence := ConfidenceMedium
	if prefixed {
		confidence = ConfidenceHigh
	}
	pos := locate(val, d.lines, 0, len(val.Value))
	endCol := pos.EndColumn
	if pos.EndLine != pos.Line && pos.Line <= len(d.lines) {
		endCol = utf8.RuneCountInString(d.lines[pos.Line-1]) + 1
	}
	emit(Finding{
		RuleID:     "hardcoded-secret",
		Confidence: confidence,
		Message:    label + " has a literal value (" + redact(val.Value) + ")",
		Fix:        secretFix,
		Position:   &pos,
		secret:     &redaction{line: pos.Line, col: pos.Column, endCol: endCol, with: redact(val.Value)},
	})
}

// checkSSHKeys flags add_ssh_keys steps that do not name the keys to add, so
// every SSH key of the project is loaded into the job.
func checkSSHKeys(d *Doc, emit func(Finding)) {
	for _, o := range d.owners("job", "command") {
		walkSteps(mapGet(o.node, "steps"), func(s step) {
			if s.name != "add_ssh_keys" || hasFingerprints(s.body) {
				return
			}
			pos := locate(s.nameNode, d.lines, 0, len(s.name))
			emit(Finding{
				RuleID:   "ssh-keys-unrestricted",
				Message:  "add_ssh_keys loads every SSH key of the project",
				Fix:      sshFix,
				Position: &pos,
			})
		})
	}
}

// hasFingerprints reports whether an add_ssh_keys body lists a fingerprint.
func hasFingerprints(body *yaml.Node) bool {
	if body == nil || body.Kind != yaml.MappingNode {
		return false
	}
	fp := mapGet(body, "fingerprints")
	if fp == nil {
		return false
	}
	if fp.Kind == yaml.ScalarNode {
		return strings.TrimSpace(fp.Value) != ""
	}
	return slices.ContainsFunc(seqItems(fp), func(item *yaml.Node) bool {
		return item.Kind != yaml.ScalarNode || strings.TrimSpace(item.Value) != ""
	})
}

// checkCurlPipeShell flags run text that pipes a downloaded script into a shell.
func checkCurlPipeShell(d *Doc, emit func(Finding)) {
	d.eachRunCommand(func(_ owner, cmd *yaml.Node) {
		joined, orig := joinContinuations(cmd.Value)
		type span struct{ start, end int } // byte offsets in cmd.Value
		var spans []span
		for _, re := range curlPipeREs {
			for _, m := range re.FindAllStringIndex(joined, -1) {
				spans = append(spans, span{orig[m[0]], orig[m[1]-1] + 1})
			}
		}
		slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
		last := -1
		for _, sp := range spans {
			if sp.start < last {
				continue // inside the match before it
			}
			last = sp.end
			pos := locate(cmd, d.lines, sp.start, sp.end-sp.start)
			emit(Finding{
				RuleID:   "curl-pipe-shell",
				Message:  "a remote script is piped into a shell",
				Fix:      curlFix,
				Position: &pos,
			})
		}
	})
}

// joinContinuations replaces each backslash-newline line continuation in s with
// a space. orig maps each byte of the result to its offset in s; it has one
// more entry than the result's length.
func joinContinuations(s string) (joined string, orig []int) {
	var b strings.Builder
	orig = make([]int, 0, len(s)+1)
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '\n' {
			b.WriteByte(' ')
			orig = append(orig, i)
			i++
			continue
		}
		b.WriteByte(s[i])
		orig = append(orig, i)
	}
	return b.String(), append(orig, len(s))
}
