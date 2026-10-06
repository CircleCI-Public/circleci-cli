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

package dlc

import (
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/shellword"
)

// Code says why part of a job could not be inspected. Codes are metadata on a
// DLC_UNVERIFIABLE finding, not a disposition, and the report ranks by them.
type Code string

// Reason codes.
const (
	CodePackageLifecycle Code = "PACKAGE_LIFECYCLE" // installs that run package lifecycle scripts
	CodeTestRunner       Code = "TEST_RUNNER"       // tests can build images (e.g. Testcontainers)
	CodePackageScript    Code = "PACKAGE_SCRIPT"    // npm run, yarn <script>, npx
	CodeTaskRunner       Code = "TASK_RUNNER"       // make, gradle, go tool
	CodeExternalScript   Code = "EXTERNAL_SCRIPT"   // a script file, an interpreter running project code, git hooks
	CodeComposeUp        Code = "COMPOSE_UP"        // compose builds missing service images
	CodeContainerRun     Code = "CONTAINER_RUN"     // a container can reach the Docker daemon
	CodeDynamicCommand   Code = "DYNAMIC_COMMAND"   // command text built at run time
	CodeShellState       Code = "SHELL_STATE"       // aliases, shadowing functions, PATH, $BASH_ENV
	CodeUnknownCommand   Code = "UNKNOWN_COMMAND"   // not on the allowlist
	CodeUnsupportedShell Code = "UNSUPPORTED_SHELL" // a run step under a shell other than bash or sh
	CodeUnsupportedStep  Code = "UNSUPPORTED_STEP"  // a step type this module cannot read
	CodeUnparseable      Code = "UNPARSEABLE"       // the shell could not be parsed
)

// codePhrase explains a code after a command name, e.g. "go test runs tests,
// which can build images".
var codePhrase = map[Code]string{
	CodePackageLifecycle: "runs package build or lifecycle scripts",
	CodeTestRunner:       "runs tests, which can build images",
	CodePackageScript:    "runs a package script",
	CodeTaskRunner:       "runs a tool or target the config does not show",
	CodeExternalScript:   "runs project code",
	CodeContainerRun:     "runs a container that can reach the Docker daemon",
}

// opaque is one reason part of a script could not be inspected.
type opaque struct {
	code Code
	why  string
}

// scan is what one shell script revealed.
type scan struct {
	// builds are the Docker image builds found, e.g. "docker build".
	builds []string
	opaque []opaque
}

func (s *scan) merge(o scan) {
	s.builds = append(s.builds, o.builds...)
	s.opaque = append(s.opaque, o.opaque...)
}

func clean() scan { return scan{} }

func hidden(code Code, why string) scan { return scan{opaque: []opaque{{code, why}}} }

func built(what string) scan { return scan{builds: []string{what}} }

// scanScript parses a shell script and classifies every command in it,
// including those in command substitutions, subshells, functions, traps and
// conditionals. Matching is on parsed words, never on text.
//
// Only commands on the allowlist in this file count as clean. Anything else
// makes the job uninspectable, so a command config optimize cannot see
// through is never read as "no build".
func scanScript(script string) scan {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return hidden(CodeUnparseable, "the shell script could not be parsed")
	}
	// Only a function declared unconditionally at the top level is known to
	// exist when it is called. A call to one declared anywhere else is read
	// as an unknown command, because the name may resolve to a program.
	functions := map[string]bool{}
	for _, stmt := range file.Stmts {
		if fn, ok := stmt.Cmd.(*syntax.FuncDecl); ok {
			functions[fn.Name.Value] = true
		}
	}
	var out scan
	syntax.Walk(file, func(node syntax.Node) bool {
		if fn, ok := node.(*syntax.FuncDecl); ok && handlerFor(fn.Name.Value) != nil {
			out.merge(hidden(CodeShellState, "a function shadows the command "+fn.Name.Value))
		}
		return true
	})
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.Stmt:
			if s, ok := bashEnvWrite(n); ok {
				out.merge(s)
				return false
			}
		case *syntax.CallExpr:
			out.merge(checkAssigns(n.Assigns))
			if len(n.Args) > 0 {
				out.merge(classifyCall(n.Args, functions))
			}
		case *syntax.DeclClause:
			out.merge(checkAssigns(n.Args))
		}
		return true
	})
	return out
}

// bashEnvWrite handles `echo '...' >> $BASH_ENV`. Text written there runs at
// the start of every later step, so it is scanned as a script; anything but a
// literal echo or printf is uninspectable.
func bashEnvWrite(stmt *syntax.Stmt) (scan, bool) {
	for _, r := range stmt.Redirs {
		if r.Word == nil || !isParamWord(r.Word, "BASH_ENV") {
			continue
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return hidden(CodeShellState, "writes computed text to $BASH_ENV"), true
		}
		args, ok := literals(call.Args)
		if !ok || (args[0] != "echo" && args[0] != "printf") {
			return hidden(CodeShellState, "writes computed text to $BASH_ENV"), true
		}
		return scanScript(strings.Join(args[1:], " ")), true
	}
	return scan{}, false
}

// execEnv are environment variables that change what programs run: shell
// startup files, command lookup, dynamic loading, and tool hooks.
var execEnv = []string{
	"BASH_ENV", "ENV", "PATH", "IFS", "CDPATH", "PROMPT_COMMAND", "SHELLOPTS", "BASHOPTS",
	"GOFLAGS", "GOTOOLCHAIN", "CC", "CXX", "NODE_OPTIONS", "RUBYOPT", "PERL5OPT", "PERL5LIB",
	"PYTHONSTARTUP", "PYTHONPATH", "PYTHONHOME", "TAR_OPTIONS", "DOCKER_HOST", "DOCKER_CONTEXT",
	"DOCKER_CONFIG", "CURL_HOME", "WGETRC", "PS4", "GOENV",
}

// execEnvPrefixes are variable name prefixes with the same effect.
var execEnvPrefixes = []string{"LD_", "DYLD_", "GIT_", "npm_config_", "NPM_CONFIG_", "BASH_FUNC_"}

