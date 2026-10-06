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
	"strings"
)

// TokenKind is the type of one `{{ … }}` template token. Keys are compared
// by these types, never by raw text.
type TokenKind int

// Token kinds: TokOther is any expression the model does not know; the
// rest are {{ checksum "f" }}, {{ epoch }}, {{ .Revision }},
// {{ .BuildNum }}, {{ .Branch }}, {{ arch }} and {{ .Environment.X }}.
const (
	TokOther TokenKind = iota
	TokChecksum
	TokEpoch
	TokRevision
	TokBuildNum
	TokBranch
	TokArch
	TokEnv
)

// Token is one parsed `{{ … }}` expression.
type Token struct {
	Kind TokenKind
	// Arg is the checksum's file or the environment variable's name.
	Arg  string
	Expr string
}

// Scope is how far a cache saved under a key can be reused (X1-static-v2).
// Scopes run from narrowest to broadest: a key scoped to the
// time of each cache step cannot be reused at all, one scoped to a revision
// can be reused by every job and pipeline on that commit.
type Scope int

// Scopes. Unscoped is a key with no scoped part (stable, or keyed on a
// checksum); it gives no finding.
const (
	Unscoped Scope = iota
	ScopeTime
	ScopeJob
	ScopeWorkflow
	ScopePipeline
	ScopeRevision
)

// Code is the scope's reason code.
func (s Scope) Code() string {
	switch s {
	case ScopeTime:
		return "TIME_SCOPED"
	case ScopeJob:
		return "JOB_SCOPED"
	case ScopeWorkflow:
		return "WORKFLOW_SCOPED"
	case ScopePipeline:
		return "PIPELINE_SCOPED"
	case ScopeRevision:
		return "REVISION_SCOPED"
	case Unscoped:
	}
	return "UNSCOPED"
}

// limit says what a key of this scope cannot do, in the owner's wording.
func (s Scope) limit() string {
	switch s {
	case ScopeTime:
		return "embeds the time of each cache step, so exact matches across jobs are effectively impossible"
	case ScopeJob:
		return "normally can't reuse a cache from an earlier job"
	case ScopeWorkflow:
		return "can't reuse caches from another workflow"
	case ScopePipeline:
		return "can't reuse caches from another pipeline"
	case ScopeRevision:
		return "can't reuse caches from a different commit"
	case Unscoped:
	}
	return "is not scoped"
}

// needsDependencyPaths: a workflow-, pipeline- or revision-scoped cache is
// a normal way to pass build output between jobs or workflows (the
// Workspaces research page: "for output shared across pipelines, use a
// cache keyed on the commit"), so these scopes are reported only for
// dependency caches.
func (s Scope) needsDependencyPaths() bool {
	return s == ScopeWorkflow || s == ScopePipeline || s == ScopeRevision
}

func parseToken(expr string) Token {
	t := Token{Expr: expr}
	switch {
	case strings.HasPrefix(expr, "checksum "):
		t.Kind, t.Arg = TokChecksum, strings.Trim(strings.TrimSpace(strings.TrimPrefix(expr, "checksum ")), `"'`)
	case expr == "epoch":
		t.Kind = TokEpoch
	case expr == ".Revision":
		t.Kind = TokRevision
	case expr == ".BuildNum":
		t.Kind = TokBuildNum
	case expr == ".Branch":
		t.Kind = TokBranch
	case expr == "arch":
		t.Kind = TokArch
	case strings.HasPrefix(expr, ".Environment."):
		t.Kind, t.Arg = TokEnv, strings.TrimPrefix(expr, ".Environment.")
		// The same value under another name is the same token, so a restore
		// of {{ .Revision }} matches a save of {{ .Environment.CIRCLE_SHA1 }}.
		switch t.Arg {
		case "CIRCLE_SHA1":
			t.Kind, t.Arg = TokRevision, ""
		case "CIRCLE_BUILD_NUM":
			t.Kind, t.Arg = TokBuildNum, ""
		}
	}
	return t
}

// known reports whether the model knows what the token is: a scoped token,
// or a stable one ({{ checksum }}, {{ .Branch }}, {{ arch }}). Any other
// environment value may come from a context or project setting. Before
// CIRCLE_PIPELINE_ID, CIRCLE_PIPELINE_NUMBER, CIRCLE_JOB_ID or
// CIRCLE_BUILD_URL are added, each must be verified to exist and its scope
// confirmed.
func (t Token) known() bool {
	switch t.Kind {
	case TokChecksum, TokEpoch, TokRevision, TokBuildNum, TokBranch, TokArch:
		return true
	case TokEnv:
		return t.Arg == "CIRCLE_WORKFLOW_ID"
	case TokOther:
	}
	return false
}

// Scope of one token; Unscoped for a stable or unknown one.
func (t Token) Scope() Scope {
	switch t.Kind {
	case TokEpoch:
		return ScopeTime
	case TokBuildNum:
		return ScopeJob
	case TokRevision:
		return ScopeRevision
	case TokEnv:
		if t.Arg == "CIRCLE_WORKFLOW_ID" {
			return ScopeWorkflow
		}
	case TokOther, TokChecksum, TokBranch, TokArch:
	}
	return Unscoped
}

