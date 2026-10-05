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
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/syntax"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Install commands, from the team's cache criteria (2026-09-25): npm ci,
// yarn install, pnpm install, pip install -r, poetry install, bundle
// install, mvn, go mod download, cargo fetch, cargo build. Matching is on
// parsed command words, never substrings, so `echo "npm ci"` is not an
// install.
func installOf(words []string) string {
	words = stripWrappers(words)
	if len(words) > 0 && strings.HasPrefix(words[0], "python") && startsWith(words[1:], "-m", "pip") {
		words = append([]string{"pip"}, words[len([]string{"python", "-m", "pip"}):]...)
	}
	switch {
	case startsWith(words, "npm", "ci"):
		return "npm ci"
	case startsWith(words, "yarn", "install"), startsWith(words, "pnpm", "install"),
		startsWith(words, "poetry", "install"), startsWith(words, "bundle", "install"):
		return words[0] + " install"
	case startsWith(words, "pip", "install"), startsWith(words, "pip3", "install"):
		if slices.Contains(words, "-r") || slices.ContainsFunc(words, func(w string) bool {
			return strings.HasPrefix(w, "--requirement")
		}) {
			return "pip install -r"
		}
	case startsWith(words, "mvn"), startsWith(words, "mvnw"):
		if mavenGoal(words[1:]) {
			return "mvn"
		}
	case startsWith(words, "go", "mod", "download"):
		return "go mod download"
	case startsWith(words, "cargo", "fetch"), startsWith(words, "cargo", "build"):
		return strings.Join(words[:len([]string{"cargo", "fetch"})], " ")
	}
	return ""
}

// mavenValueOptions are Maven options whose value is a separate word.
var mavenValueOptions = []string{
	"-f", "--file", "-s", "--settings", "-gs", "--global-settings", "-pl", "--projects",
	"-rf", "--resume-from", "-t", "--toolchains", "-l", "--log-file", "-T", "--threads",
	"-b", "--builder", "-D", "--define", "-P", "--activate-profiles",
	"--color", "-gt", "--global-toolchains", "-emp", "--encrypt-master-password", "-ep", "--encrypt-password",
}

// mavenGoal reports whether a Maven command runs a goal, which resolves
// dependencies. --version, -v and help:… goals do not.
func mavenGoal(args []string) bool {
	goal := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v" || a == "--version" || a == "-h" || a == "--help":
			return false
		case slices.Contains(mavenValueOptions, a):
			i++ // skip the option's value
		case strings.HasPrefix(a, "-"):
		case !strings.HasPrefix(a, "help:"):
			goal = true
		}
	}
	return goal
}

// wrapperArgs are the options of each wrapper that take a value, e.g.
// `sudo -u circleci` or `env -u GOPROXY`.
var wrapperArgs = map[string][]string{
	"sudo": {
		"-u", "-g", "-C", "-h", "-p", "-U", "-r", "-t", "-D",
		"--user", "--group", "--close-from", "--host", "--prompt", "--other-user", "--role", "--type", "--chdir",
		"-R", "--chroot",
	},
	"env":     {"-u", "-C", "--unset", "--chdir", "-a", "--argv0"},
	"time":    {"-f", "-o"},
	"command": nil,
	"exec":    {"-a"},
}

// stripWrappers removes leading wrappers (sudo, env, time, command, exec),
// their options and variable assignments, and reduces a path-qualified
// program to its name.
func stripWrappers(words []string) []string {
	for len(words) > 0 {
		wrapper := path.Base(words[0])
		takesArg, isWrapper := wrapperArgs[wrapper]
		if !isWrapper {
			break
		}
		words = words[1:]
		for len(words) > 0 && (strings.HasPrefix(words[0], "-") || strings.Contains(words[0], "=")) {
			if wrapper == "env" && (words[0] == "-S" || words[0] == "--split-string") && len(words) > 1 {
				// env -S 'NODE_ENV=test npm ci': the value is split into
				// words, then read again as env's own arguments.
				split, ok := splitString(words[1])
				if !ok {
					return nil // env refuses the string, so nothing after it runs
				}
				words = append(append([]string{"env"}, split...), words[2:]...)
				break
			}
			if slices.Contains(takesArg, words[0]) && len(words) > 1 {
				words = words[1:]
			}
			words = words[1:]
		}
	}
	if len(words) > 0 && strings.Contains(words[0], "/") {
		words = append([]string{path.Base(words[0])}, words[1:]...)
	}
	return words
}