// isExecEnv reports whether setting the variable can change what runs.
func isExecEnv(name string) bool {
	if slices.Contains(execEnv, name) {
		return true
	}
	return slices.ContainsFunc(execEnvPrefixes, func(p string) bool { return strings.HasPrefix(name, p) })
}

// isExecEnvValue is isExecEnv with the value known. GOFLAGS only matters when
// it names a program to run; a computed value is assumed to.
func isExecEnvValue(name, value string) bool {
	if name == "GOFLAGS" {
		return value == dynamicArg || slices.ContainsFunc([]string{"-toolexec", "-vettool", "-exec", "-overlay"},
			func(f string) bool { return strings.Contains(value, f) })
	}
	return isExecEnv(name)
}

// checkAssigns flags an assignment that can change what later commands run.
// PATH is allowed only when it keeps the existing PATH first.
func checkAssigns(assigns []*syntax.Assign) scan {
	for _, a := range assigns {
		if a.Name == nil {
			continue
		}
		value := dynamicArg
		if a.Value != nil {
			if lit, ok := shellword.Literal(a.Value); ok {
				value = lit
			}
		} else if !a.Naked {
			value = ""
		}
		if !isExecEnvValue(a.Name.Value, value) {
			continue
		}
		if a.Name.Value == "PATH" && a.Value != nil && len(a.Value.Parts) > 0 && isParamPart(a.Value.Parts[0], "PATH") {
			continue
		}
		return hidden(CodeShellState, a.Name.Value+" is set, which can change what later commands run")
	}
	return clean()
}

// classifyCall classifies one simple command given its words.
func classifyCall(words []*syntax.Word, functions map[string]bool) scan {
	for _, w := range words {
		if mentionsParam(w, "BASH_ENV") {
			// Anything that can write the file (cp, tee, curl -o) injects
			// code into every later step.
			return hidden(CodeShellState, "a command reads or writes $BASH_ENV, which runs in every later step")
		}
	}
	args := make([]string, 0, len(words))
	for i, w := range words {
		lit, ok := shellword.Literal(w)
		if !ok {
			if i == 0 {
				return hidden(CodeDynamicCommand, "a command name is computed at run time")
			}
			// A dynamic argument cannot turn a non-build into a build, but it
			// can hide a subcommand: keep a marker the handlers refuse.
			lit = dynamicArg
		}
		args = append(args, lit)
	}
	if functions[args[0]] {
		// A call to a function defined in the script: its body is walked on
		// its own, and a function shadowing a known command is flagged there.
		return clean()
	}
	return classifyArgs(args)
}

// dynamicArg stands in for an argument known only at run time.
const dynamicArg = "\x00dynamic"

// handler classifies a command from its arguments, not including its name.
type handler func(args []string) scan

func classifyArgs(args []string) scan {
	if len(args) == 0 {
		return clean()
	}
	name, rest := args[0], args[1:]
	if name == dynamicArg {
		return hidden(CodeDynamicCommand, "a command name is computed at run time")
	}
	for _, a := range rest {
		if strings.Contains(a, "docker.sock") || a == "--unix-socket" || strings.HasPrefix(a, "--unix-socket=") {
			return hidden(CodeContainerRun, name+" talks to the Docker daemon API directly")
		}
	}
	if h := handlerFor(name); h != nil {
		out := h(rest)
		// A handler proves a command clean from its literal arguments. A
		// computed argument could be an option or subcommand that changes
		// what runs, so only commands that run nothing are exempt.
		if len(out.builds) == 0 && len(out.opaque) == 0 && slices.Contains(rest, dynamicArg) &&
			!slices.Contains(cleanCommands, name) && !slices.Contains(literalSubcommandTools, name) {
			return hidden(CodeDynamicCommand, name+" has an argument computed at run time")
		}
		return out
	}
	if strings.Contains(name, "/") {
		return hidden(CodeExternalScript, "runs the script "+name)
	}
	return hidden(CodeUnknownCommand, name+" is not a command this tool can inspect")
}

// The allowlist. A command missing from every list and from handlerFor is
// uninspectable. TestAllowlist has a case for every entry.
var (
	// cleanCommands run no other program and cannot build an image.
	cleanCommands = []string{
		":", "true", "false", "echo", "printf", "cd", "pwd", "export", "readonly",
		"unset", "set", "shift", "test", "[", "exit", "return", "local", "declare",
		"typeset", "wait", "sleep", "read", "unalias", "type", "which", "printenv",
		"mkdir", "rmdir", "touch", "chmod", "chown", "chgrp", "ln", "cat", "ls",
		"cp", "mv", "rm", "install", "mktemp", "basename", "dirname", "realpath",
		"readlink", "stat", "file", "head", "tail", "wc", "sort", "uniq", "cut",
		"tr", "paste", "join", "column", "fold", "nl", "od", "xxd", "hexdump",
		"grep", "egrep", "fgrep", "diff", "cmp", "comm", "tee", "seq", "date",
		"whoami", "id", "uname", "hostname", "df", "du", "free", "nproc", "ps",
		"kill", "pkill", "ulimit", "umask", "sync", "md5sum", "sha1sum",
		"sha256sum", "shasum", "base64", "gzip", "gunzip", "zcat", "bzip2", "xz",
		"zip", "unzip", "jq", "yq",
	}
	// shellStateCommands change how later command names resolve.
	shellStateCommands = []string{"alias", "hash", "enable"}
	// taskRunners run targets defined outside the config.
	taskRunners = []string{
		"make", "gmake", "just", "task", "go-task", "rake", "invoke", "gradle",
		"mvn", "bazel", "bazelisk", "earthly", "skaffold", "tilt", "mage", "nx",
		"turbo", "lerna", "ant", "sbt", "cmake", "ninja",
	}
	// testRunners run tests, which can build images themselves.
	testRunners = []string{
		"pytest", "py.test", "jest", "vitest", "mocha", "karma", "cypress",
		"playwright", "rspec", "cucumber", "phpunit", "tox", "nox", "ctest",
		"bats", "ava", "ginkgo", "gotestsum",
	}
	// interpreters run project code when given a program.
	interpreters = []string{
		"python", "python2", "python3", "node", "ruby", "perl", "php", "deno",
		"tsx", "ts-node", "pwsh", "powershell", "Rscript", "java", "lua",
	}
	// shells run a script; supportedShells are the ones this module reads.
	shells          = []string{"bash", "sh", "zsh", "dash", "ksh"}
	supportedShells = []string{"bash", "sh"}
	// packageRunners always run a package's code.
	packageRunners = []string{"npx", "pnpx", "bunx", "uvx"}
	// specialCommands have their own handler in handlerFor.
	specialCommands = []string{
		"docker", "docker-compose", "podman", "nerdctl", "buildah", "pack",
		"npm", "yarn", "pnpm", "bun", "pip", "pip3", "uv", "poetry", "pipenv",
		"bundle", "gem", "composer", "cargo", "go", "git", "tar", "sed", "awk",
		"gawk", "mawk", "nawk", "find", "xargs", "parallel", "trap", "eval",
		"source", ".", "sudo", "env", "nice", "timeout", "time", "nohup", "exec",
		"command", "builtin", "stdbuf", "ionice", "circleci", "circleci-agent", "curl", "wget",
	}
)