// atom is one element of a typed template: a literal byte or a token.
type atom struct {
	char byte
	tok  *Token
}

// atoms splits template text into literal bytes and typed tokens. An
// unclosed `{{` makes the rest one TokOther token. A bare $VAR is literal
// text: environment values count only in the {{ .Environment.X }} form.
func atoms(text string) []atom {
	var out []atom
	for text != "" {
		before, rest, found := strings.Cut(text, "{{")
		for i := range len(before) {
			out = append(out, atom{char: before[i]})
		}
		if !found {
			break
		}
		expr, after, closed := strings.Cut(rest, "}}")
		if !closed {
			out = append(out, atom{tok: &Token{Kind: TokOther, Expr: rest}})
			break
		}
		t := parseToken(strings.TrimSpace(expr))
		out = append(out, atom{tok: &t})
		text = after
	}
	return out
}

// textScope scopes template text by its tokens. Within one key the
// narrowest token wins: "{{ epoch }}-{{ .Revision }}" is time-scoped. Any
// unknown token makes the text uninspectable: an unknown part next to a
// scoped one can cancel it ("{{ .BuildNum }}{{ .Environment.PAD }}" is
// "123" for 1+"23" and for 12+"3").
func textScope(text string) (Scope, bool) {
	scope := Unscoped
	for _, a := range atoms(text) {
		if a.tok == nil {
			continue
		}
		if !a.tok.known() {
			return Unscoped, false
		}
		scope = narrowest(scope, a.tok.Scope())
	}
	return scope, true
}

// narrowest returns the narrower of two scopes, ignoring Unscoped.
func narrowest(a, b Scope) Scope {
	switch {
	case a == Unscoped:
		return b
	case b == Unscoped:
		return a
	case b < a:
		return b
	}
	return a
}

// match is how a restore key relates to a save key.
type match int

const (
	noMatch match = iota
	// exactMatch: the same typed template.
	exactMatch
	// prefixMatch: the restore key is a typed prefix of the save key.
	prefixMatch
	// ambiguousMatch: a literal meets a token, or the same token kind has a
	// different argument, so the model cannot rule a match out.
	ambiguousMatch
)

// compare decides whether restore key r may match save key s. restore_cache
// matches when r's expansion is a prefix of a saved key's expansion.
// Different token kinds never match: an epoch and a checksum are not the
// same value (the flask-baseline and flask-optimized case).
func compare(r, s []atom) match {
	ambiguous := false
	for i := range r {
		if i >= len(s) {
			return noMatch
		}
		a, b := r[i], s[i]
		switch {
		case a.tok == nil && b.tok == nil:
			if a.char != b.char {
				return noMatch
			}
		case a.tok != nil && b.tok != nil:
			if a.tok.Kind != b.tok.Kind {
				return noMatch
			}
			if a.tok.Arg != b.tok.Arg || a.tok.Kind == TokOther && a.tok.Expr != b.tok.Expr {
				ambiguous = true
			}
		default:
			// A literal against a token's expansion. If the token can never
			// produce that character there is no match; otherwise nothing
			// after this point can be aligned.
			// lits is the literal side, from this position on.
			tok, char, lits := a.tok, b.char, s[i:]
			if tok == nil {
				tok, char, lits = b.tok, a.char, r[i:]
			}
			if !tok.canStartWith(char) || tok.hashCollision(lits) {
				return noMatch
			}
			return ambiguousMatch
		}
	}
	switch {
	case ambiguous:
		return ambiguousMatch
	case len(r) == len(s):
		return exactMatch
	}
	return prefixMatch
}

// canStartWith reports whether the token's expansion can begin with c:
// epoch and build numbers are decimal, a revision is lowercase hex, and a
// checksum is base64. Anything else can be any text.
func (t Token) canStartWith(c byte) bool {
	digit := c >= '0' && c <= '9'
	switch t.Kind {
	case TokEpoch, TokBuildNum:
		return digit
	case TokRevision:
		return digit || c >= 'a' && c <= 'f'
	case TokChecksum:
		return digit || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '+' || c == '/'
	case TokBranch, TokArch, TokEnv, TokOther:
	}
	return true
}

// hashLiteralRun is the length of a fixed literal a hash token is taken
// never to equal: a base64 SHA-256 checksum matches a given 12-character run
// with probability 64^-12, a hex revision with 16^-12.
const hashLiteralRun = 12

// hashCollision reports whether matching the hash token would need its
// expansion to equal a fixed literal run of at least hashLiteralRun
// characters, which is taken as impossible. A job called twice, once with
// "rd-<< pipeline.git.revision >>" (compiled to "rd-0123…") and once with
// "rd-{{ checksum … }}", then gives no false "may match" edge.
func (t Token) hashCollision(lits []atom) bool {
	if t.Kind != TokChecksum && t.Kind != TokRevision {
		return false
	}
	n := 0
	for _, a := range lits {
		if a.tok != nil {
			break
		}
		n++
	}
	return n >= hashLiteralRun
}
