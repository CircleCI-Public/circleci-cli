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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// outcome is what a script should classify as. An empty code with build
// false means clean.
type outcome struct {
	build bool
	code  Code
}

var (
	isClean = outcome{}
	isBuild = outcome{build: true}
)

func opaqueAs(c Code) outcome { return outcome{code: c} }

func checkScript(t *testing.T, script string, want outcome) {
	t.Helper()
	got := scanScript(script)
	switch {
	case want.build:
		assert.Check(t, len(got.builds) > 0, "%q: no build found; opaque: %v", script, got.opaque)
	case want.code == "":
		assert.Check(t, cmp.Len(got.builds, 0), "%q: builds %v", script, got.builds)
		assert.Check(t, cmp.Len(got.opaque, 0), "%q: opaque %v", script, got.opaque)
	default:
		assert.Check(t, cmp.Len(got.builds, 0), "%q: builds %v", script, got.builds)
		codes := make([]Code, 0, len(got.opaque))
		for _, o := range got.opaque {
			codes = append(codes, o.code)
		}
		assert.Check(t, cmp.Contains(codes, want.code), "%q: want %s, got %v", script, want.code, got.opaque)
	}
}

func TestScanScript(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   outcome
	}{
		// Text that only mentions a build.
		{"echoed text", `echo "run docker build to start"`, isClean},
		{"comment", "# docker build .\nx=1", isClean},
		{"heredoc body", "cat <<EOF\ndocker build .\nEOF", isClean},
		{"grep for it", `grep -r "docker build" .`, isClean},
		{"docker help", "docker build --help", isClean},
		{"docker version", "docker --version", isClean},
		{"bash version", "bash --version", isClean},
		{"pull and login", `echo "$PASS" | docker login -u me --password-stdin && docker pull x`, isClean},
		{"ignore-scripts install", "npm ci --ignore-scripts", isClean},
		{"go build", "go build ./... && go vet ./...", isClean},
		{"PATH appended", `export PATH="$PATH:$HOME/go/bin"`, isClean},
		{"BASH_ENV literal", `echo 'export GOFLAGS=-mod=mod' >> "$BASH_ENV"`, isClean},
		{"local function", "greet() { echo hi; }\ngreet", isClean},

		// Builds.
		{"docker build", "docker build -t x .", isBuild},
		{"buildx build", "docker buildx build --platform linux/amd64,linux/arm64 .", isBuild},
		{"buildx bake", "docker buildx bake", isBuild},
		{"image build", "docker image build .", isBuild},
		{"compose build", "docker compose -f ci.yml build", isBuild},
		{"legacy compose build", "docker-compose build web", isBuild},
		{"compose up --build", "docker compose up -d --build", isBuild},
		{"global option", "docker --context ci build .", isBuild},
		{"chained", "docker pull x && docker build .", isBuild},
		{"sudo long option", "sudo --user root docker build .", isBuild},
		{"stdbuf valued", "stdbuf -o L docker build .", isBuild},
		{"stdbuf attached", "stdbuf -oL docker build .", isBuild},
		{"env prefix", "env DOCKER_BUILDKIT=1 docker build .", isBuild},
		{"assignment prefix", "DOCKER_BUILDKIT=1 docker build .", isBuild},
		{"timeout", "timeout 10m docker build .", isBuild},
		{"command substitution", `ID=$(docker build -q .)`, isBuild},
		{"inside a function", "b() { docker build .; }\nb", isBuild},
		{"bash -c", `bash -c "docker build ."`, isBuild},
		{"trap handler", "trap 'docker build .' EXIT\necho done", isBuild},
		{"xargs docker", "echo . | xargs docker build", isBuild},
		{"BASH_ENV carries a build", `echo 'docker build .' >> $BASH_ENV`, isBuild},

		// Uninspectable, with the reason code the report ranks by.
		{"script file", "./scripts/ci.sh", opaqueAs(CodeExternalScript)},
		{"absolute docker path", "/usr/bin/docker build .", opaqueAs(CodeExternalScript)},
		{"git commit hooks", "git commit -m x", opaqueAs(CodeExternalScript)},
		{"go tool", "go tool task ci:test", opaqueAs(CodeTaskRunner)},
		{"npm ci", "npm ci", opaqueAs(CodePackageLifecycle)},
		{"python -m pip", "python -m pip install .", opaqueAs(CodePackageLifecycle)},
		{"bare yarn", "yarn --frozen-lockfile", opaqueAs(CodePackageLifecycle)},
		{"cargo build", "cargo build --release", opaqueAs(CodePackageLifecycle)},
		{"go test", "go test ./...", opaqueAs(CodeTestRunner)},
		{"npm test", "npm test", opaqueAs(CodeTestRunner)},
		{"npm run", "npm run build:image", opaqueAs(CodePackageScript)},
		{"yarn script", "yarn build", opaqueAs(CodePackageScript)},
		{"docker run with socket", "docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker build .", opaqueAs(CodeContainerRun)},
		{"compose up", "docker compose up -d", opaqueAs(CodeComposeUp)},
		{"legacy compose up", "docker-compose up -d", opaqueAs(CodeComposeUp)},
		{"dynamic command", `$BUILD_CMD .`, opaqueAs(CodeDynamicCommand)},
		{"dynamic docker subcommand", `docker "$SUB" .`, opaqueAs(CodeDynamicCommand)},
		{"eval of a variable", `eval "$CMD"`, opaqueAs(CodeDynamicCommand)},
		{"awk system", `awk 'BEGIN { system("docker build .") }'`, opaqueAs(CodeDynamicCommand)},
		{"sed e command", `sed -e 'e docker build .' x`, opaqueAs(CodeDynamicCommand)},
		{"find exec", "find . -exec sh {} ;", opaqueAs(CodeDynamicCommand)},
		{"tar to-command", "tar --to-command=sh -xf a.tar", opaqueAs(CodeDynamicCommand)},
		{"git -c", "git -c alias.b='!docker build .' b", opaqueAs(CodeDynamicCommand)},
		{"unknown sudo option", "sudo --weird docker build .", opaqueAs(CodeDynamicCommand)},
		{"unknown docker option", "docker --weird x build .", opaqueAs(CodeDynamicCommand)},
		{"alias", "alias ls='docker build .'\nls", opaqueAs(CodeShellState)},
		{"function shadows docker", "docker() { echo no; }\ndocker build .", opaqueAs(CodeShellState)},
		{"PATH prepended", "export PATH=/opt/bin:$PATH", opaqueAs(CodeShellState)},
		{"BASH_ENV computed", `cat env.txt >> $BASH_ENV`, opaqueAs(CodeShellState)},
		{"unknown tool", "dockerize -wait tcp://localhost:5432", opaqueAs(CodeUnknownCommand)},
		{"git alias", "git mybuild", opaqueAs(CodeUnknownCommand)},
		{"unparseable", "if then fi (", opaqueAs(CodeUnparseable)},

		// Regression cases from code review.
		{"xargs supplies the subcommand", "printf 'build .\n' | xargs docker", opaqueAs(CodeDynamicCommand)},
		{"computed find option", "op=-exec\nfind . \"$op\" sh -c 'docker build .' ';'", opaqueAs(CodeDynamicCommand)},
		{"conditionally declared function", "if false; then\n  f() { :; }\nfi\nf", opaqueAs(CodeUnknownCommand)},
		{"git config installs hooks", "git config core.hooksPath .hooks", opaqueAs(CodeShellState)},
		{"git checkout runs hooks", "git checkout -b t", opaqueAs(CodeExternalScript)},
		{"git submodule foreach", "git submodule foreach 'docker build .'", opaqueAs(CodeExternalScript)},
		{"git attached -c", "git -ccore.hooksPath=.h status", opaqueAs(CodeDynamicCommand)},
		{"git unknown global option", "git --exec-path=/x status", opaqueAs(CodeDynamicCommand)},
		{"go -toolexec", "go build -toolexec='/bin/sh -c x' ./...", opaqueAs(CodeExternalScript)},
		{"go vet -vettool", "go vet -vettool=./tool ./...", opaqueAs(CodeExternalScript)},
		{"go -C then test", "go -C build test ./...", opaqueAs(CodeTestRunner)},
		{"sed attached -e", "printf x | sed -e'e docker build .'", opaqueAs(CodeDynamicCommand)},
		{"sed clustered -ne", "sed -ne 'e docker build .' f", opaqueAs(CodeDynamicCommand)},
		{"sed attached -f", "sed -fscript.sed input", opaqueAs(CodeExternalScript)},
		{"awk system with a space", `awk 'BEGIN { system ("docker build .") }'`, opaqueAs(CodeDynamicCommand)},
		{"awk unknown option", "awk -E prog.awk", opaqueAs(CodeExternalScript)},
		{"tar attached -I", `tar -I'sh -c x' -cf o.tar .`, opaqueAs(CodeDynamicCommand)},
		{"tar traditional cluster", `tar cIf zstd o.tar .`, opaqueAs(CodeDynamicCommand)},
		{"sudo -s reads stdin", "printf 'docker build .\n' | sudo -s", opaqueAs(CodeExternalScript)},
		{"npm last ignore-scripts wins", "npm ci --ignore-scripts --ignore-scripts=false", opaqueAs(CodePackageLifecycle)},
		{"npm option before subcommand", "npm --prefix config run build-image", opaqueAs(CodeDynamicCommand)},
		{"cp into BASH_ENV", `cp /tmp/environment "$BASH_ENV"`, opaqueAs(CodeShellState)},
		{"tee into BASH_ENV", `printf 'docker build .\n' | tee -a "$BASH_ENV"`, opaqueAs(CodeShellState)},
		{"curl the Docker socket", "curl --unix-socket /var/run/docker.sock -X POST http://localhost/build", opaqueAs(CodeContainerRun)},
		{"docker login with variables", `echo "$PASS" | docker login -u "$USER" --password-stdin`, isClean},
		{"docker commit is not a build", "docker commit c img", isClean},
		{"python reads stdin", "echo 'import os' | python3", opaqueAs(CodeExternalScript)},
		{"python heredoc", "python3 <<'PY'\nimport subprocess\nPY", opaqueAs(CodeExternalScript)},
		{"BASH_ENV assignment prefix", "BASH_ENV=./ci/env bash -c ':'", opaqueAs(CodeShellState)},
		{"GOFLAGS assignment prefix", "GOFLAGS=-toolexec=./w go build ./...", opaqueAs(CodeShellState)},
		{"exported LD_PRELOAD", "export LD_PRELOAD=./x.so", opaqueAs(CodeShellState)},
		{"env sets GIT_", "env GIT_SSH_COMMAND=./x git log", opaqueAs(CodeShellState)},
		{"harmless assignment", "export NODE_ENV=production GOPROXY=direct", isClean},
		{"curl -K attached", "curl -K./ci/build.curlrc", opaqueAs(CodeExternalScript)},
		{"curl referer is fine", "curl -e https://x -o out https://example.com/f", isClean},
		{"wget execute", "wget -e 'use_proxy=on' https://x", opaqueAs(CodeExternalScript)},
		{"wget clustered -e", "wget -qe'use_askpass = x' --ask-password https://x/", opaqueAs(CodeExternalScript)},
		{"curl clustered -K", "curl -sKfile", opaqueAs(CodeExternalScript)},
		{"PS4 assignment", "export PS4='$(docker build .)'", opaqueAs(CodeShellState)},
		{"bash -xc is read", "bash -xc 'docker build .'", isBuild},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { checkScript(t, tc.script, tc.want) })
	}
}