// literalSubcommandTools are handlers that refuse a computed subcommand
// themselves, so a computed operand after a literal subcommand (docker login
// -u "$USER") cannot change what runs.
var literalSubcommandTools = []string{"docker", "docker-compose", "git", "podman", "nerdctl", "buildah", "pack"}

// handlerFor returns the handler for a command name, or nil if the command is
// not on the allowlist.
func handlerFor(name string) handler {
	for _, lookup := range []func(string) handler{listHandler, packageHandler, toolHandler, shellToolHandler, cliHandler} {
		if h := lookup(name); h != nil {
			return h
		}
	}
	return nil
}

// listHandler covers the commands classified by list membership.
func listHandler(name string) handler {
	switch {
	case slices.Contains(cleanCommands, name):
		return func([]string) scan { return clean() }
	case slices.Contains(shellStateCommands, name):
		return func([]string) scan { return hidden(CodeShellState, name+" changes how later commands resolve") }
	case slices.Contains(taskRunners, name):
		return versionOr(hidden(CodeTaskRunner, name+" runs targets the config does not show"))
	case slices.Contains(testRunners, name):
		return versionOr(hidden(CodeTestRunner, name+" runs tests, which can build images"))
	case slices.Contains(interpreters, name):
		return interpreter(name)
	case slices.Contains(shells, name):
		return shell(name)
	case slices.Contains(packageRunners, name):
		return versionOr(hidden(CodePackageScript, name+" runs a package's code"))
	}
	return nil
}

// packageHandler covers package managers and language toolchains.
func packageHandler(name string) handler {
	switch name {
	case "npm":
		return classifyNpm
	case "yarn":
		return classifyYarn
	case "pnpm":
		return classifyPnpm
	case "bun":
		return subcommands("bun", map[string]Code{
			"install": CodePackageLifecycle, "i": CodePackageLifecycle, "add": CodePackageLifecycle,
			"update": CodePackageLifecycle, "remove": CodePackageLifecycle, "test": CodeTestRunner,
		}, &opaque{CodePackageScript, "bun runs a package script or file"})
	case "pip", "pip3":
		return subcommands(name, map[string]Code{
			"install": CodePackageLifecycle, "download": CodePackageLifecycle, "wheel": CodePackageLifecycle,
		}, nil, "list", "freeze", "show", "config", "cache", "check", "debug", "inspect", "uninstall", "help")
	case "uv":
		return subcommands("uv", map[string]Code{
			"sync": CodePackageLifecycle, "add": CodePackageLifecycle, "lock": CodePackageLifecycle,
			"pip": CodePackageLifecycle, "run": CodeExternalScript, "tool": CodeExternalScript,
		}, nil, "version", "cache", "python", "help")
	case "poetry", "pipenv":
		return subcommands(name, map[string]Code{
			"install": CodePackageLifecycle, "add": CodePackageLifecycle, "update": CodePackageLifecycle,
			"lock": CodePackageLifecycle, "sync": CodePackageLifecycle, "run": CodeExternalScript,
		}, nil, "check", "show", "config", "env", "help", "graph", "requirements")
	case "bundle":
		return subcommands("bundle", map[string]Code{
			"install": CodePackageLifecycle, "update": CodePackageLifecycle, "exec": CodeExternalScript,
		}, nil, "config", "check", "list", "show", "help")
	case "gem":
		return subcommands("gem", map[string]Code{"install": CodePackageLifecycle, "update": CodePackageLifecycle},
			nil, "list", "sources", "env", "help")
	case "composer":
		return subcommands("composer", map[string]Code{
			"install": CodePackageLifecycle, "update": CodePackageLifecycle, "require": CodePackageLifecycle,
			"run-script": CodePackageScript, "run": CodePackageScript, "exec": CodePackageScript,
		}, nil, "validate", "show", "config", "diagnose", "help")
	case "cargo":
		// Build scripts (build.rs) run during every build.
		return subcommands("cargo", map[string]Code{
			"build": CodePackageLifecycle, "check": CodePackageLifecycle, "doc": CodePackageLifecycle,
			"install": CodePackageLifecycle, "clippy": CodePackageLifecycle, "test": CodeTestRunner,
			"nextest": CodeTestRunner, "bench": CodeTestRunner, "run": CodeExternalScript,
		}, nil, "fmt", "metadata", "tree", "fetch", "help")
	case "go":
		inner := subcommands("go", map[string]Code{
			"test": CodeTestRunner, "run": CodeExternalScript, "generate": CodeExternalScript, "tool": CodeTaskRunner,
		}, nil, "build", "install", "vet", "mod", "fmt", "version", "env", "list", "doc", "clean", "work", "get", "help")
		return func(args []string) scan {
			if len(args) > 1 && args[0] == "-C" {
				args = args[1:][1:]
			}
			for _, a := range args {
				for _, flag := range []string{"-toolexec", "-vettool", "-exec", "-overlay"} {
					if a == flag || strings.HasPrefix(a, flag+"=") || a == "-"+flag || strings.HasPrefix(a, "-"+flag+"=") {
						return hidden(CodeExternalScript, "go "+flag+" runs an external program")
					}
				}
			}
			return inner(args)
		}
	}
	return nil
}