// startsWith reports whether words begins with prefix.
func startsWith(words []string, prefix ...string) bool {
	return len(words) >= len(prefix) && slices.Equal(words[:len(prefix)], prefix)
}

// commandsOf returns every simple command in a run step, as literal words.
// A word that is not a plain literal ends the command there.
func commandsOf(step pipelineconfig.Step) ([][]string, bool) {
	var script string
	switch {
	case step.Body == nil:
		return nil, true
	case step.Body.Kind == yaml.ScalarNode:
		script = step.Body.Value
	default:
		c := pipelineconfig.MapGet(step.Body, "command")
		if c == nil {
			return nil, true
		}
		script = c.Value
	}
	f, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return nil, false
	}
	// Walk the script in order, keeping the functions defined so far: a call
	// runs the body in force at that point, a later redefinition does not
	// change it, and a call before any definition is an ordinary command.
	funcs := map[string]*syntax.Stmt{}
	var out [][]string
	var walk func(n syntax.Node, calling map[string]bool)
	walk = func(n syntax.Node, calling map[string]bool) {
		syntax.Walk(n, func(n syntax.Node) bool {
			if fd, isFunc := n.(*syntax.FuncDecl); isFunc {
				funcs[fd.Name.Value] = fd.Body
				return false
			}
			call, ok := n.(*syntax.CallExpr)
			if !ok {
				return true
			}
			var words []string
			for _, w := range call.Args {
				lit, ok := wordText(w)
				if !ok {
					break
				}
				words = append(words, lit)
			}
			if len(words) == 0 {
				return true
			}
			if body, isFunc := funcs[words[0]]; isFunc {
				if !calling[words[0]] {
					calling[words[0]] = true
					walk(body, calling)
					delete(calling, words[0])
				}
				return true
			}
			out = append(out, words)
			return true
		})
	}
	walk(f, map[string]bool{})
	return out, true
}

// isFunctionRun reports a `circleci run …` command: a CircleCI function,
// whose caching the model knows only from the registry.
func isFunctionRun(words []string) bool {
	return startsWith(stripWrappers(words), "circleci", "run")
}

// wordText returns a word's text when it has no expansion: plain,
// single-quoted, or double-quoted literal parts.
func wordText(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), len(w.Parts) > 0
}

// splitString splits env -S text the way GNU env does: on spaces and tabs,
// honouring single quotes, double quotes and the escapes env defines (\_
// separates words outside quotes). Shell operators such as ";" are ordinary
// characters. Input env would reject (an unknown escape, a trailing
// backslash, an unclosed quote) gives no words, so no command is claimed.
func splitString(text string) ([]string, bool) {
	var words []string
	var b strings.Builder
	inWord := false
	var quote byte
	flush := func() {
		if inWord {
			words, inWord = append(words, b.String()), false
			b.Reset()
		}
	}
	escapes := map[byte]string{'\\': "\\", '\'': "'", '"': "\"", '#': "#", '$': "$", 't': "\t", 'n': "\n", 'r': "\r", 'f': "\f", 'v': "\v"}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote, inWord = c, true
		case c == '\\' && quote != '\'':
			if i+1 >= len(text) {
				return nil, false
			}
			i++
			if text[i] == '_' && quote == 0 {
				flush()
				continue
			}
			e, ok := escapes[text[i]]
			if !ok {
				return nil, false
			}
			b.WriteString(e)
			inWord = true
		case quote == 0 && (c == ' ' || c == '\t'):
			flush()
		default:
			b.WriteByte(c)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, false
	}
	flush()
	return words, true
}