// specialCases gives every command in specialCommands at least one case.
// TestAllowlist fails when a special command has none, so the table cannot
// grow without a test.
var specialCases = map[string][]struct {
	script string
	want   outcome
}{
	"docker":         {{"docker ps", isClean}, {"docker exec c sh", opaqueAs(CodeContainerRun)}},
	"docker-compose": {{"docker-compose down", isClean}},
	"podman":         {{"podman build .", isClean}, {"podman run x", opaqueAs(CodeContainerRun)}},
	"nerdctl":        {{"nerdctl build .", isClean}, {"nerdctl run x", opaqueAs(CodeContainerRun)}},
	"buildah":        {{"buildah bud .", isClean}, {"buildah run c sh", opaqueAs(CodeContainerRun)}},
	"pack":           {{"pack build app", isBuild}, {"pack config", isClean}},
	"npm":            {{"npm ls", isClean}, {"npm install", opaqueAs(CodePackageLifecycle)}, {"npm audit fix", opaqueAs(CodePackageLifecycle)}},
	"yarn":           {{"yarn cache dir", isClean}, {"yarn install", opaqueAs(CodePackageLifecycle)}, {"yarn test", opaqueAs(CodeTestRunner)}},
	"pnpm":           {{"pnpm store path", isClean}, {"pnpm install", opaqueAs(CodePackageLifecycle)}, {"pnpm build", opaqueAs(CodePackageScript)}},
	"bun":            {{"bun --version", isClean}, {"bun install", opaqueAs(CodePackageLifecycle)}, {"bun x.ts", opaqueAs(CodePackageScript)}},
	"pip":            {{"pip list", isClean}, {"pip install x", opaqueAs(CodePackageLifecycle)}},
	"pip3":           {{"pip3 freeze", isClean}, {"pip3 install x", opaqueAs(CodePackageLifecycle)}},
	"uv":             {{"uv version", isClean}, {"uv sync", opaqueAs(CodePackageLifecycle)}, {"uv run x", opaqueAs(CodeExternalScript)}},
	"poetry":         {{"poetry check", isClean}, {"poetry install", opaqueAs(CodePackageLifecycle)}},
	"pipenv":         {{"pipenv graph", isClean}, {"pipenv install", opaqueAs(CodePackageLifecycle)}},
	"bundle":         {{"bundle check", isClean}, {"bundle install", opaqueAs(CodePackageLifecycle)}, {"bundle exec rspec", opaqueAs(CodeExternalScript)}},
	"gem":            {{"gem list", isClean}, {"gem install x", opaqueAs(CodePackageLifecycle)}},
	"composer":       {{"composer validate", isClean}, {"composer install", opaqueAs(CodePackageLifecycle)}},
	"cargo":          {{"cargo fmt", isClean}, {"cargo test", opaqueAs(CodeTestRunner)}},
	"go":             {{"go mod download", isClean}, {"go generate ./...", opaqueAs(CodeExternalScript)}},
	"git":            {{"git log --oneline", isClean}, {"git push", opaqueAs(CodeExternalScript)}, {"git config --get user.name", isClean}},
	"tar":            {{"tar -xzf a.tgz", isClean}, {"tar -I zstd -xf a.tar", opaqueAs(CodeDynamicCommand)}},
	"sed":            {{"sed -i 's/a/b/g' f", isClean}, {"sed -f s.sed f", opaqueAs(CodeExternalScript)}},
	"awk":            {{"awk '{print $1}' f", isClean}, {"awk -f p.awk f", opaqueAs(CodeExternalScript)}},
	"gawk":           {{"gawk '{print $1}' f", isClean}},
	"mawk":           {{"mawk '{print $1}' f", isClean}},
	"nawk":           {{"nawk '{print $1}' f", isClean}},
	"find":           {{"find . -type f", isClean}, {"find . -execdir sh {} ;", opaqueAs(CodeDynamicCommand)}},
	"xargs":          {{"xargs -n1 echo", isClean}, {"xargs make", opaqueAs(CodeTaskRunner)}},
	"parallel":       {{"parallel echo ::: a", opaqueAs(CodeDynamicCommand)}},
	"trap":           {{"trap - EXIT", isClean}, {"trap 'rm -f x' EXIT", isClean}},
	"eval":           {{"eval 'echo hi'", isClean}, {"eval 'docker build .'", isBuild}},
	"source":         {{"source env.sh", opaqueAs(CodeExternalScript)}},
	".":              {{". env.sh", opaqueAs(CodeExternalScript)}},
	"sudo":           {{"sudo -E apt-get update", opaqueAs(CodeUnknownCommand)}, {"sudo chmod 755 x", isClean}},
	"env":            {{"env A=1 echo hi", isClean}, {"env PATH=/x docker build .", opaqueAs(CodeShellState)}},
	"nice":           {{"nice -n 10 echo hi", isClean}},
	"timeout":        {{"timeout 5 sleep 1", isClean}},
	"time":           {{"time echo hi", isClean}},
	"nohup":          {{"nohup sleep 1", isClean}},
	"exec":           {{"exec echo hi", isClean}},
	"command":        {{"command -v docker", isClean}, {"command docker build .", isBuild}},
	"builtin":        {{"builtin echo hi", isClean}},
	"stdbuf":         {{"stdbuf -oL echo hi", isClean}},
	"ionice":         {{"ionice -c 3 echo hi", isClean}},
	"circleci":       {{"circleci tests glob '**/*.go'", isClean}, {"circleci tests run --command x", opaqueAs(CodeUnknownCommand)}},
	"circleci-agent": {{"circleci-agent step halt", isClean}},
	"curl":           {{"curl -fsSL https://x -o f", isClean}, {"curl --config f", opaqueAs(CodeExternalScript)}},
	"wget":           {{"wget -q https://x", isClean}, {"wget --execute x https://y", opaqueAs(CodeExternalScript)}},
}