// toolHandler covers container engines, shell tools and wrappers.
func toolHandler(name string) handler {
	switch name {
	case "docker":
		return classifyDocker
	case "docker-compose":
		return func(args []string) scan { return classifyCompose("docker-compose", args) }
	case "podman", "nerdctl":
		return otherEngine(name)
	case "buildah":
		// buildah builds into containers/storage, which DLC does not cache;
		// only a container it runs can reach the Docker daemon.
		return func(args []string) scan {
			if firstNonFlag(args) == "run" {
				return hidden(CodeContainerRun, "buildah run runs a container that can reach the Docker daemon")
			}
			return clean()
		}
	case "pack":
		return func(args []string) scan {
			if firstNonFlag(args) == "build" {
				return built("pack build")
			}
			return clean()
		}
	case "git":
		return classifyGit
	case "tar":
		return classifyTar
	case "sed":
		return classifySed
	case "awk", "gawk", "mawk", "nawk":
		return classifyAwk
	case "find":
		return func(args []string) scan {
			for _, a := range args {
				if a == "-exec" || a == "-execdir" || a == "-ok" || a == "-okdir" {
					return hidden(CodeDynamicCommand, "find "+a+" runs commands built at run time")
				}
			}
			return clean()
		}
	}
	return nil
}

// shellToolHandler covers shell builtins and wrappers that run other commands.
func shellToolHandler(name string) handler {
	switch name {
	case "xargs":
		return classifyXargs
	case "parallel":
		return func([]string) scan { return hidden(CodeDynamicCommand, "parallel builds commands at run time") }
	case "trap":
		return classifyTrap
	case "eval":
		return func(args []string) scan {
			if slices.Contains(args, dynamicArg) {
				return hidden(CodeDynamicCommand, "eval runs text built at run time")
			}
			return scanScript(strings.Join(args, " "))
		}
	case "source", ".":
		return func([]string) scan { return hidden(CodeExternalScript, "sources a script file") }
	case "sudo", "env", "nice", "timeout", "time", "nohup", "exec", "command", "builtin", "stdbuf", "ionice":
		return wrapper(name)
	}
	return nil
}

// cliHandler covers network clients and the CircleCI CLI.
func cliHandler(name string) handler {
	switch name {
	case "curl", "wget":
		// curl -K/--config and wget -e/--execute/--config read options from a
		// file or string, which can point either at the Docker daemon.
		short, long := []string{"-K"}, []string{"--config"}
		if name == "wget" {
			short, long = []string{"-e"}, []string{"--config", "--execute"}
		}
		return func(args []string) scan {
			for _, a := range args {
				// A short cluster (-sKfile, -qe'...') hides the option letter.
				cluster := strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--")
				isShort := slices.ContainsFunc(short, func(o string) bool {
					return strings.HasPrefix(a, o) || cluster && strings.Contains(a[1:], o[1:])
				})
				isLong := slices.ContainsFunc(long, func(o string) bool { return a == o || strings.HasPrefix(a, o+"=") })
				if isShort || isLong {
					return hidden(CodeExternalScript, name+" "+a+" reads options from a file")
				}
			}
			return clean()
		}
	case "circleci":
		return func(args []string) scan {
			if len(args) >= 1 && args[0] == "tests" && firstNonFlag(args[1:]) != "run" {
				return clean()
			}
			if isVersionOrHelp(args) {
				return clean()
			}
			return hidden(CodeUnknownCommand, "circleci "+firstNonFlag(args)+" is not a command this tool can inspect")
		}
	case "circleci-agent":
		return func(args []string) scan {
			if len(args) >= 1 && args[0] == "step" {
				return clean()
			}
			return hidden(CodeUnknownCommand, "circleci-agent "+firstNonFlag(args)+" is not a command this tool can inspect")
		}
	}
	return nil
}

// versionOr is clean for --version or --help, and otherwise returns s.
func versionOr(s scan) handler {
	return func(args []string) scan {
		if isVersionOrHelp(args) {
			return clean()
		}
		return s
	}
}

func isVersionOrHelp(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "--version", "-v", "-V", "version", "--help", "-h", "help":
		return true
	}
	return false
}

// subcommands classifies `<name> <sub> ...`: subs in codes are uninspectable
// with that code, subs in safe are clean, and anything else is fallback (or
// unknown when fallback is nil).
func subcommands(name string, codes map[string]Code, fallback *opaque, safe ...string) handler {
	return func(args []string) scan {
		if isVersionOrHelp(args) || len(args) == 0 {
			return clean()
		}
		if strings.HasPrefix(args[0], "-") {
			// A valued option before the subcommand would shift it; they are
			// not parsed, so refuse rather than guess.
			return hidden(CodeDynamicCommand, name+" is called with options before its subcommand")
		}
		sub := args[0]
		if code, ok := codes[sub]; ok {
			return hidden(code, name+" "+sub+" "+codePhrase[code])
		}
		if slices.Contains(safe, sub) || sub == "" {
			return clean()
		}
		if fallback != nil {
			return scan{opaque: []opaque{*fallback}}
		}
		return hidden(CodeUnknownCommand, name+" "+sub+" is not a command this tool can inspect")
	}
}

func interpreter(name string) handler {
	return func(args []string) scan {
		if isVersionOrHelp(args) {
			return clean()
		}
		if len(args) == 0 {
			return hidden(CodeExternalScript, name+" reads a program from stdin")
		}
		if len(args) > 1 && args[0] == "-m" {
			module, moduleArgs := args[1], args[1:]
			moduleArgs = moduleArgs[1:]
			switch module {
			case "pytest":
				return hidden(CodeTestRunner, name+" -m pytest runs tests, which can build images")
			case "pip":
				if firstNonFlag(moduleArgs) == "install" {
					return hidden(CodePackageLifecycle, name+" -m pip install runs package build scripts")
				}
			}
		}
		return hidden(CodeExternalScript, name+" runs project code")
	}
}