func TestAllowlist(t *testing.T) {
	t.Run("clean commands", func(t *testing.T) {
		for _, c := range cleanCommands {
			checkScript(t, c, isClean)
		}
	})
	t.Run("shell state commands", func(t *testing.T) {
		for _, c := range shellStateCommands {
			checkScript(t, c+" x", opaqueAs(CodeShellState))
		}
	})
	t.Run("task runners", func(t *testing.T) {
		for _, c := range taskRunners {
			checkScript(t, c+" all", opaqueAs(CodeTaskRunner))
			checkScript(t, c+" --version", isClean)
		}
	})
	t.Run("test runners", func(t *testing.T) {
		for _, c := range testRunners {
			checkScript(t, c+" ./...", opaqueAs(CodeTestRunner))
		}
	})
	t.Run("interpreters", func(t *testing.T) {
		for _, c := range interpreters {
			checkScript(t, c+" main.x", opaqueAs(CodeExternalScript))
			checkScript(t, c+" --version", isClean)
		}
	})
	t.Run("shells", func(t *testing.T) {
		for _, c := range shells {
			checkScript(t, c+" run.sh", opaqueAs(CodeExternalScript))
			checkScript(t, c+" -c 'echo hi'", isClean)
		}
	})
	t.Run("package runners", func(t *testing.T) {
		for _, c := range packageRunners {
			checkScript(t, c+" tool", opaqueAs(CodePackageScript))
		}
	})
	t.Run("special commands", func(t *testing.T) {
		for _, c := range specialCommands {
			cases, ok := specialCases[c]
			assert.Check(t, ok, "special command %q has no test case", c)
			assert.Check(t, len(cases) > 0, "special command %q has no test case", c)
			for _, tc := range cases {
				checkScript(t, tc.script, tc.want)
			}
		}
	})
}