func shell(name string) handler {
	return func(args []string) scan {
		if isVersionOrHelp(args) {
			return clean()
		}
		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "-c" || strings.HasPrefix(args[i], "-") && !strings.HasPrefix(args[i], "--") && strings.HasSuffix(args[i], "c"):
				if i+1 >= len(args) || args[i+1] == dynamicArg {
					return hidden(CodeDynamicCommand, name+" -c runs text built at run time")
				}
				return scanScript(args[i+1])
			case strings.HasPrefix(args[i], "-"):
				continue
			default:
				return hidden(CodeExternalScript, name+" runs the script "+args[i])
			}
		}
		return hidden(CodeExternalScript, name+" reads a script from stdin")
	}
}

// wrapperOptions lists, per wrapper, the options that take a value and those
// that do not. An option in neither makes the command uninspectable, because
// it could shift which word is the wrapped command.
var wrapperOptions = map[string]struct{ valued, boolean []string }{
	"sudo": {
		valued: []string{"-u", "--user", "-g", "--group", "-h", "--host", "-C", "--close-from", "-p", "--prompt",
			"-U", "--other-user", "-r", "--role", "-t", "--type", "-D", "--chdir", "-R", "--chroot", "-T", "--command-timeout"},
		boolean: []string{"-E", "--preserve-env", "-H", "--set-home", "-n", "--non-interactive", "-S", "--stdin",
			"-k", "--reset-timestamp", "-b", "--background", "-i", "--login", "-s", "--shell", "-A", "--askpass", "-P", "--preserve-groups"},
	},
	"env":     {valued: []string{"-u", "--unset", "-C", "--chdir"}, boolean: []string{"-i", "--ignore-environment", "-0", "--null"}},
	"nice":    {valued: []string{"-n", "--adjustment"}},
	"timeout": {valued: []string{"-s", "--signal", "-k", "--kill-after"}, boolean: []string{"-v", "--verbose", "--preserve-status", "--foreground"}},
	"time":    {boolean: []string{"-p"}},
	"nohup":   {},
	"exec":    {valued: []string{"-a"}, boolean: []string{"-c", "-l"}},
	"command": {boolean: []string{"-p"}},
	"builtin": {},
	"stdbuf":  {valued: []string{"-i", "--input", "-o", "--output", "-e", "--error"}},
	"ionice":  {valued: []string{"-c", "--class", "-n", "--classdata", "-p", "--pid"}, boolean: []string{"-t", "--ignore"}},
}

// wrapper runs another command: classify that command.
func wrapper(name string) handler {
	return func(args []string) scan {
		if name == "command" && len(args) >= 1 && (args[0] == "-v" || args[0] == "-V") {
			return clean() // command -v only looks a name up
		}
		opts := wrapperOptions[name]
		rest, ok := skipKnownOptions(args, opts.valued, opts.boolean)
		if !ok {
			return hidden(CodeDynamicCommand, name+" is called with an option this tool does not recognise")
		}
		if name == "env" {
			for len(rest) > 0 && strings.Contains(rest[0], "=") {
				varName, value, _ := strings.Cut(rest[0], "=")
				if isExecEnvValue(varName, value) && (varName != "PATH" || !strings.HasPrefix(value, "$PATH")) {
					return hidden(CodeShellState, "env sets "+varName+", which can change what the command runs")
				}
				rest = rest[1:]
			}
		}
		if name == "timeout" && len(rest) > 0 {
			rest = rest[1:] // the duration
		}
		if len(rest) == 0 {
			if name == "sudo" && slices.ContainsFunc(args, func(a string) bool {
				return a == "-s" || a == "-i" || a == "--shell" || a == "--login"
			}) {
				return hidden(CodeExternalScript, "sudo starts a shell that reads commands from stdin")
			}
			return clean()
		}
		return classifyArgs(rest)
	}
}

var (
	dockerValued  = []string{"--config", "-c", "--context", "-H", "--host", "-l", "--log-level", "--tlscacert", "--tlscert", "--tlskey"}
	dockerBoolean = []string{"-D", "--debug", "--tls", "--tlsverify"}
	buildxValued  = []string{"--builder"}
	composeValued = []string{
		"-f", "--file", "-p", "--project-name", "--profile", "--project-directory",
		"--env-file", "--ansi", "--progress", "--parallel",
	}
	composeBoolean = []string{"--compatibility", "--dry-run", "--all-resources", "--verbose"}

	// dockerSafe are docker subcommands that run no container and build nothing.
	dockerSafe = []string{
		"pull", "push", "login", "logout", "tag", "images", "ps", "version", "info", "inspect",
		"rm", "rmi", "logs", "stop", "kill", "wait", "top", "stats", "events", "history",
		"search", "save", "load", "cp", "port", "pause", "unpause", "rename", "update", "diff",
		"export", "import", "commit", "network", "volume", "system", "context", "manifest", "trust",
	}
	// dockerRuns start a container, which can reach the daemon and build.
	dockerRuns = []string{"run", "exec", "create", "start", "restart", "attach"}
)

func classifyDocker(args []string) scan {
	if slices.Contains(args, "--help") {
		return clean()
	}
	rest, ok := skipKnownOptions(args, dockerValued, append([]string{"-v", "--version"}, dockerBoolean...))
	if !ok {
		return hidden(CodeDynamicCommand, "docker is called with an option this tool does not recognise")
	}
	if len(rest) == 0 {
		return clean()
	}
	sub := rest[0]
	switch {
	case sub == dynamicArg:
		return hidden(CodeDynamicCommand, "a docker subcommand is computed at run time")
	case sub == "build":
		return built("docker build")
	case sub == "compose":
		return classifyCompose("docker compose", rest[1:])
	case sub == "buildx":
		return classifyBuildx(rest[1:])
	case sub == "image" || sub == "builder":
		if len(rest) > 1 && rest[1] == "build" {
			return built("docker " + sub + " build")
		}
		return clean()
	case sub == "container":
		if len(rest) > 1 && slices.Contains(dockerRuns, rest[1]) {
			return hidden(CodeContainerRun, "docker container "+rest[1]+" starts a container that can reach the Docker daemon")
		}
		return clean()
	case slices.Contains(dockerRuns, sub):
		return hidden(CodeContainerRun, "docker "+sub+" starts a container that can reach the Docker daemon")
	case slices.Contains(dockerSafe, sub):
		return clean()
	}
	return hidden(CodeUnknownCommand, "docker "+sub+" is not a command this tool can inspect")
}

func classifyBuildx(args []string) scan {
	rest, ok := skipKnownOptions(args, buildxValued, nil)
	if !ok {
		return hidden(CodeDynamicCommand, "docker buildx is called with an option this tool does not recognise")
	}
	if len(rest) == 0 {
		return clean()
	}
	switch rest[0] {
	case "build", "b", "bake":
		return built("docker buildx " + rest[0])
	case "create", "use", "ls", "inspect", "rm", "stop", "prune", "du", "version", "imagetools":
		return clean()
	}
	return hidden(CodeUnknownCommand, "docker buildx "+rest[0]+" is not a command this tool can inspect")
}

func classifyCompose(prefix string, args []string) scan {
	if slices.Contains(args, "--help") {
		return clean()
	}
	rest, ok := skipKnownOptions(args, composeValued, composeBoolean)
	if !ok {
		return hidden(CodeDynamicCommand, prefix+" is called with an option this tool does not recognise")
	}
	if len(rest) == 0 {
		return clean()
	}
	sub := rest[0]
	switch sub {
	case dynamicArg:
		return hidden(CodeDynamicCommand, "a "+prefix+" subcommand is computed at run time")
	case "build":
		return built(prefix + " build")
	case "up", "run", "create":
		if slices.Contains(rest, "--build") {
			return built(prefix + " " + sub + " --build")
		}
		return hidden(CodeComposeUp, prefix+" "+sub+" builds missing service images, and the compose file is not in the config")
	case "start", "restart", "exec", "watch":
		return hidden(CodeContainerRun, prefix+" "+sub+" runs containers that can reach the Docker daemon")
	case "down", "ps", "logs", "pull", "push", "config", "stop", "rm", "kill", "version", "ls",
		"images", "port", "top", "pause", "unpause", "events", "cp", "wait":
		return clean()
	}
	return hidden(CodeUnknownCommand, prefix+" "+sub+" is not a command this tool can inspect")
}

// otherEngine handles podman and nerdctl. Their builds do not use Docker's
// image store, so DLC does not cache them; but a container they run can mount
// the Docker socket and build.
func otherEngine(name string) handler {
	return func(args []string) scan {
		sub := firstNonFlag(args)
		if sub == "container" && len(args) > 1 {
			sub = args[slices.Index(args, "container")+1]
		}
		switch sub {
		case "run", "exec", "create", "start", "restart", "attach", "compose":
			return hidden(CodeContainerRun, name+" "+sub+" runs containers that can reach the Docker daemon")
		}
		return clean()
	}
}

// npmLifecycle are npm subcommands that run package lifecycle scripts.
var npmLifecycle = []string{
	"ci", "install", "i", "isntall", "add", "update", "up", "upgrade", "rebuild", "rb",
	"link", "ln", "pack", "publish", "version", "dedupe", "prune", "uninstall", "remove", "rm", "un",
}

func classifyNpm(args []string) scan {
	if len(args) == 0 || isVersionOrHelp(args) {
		return clean()
	}
	if strings.HasPrefix(args[0], "-") {
		return hidden(CodeDynamicCommand, "npm is called with options before its subcommand")
	}
	sub := args[0]
	switch {
	case slices.Contains(npmLifecycle, sub):
		// npm documents --ignore-scripts as skipping lifecycle scripts for
		// this command; it is the only suppression flag accepted as proof,
		// and the last occurrence wins.
		if npmIgnoresScripts(args) {
			return clean()
		}
		return hidden(CodePackageLifecycle, "npm "+sub+" runs package lifecycle scripts")
	case sub == "test" || sub == "t" || sub == "tst" || sub == "install-test" || sub == "it":
		return hidden(CodeTestRunner, "npm "+sub+" runs the test script")
	case slices.Contains([]string{"run", "run-script", "rum", "urn", "start", "stop", "restart", "exec", "x", "explore"}, sub):
		return hidden(CodePackageScript, "npm "+sub+" runs a package script")
	case slices.Contains([]string{"config", "get", "set", "view", "info", "show", "ls", "list", "ll", "la", "outdated",
		"audit", "cache", "whoami", "ping", "doctor", "help", "prefix", "root", "bin", "search", "owner", "token",
		"login", "logout", "adduser", "fund", "query", "pkg", "sbom"}, sub):
		if sub == "audit" && slices.Contains(args, "fix") {
			return hidden(CodePackageLifecycle, "npm audit fix runs an install")
		}
		return clean()
	}
	return hidden(CodeUnknownCommand, "npm "+sub+" is not a command this tool can inspect")
}

func npmIgnoresScripts(args []string) bool {
	ignore := false
	for _, a := range args {
		switch a {
		case "--ignore-scripts", "--ignore-scripts=true":
			ignore = true
		case "--ignore-scripts=false", "--no-ignore-scripts":
			ignore = false
		}
	}
	return ignore
}

func classifyYarn(args []string) scan {
	if isVersionOrHelp(args) {
		return clean()
	}
	sub := firstNonFlag(args)
	switch {
	case sub == "" || slices.Contains([]string{"install", "add", "upgrade", "up", "remove", "import", "dedupe", "version"}, sub):
		return hidden(CodePackageLifecycle, "yarn "+pathOrInstall(sub)+" runs package lifecycle scripts")
	case sub == "test":
		return hidden(CodeTestRunner, "yarn test runs the test script")
	case slices.Contains([]string{"config", "cache", "info", "list", "why", "licenses", "owner", "login", "logout", "bin", "dir", "check"}, sub):
		return clean()
	}
	return hidden(CodePackageScript, "yarn "+sub+" runs a package script")
}