func TestShellSupported(t *testing.T) {
	tests := []struct {
		name  string // the shell
		shell string
		want  bool
		why   string
	}{
		{name: "the default shell", shell: "", want: true},
		{name: "bash with options", shell: "/bin/bash -eo pipefail", want: true},
		{name: "sh", shell: "sh", want: true},
		{name: "zsh", shell: "zsh", want: false},
		{name: "powershell", shell: "powershell.exe -ExecutionPolicy Bypass", want: false},
		{name: "bash outside a system directory", shell: "/tmp/bash", want: false, why: "a bash-named binary elsewhere may be anything"},
		{name: "bash through env", shell: "/usr/bin/env bash", want: true},
		{name: "sh with an option", shell: "/usr/bin/sh -e", want: true},
		{name: "bash without startup files", shell: "/bin/bash --noprofile --norc -e -o pipefail", want: true},
		{name: "bash with an rcfile", shell: "/bin/bash --rcfile ./ci/rc -i", want: false, why: "startup files run unseen code"},
		{name: "a login shell", shell: "/bin/bash -l", want: false, why: "a login shell reads profile files"},
		{name: "an interactive bash through env", shell: "/usr/bin/env bash -i", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(shellSupported(tc.shell), tc.want), tc.why)
		})
	}
}

func TestSedRunsCommands(t *testing.T) {
	assert.Check(t, !sedRunsCommands("s/a/b/g"))
	assert.Check(t, !sedRunsCommands("s|/usr|/opt|"))
	assert.Check(t, !sedRunsCommands("/^#/d"))
	assert.Check(t, sedRunsCommands("e date"))
	assert.Check(t, sedRunsCommands("1e date"))
	assert.Check(t, sedRunsCommands("s/x/date/e"))
	assert.Check(t, sedRunsCommands("/re/e date"))
	assert.Check(t, sedRunsCommands("s/unterminated"), "an s command it cannot split counts as running commands")
	assert.Check(t, sedRunsCommands(`/a\/b/e date`), "an escaped slash does not end the address")
	assert.Check(t, sedRunsCommands(`\,x,e date`), "custom address delimiters are refused")
	assert.Check(t, !sedRunsCommands(`/a\/b/d`))
}