func classifyPnpm(args []string) scan {
	if isVersionOrHelp(args) {
		return clean()
	}
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return hidden(CodeDynamicCommand, "pnpm is called with options before its subcommand")
	}
	sub := firstNonFlag(args)
	switch {
	case slices.Contains([]string{"install", "i", "add", "update", "up", "remove", "rm", "rebuild", "rb", "import", "dedupe"}, sub):
		return hidden(CodePackageLifecycle, "pnpm "+sub+" runs package lifecycle scripts")
	case sub == "test" || sub == "t":
		return hidden(CodeTestRunner, "pnpm test runs the test script")
	case slices.Contains([]string{"store", "list", "ls", "why", "outdated", "audit", "config", "root", "bin", "fetch"}, sub):
		return clean()
	}
	return hidden(CodePackageScript, "pnpm "+pathOrInstall(sub)+" runs a package script")
}

func pathOrInstall(sub string) string {
	if sub == "" {
		return "install"
	}
	return sub
}

// gitHookSubcommands can run repository hooks (post-checkout, pre-commit,
// reference-transaction, ...) or commands the repository configures.
var gitHookSubcommands = []string{
	"checkout", "switch", "clone", "commit", "merge", "rebase", "pull", "push", "am",
	"cherry-pick", "revert", "worktree", "submodule", "gc", "reset", "restore", "stash",
	"lfs", "fetch", "tag", "branch", "clean", "add", "init",
}

// gitReadOnly subcommands only read the repository.
var gitReadOnly = []string{
	"status", "diff", "log", "rev-parse", "show", "describe", "remote", "ls-files", "ls-remote",
	"grep", "blame", "shortlog", "rev-list", "merge-base", "cat-file", "for-each-ref", "name-rev",
	"symbolic-ref", "version",
}

// gitConfigReads are `git config` forms that only read.
var gitConfigReads = []string{"--get", "--get-all", "--get-regexp", "--list", "-l"}

func classifyGit(args []string) scan {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || strings.HasPrefix(a, "-c") || strings.HasPrefix(a, "--config-env"):
			return hidden(CodeDynamicCommand, "git -c can set an alias or hooks path for this command")
		case a == "-C" || a == "--git-dir" || a == "--work-tree" || a == "--namespace":
			i++
			continue
		case strings.HasPrefix(a, "--git-dir=") || strings.HasPrefix(a, "--work-tree=") ||
			strings.HasPrefix(a, "--namespace=") || a == "--no-pager" || a == "-P" || a == "--bare":
			continue
		case strings.HasPrefix(a, "-"):
			return hidden(CodeDynamicCommand, "git is called with a global option this tool does not recognise")
		}
		sub, rest := a, args[i+1:]
		switch {
		case sub == "config":
			if slices.ContainsFunc(rest, func(r string) bool { return slices.Contains(gitConfigReads, r) }) {
				return clean()
			}
			return hidden(CodeShellState, "git config can install hooks or aliases for later git commands")
		case slices.Contains(gitHookSubcommands, sub):
			return hidden(CodeExternalScript, "git "+sub+" can run repository hooks")
		case slices.Contains(gitReadOnly, sub):
			return clean()
		}
		return hidden(CodeUnknownCommand, "git "+sub+" is not a command this tool can inspect (it may be an alias)")
	}
	return clean()
}

func classifyTar(args []string) scan {
	for i, a := range args {
		for _, p := range []string{"--to-command", "--use-compress-program", "--checkpoint-action", "--info-script",
			"--new-volume-script", "--rsh-command", "--rmt-command"} {
			if strings.HasPrefix(a, p) {
				return hidden(CodeDynamicCommand, "tar "+p+" runs a program")
			}
		}
		// A short cluster (-I, -Izstd, -cIf, or the traditional first
		// argument "cIf") containing I or F names a program to run.
		short := strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--")
		traditional := i == 0 && !strings.HasPrefix(a, "-")
		if (short || traditional) && strings.ContainsAny(a, "IF") {
			return hidden(CodeDynamicCommand, "tar -I or -F runs a program")
		}
	}
	return clean()
}

// sedSubstituteParts is how many pieces `s/pattern/replacement/flags` splits
// into on its delimiter: "", pattern, replacement, flags.
const sedSubstituteParts = 4

// sedRunsCommands reports whether a sed script uses the `e` command or the
// `e` flag of `s`, both of which run shell commands. A script it cannot split
// cleanly counts as running commands.
func sedRunsCommands(script string) bool {
	for _, cmd := range strings.FieldsFunc(script, func(r rune) bool { return r == ';' || r == '\n' || r == '{' || r == '}' }) {
		cmd = strings.TrimLeft(strings.TrimSpace(cmd), "0123456789$,!~+ ")
		for strings.HasPrefix(cmd, "/") || strings.HasPrefix(cmd, "\\") {
			if strings.HasPrefix(cmd, "\\") {
				return true // \cREGEXc addresses use a custom delimiter
			}
			parts := splitUnescaped(cmd, '/')
			if len(parts) < len([]string{"", "regex", "rest"}) {
				return true
			}
			cmd = strings.TrimLeft(strings.Join(parts[2:], "/"), "0123456789$,!~+IM ")
		}
		switch {
		case cmd == "e" || strings.HasPrefix(cmd, "e ") || strings.HasPrefix(cmd, "e\t"):
			return true
		case strings.HasPrefix(cmd, "s") && len(cmd) > len("s"):
			parts := splitUnescaped(cmd[len("s"):], cmd[len("s")])
			if len(parts) < sedSubstituteParts {
				return true
			}
			if strings.ContainsRune(parts[len(parts)-1], 'e') {
				return true
			}
		}
	}
	return false
}

// splitUnescaped splits s (starting with the delimiter) on unescaped delim.
func splitUnescaped(s string, delim byte) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
		case s[i] == delim:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	return append(parts, cur.String())
}

func classifySed(args []string) scan {
	var scripts []string
	explicit := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			continue
		case strings.HasPrefix(a, "--file") || strings.HasPrefix(a, "--expression"):
			if strings.HasPrefix(a, "--file") {
				return hidden(CodeExternalScript, "sed -f runs a sed script file")
			}
			explicit = true
			if v, ok := strings.CutPrefix(a, "--expression="); ok {
				scripts = append(scripts, v)
			} else if i+1 < len(args) {
				scripts = append(scripts, args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-") && len(a) > len("-"):
			// A short cluster such as -n, -ne 'script', -e'script' or -fFILE.
			for j := 1; j < len(a); j++ {
				switch a[j] {
				case 'f':
					return hidden(CodeExternalScript, "sed -f runs a sed script file")
				case 'e':
					explicit = true
					if rest := a[j+1:]; rest != "" {
						scripts = append(scripts, rest)
					} else if i+1 < len(args) {
						scripts = append(scripts, args[i+1])
						i++
					}
					j = len(a)
				}
			}
		case !explicit && len(scripts) == 0:
			scripts = append(scripts, a)
		}
	}
	for _, sc := range scripts {
		if sc == dynamicArg || sedRunsCommands(sc) {
			return hidden(CodeDynamicCommand, "sed can run shell commands with the e command")
		}
	}
	return clean()
}

func classifyAwk(args []string) scan {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v" || a == "-F":
			i++
		case strings.HasPrefix(a, "-v") || strings.HasPrefix(a, "-F"):
		case strings.HasPrefix(a, "-"):
			return hidden(CodeExternalScript, "awk "+a+" can load a program file or extension")
		default:
			if a == dynamicArg || strings.Contains(a, "system") || strings.Contains(a, "|") {
				return hidden(CodeDynamicCommand, "the awk program can run shell commands")
			}
			return clean()
		}
	}
	return clean()
}

func classifyXargs(args []string) scan {
	valued := []string{"-n", "-L", "-I", "-P", "-d", "-E", "-s", "-a", "--max-args", "--max-lines", "--max-procs", "--delimiter", "--arg-file"}
	boolean := []string{"-0", "--null", "-r", "--no-run-if-empty", "-t", "--verbose", "-p", "--interactive", "-x", "--exit"}
	rest, ok := skipKnownOptions(args, valued, boolean)
	if !ok {
		return hidden(CodeDynamicCommand, "xargs is called with an option this tool does not recognise")
	}
	if len(rest) == 0 {
		return clean() // xargs defaults to echo
	}
	// xargs appends words read from stdin: they can be a subcommand.
	return classifyArgs(append(slices.Clone(rest), dynamicArg))
}

func classifyTrap(args []string) scan {
	if len(args) == 0 || args[0] == "-p" || args[0] == "-l" || args[0] == "-" || args[0] == "" {
		return clean()
	}
	if args[0] == dynamicArg {
		return hidden(CodeDynamicCommand, "trap runs text built at run time")
	}
	return scanScript(args[0])
}

// skipKnownOptions drops leading options. It fails on an option it does not
// know, because an unknown valued option would shift the subcommand.
func skipKnownOptions(args, valued, boolean []string) ([]string, bool) {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		opt := args[0]
		if opt == "--" {
			return args[1:], true
		}
		name, _, hasValue := strings.Cut(opt, "=")
		switch {
		case slices.Contains(valued, name) && hasValue:
			args = args[1:]
		case slices.Contains(valued, name):
			if len(args) <= 1 {
				return nil, false
			}
			args = args[2:]
		case slices.Contains(boolean, name):
			args = args[1:]
		case len(name) > len("-o") && !strings.HasPrefix(name, "--") && slices.Contains(valued, name[:len("-o")]):
			// An attached short value, e.g. stdbuf -oL.
			args = args[1:]
		default:
			return nil, false
		}
	}
	return args, true
}

func firstNonFlag(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

func literals(words []*syntax.Word) ([]string, bool) {
	out := make([]string, 0, len(words))
	for _, w := range words {
		lit, ok := shellword.Literal(w)
		if !ok {
			return nil, false
		}
		out = append(out, lit)
	}
	return out, true
}

// isParamWord reports whether a word is exactly $name or "$name".
func isParamWord(w *syntax.Word, name string) bool {
	if len(w.Parts) != 1 {
		return false
	}
	if dq, ok := w.Parts[0].(*syntax.DblQuoted); ok {
		return len(dq.Parts) == 1 && isParamPart(dq.Parts[0], name)
	}
	return isParamPart(w.Parts[0], name)
}

// mentionsParam reports whether a word expands $name anywhere.
func mentionsParam(w *syntax.Word, name string) bool {
	found := false
	syntax.Walk(w, func(n syntax.Node) bool {
		if pe, ok := n.(*syntax.ParamExp); ok && pe.Param != nil && pe.Param.Value == name {
			found = true
		}
		return !found
	})
	return found
}

func isParamPart(p syntax.WordPart, name string) bool {
	if dq, ok := p.(*syntax.DblQuoted); ok && len(dq.Parts) > 0 {
		p = dq.Parts[0]
	}
	pe, ok := p.(*syntax.ParamExp)
	return ok && pe.Param != nil && pe.Param.Value == name
}

// shellSupported reports whether a run step's shell is one this module reads:
// bash or sh. An empty shell is the executor default, which the compiler
// materializes when it is not bash (e.g. powershell.exe on Windows).
func shellSupported(shell string) bool {
	fields := strings.Fields(shell)
	if len(fields) == 0 {
		return true
	}
	bin, flags := fields[0], fields[1:]
	if bin == "/usr/bin/env" && len(fields) > 1 {
		bin, flags = fields[1], fields[2:]
	}
	known := slices.Contains(supportedShells, bin) ||
		slices.Contains([]string{"/bin", "/usr/bin"}, path.Dir(bin)) && slices.Contains(supportedShells, path.Base(bin))
	return known && slices.IndexFunc(flags, func(f string) bool { return !safeShellFlag(f) }) < 0
}

// safeShellFlag accepts the flags that only change error handling or tracing.
// Startup-file and mode flags (--rcfile, -i, -l, --login) run unseen code.
func safeShellFlag(f string) bool {
	switch f {
	case "pipefail", "errexit", "nounset", "xtrace", "--noprofile", "--norc", "-o", "+o":
		return true
	}
	if len(f) < len("-e") || (f[0] != '-' && f[0] != '+') {
		return false
	}
	return strings.Trim(f[1:], "euxo") == ""
}
