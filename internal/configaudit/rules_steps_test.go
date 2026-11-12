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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// auditMessages audits src and returns the messages of rule id's reported
// findings, in order.
func auditMessages(t *testing.T, src, id string) []string {
	t.Helper()
	doc, err := Parse([]byte(src), "c.yml")
	assert.NilError(t, err)
	got := []string{}
	for _, f := range Audit(doc, Options{}).Findings {
		if f.RuleID == id {
			got = append(got, f.Message)
		}
	}
	return got
}

func TestPipelineValueInjection(t *testing.T) {
	const id = "pipeline-value-injection"
	high := func(line, col int) row { return row{ID: id, Line: line, Col: col, Sev: SeverityHigh} }
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "string form",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo \"<< pipeline.git.branch >>\"\n",
			want: []row{high(4, 20)},
		},
		{
			name: "command key",
			yaml: "jobs:\n  b:\n    steps:\n      - run:\n          command: echo << pipeline.git.tag >>\n",
			want: []row{high(5, 25)},
		},
		{
			name: "third line of a literal block",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          set -e\n          echo start\n" +
				"          echo << pipeline.git.commit.subject >>\n",
			want: []row{high(7, 16)},
		},
		{
			name: "two values in one command",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.git.branch >> << pipeline.git.tag >>\n",
			want: []row{high(4, 19), high(4, 45)},
		},
		{
			name: "author prefix family",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.git.commit.author_email >>\n",
			want: []row{high(4, 19)},
		},
		{
			name: "pull request family",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.event.github.pull_request.title >>\n",
			want: []row{high(4, 19)},
		},
		{
			name: "no spaces inside the brackets",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo <<pipeline.git.branch>>\n",
			want: []row{high(4, 19)},
		},
		{
			name: "command in a when step",
			yaml: "jobs:\n  b:\n    steps:\n      - when:\n          condition: true\n          steps:\n" +
				"            - run: echo << pipeline.git.branch >>\n",
			want: []row{high(7, 25)},
		},
		{
			name: "command section",
			yaml: "commands:\n  c:\n    steps:\n      - run: echo << pipeline.git.branch >>\n",
			want: []row{high(4, 19)},
		},
		{
			name: "passed through environment and used as a shell variable",
			yaml: "jobs:\n  b:\n    steps:\n      - run:\n          command: echo \"$BRANCH\"\n" +
				"          environment:\n            BRANCH: << pipeline.git.branch >>\n",
			want: []row{},
		},
		{
			name: "job environment is not inspected",
			yaml: "jobs:\n  b:\n    environment:\n      BRANCH: << pipeline.git.branch >>\n    steps:\n      - run: echo \"$BRANCH\"\n",
			want: []row{},
		},
		{
			name: "revision is not injectable",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.git.revision >>\n",
			want: []row{},
		},
		{
			name: "pipeline parameters are not flagged",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.parameters.x >>\n",
			want: []row{},
		},
		{
			name: "pipeline number is not injectable",
			yaml: "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.number >>\n",
			want: []row{},
		},
		{
			name: "step other than run",
			yaml: "jobs:\n  b:\n    steps:\n      - save_cache:\n          key: << pipeline.git.branch >>\n          paths: [x]\n",
			want: []row{},
		},
		{
			name: "command parameter with a tainted default",
			yaml: "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n" +
				"        default: << pipeline.git.branch >>\n    steps:\n      - run: echo \"hello << parameters.who >>\"\n",
			want: []row{high(8, 26)},
		},
		{
			name: "job parameter with a tainted default",
			yaml: "jobs:\n  b:\n    parameters:\n      ref:\n        type: string\n" +
				"        default: << pipeline.git.tag >>\n    steps:\n      - run: git checkout << parameters.ref >>\n",
			want: []row{high(8, 27)},
		},
		{
			name: "parameter with a plain default",
			yaml: "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n" +
				"        default: world\n    steps:\n      - run: echo \"hello << parameters.who >>\"\n",
			want: []row{},
		},
		{
			name: "parameter with a revision default",
			yaml: "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n" +
				"        default: << pipeline.git.revision >>\n    steps:\n      - run: echo \"hello << parameters.who >>\"\n",
			want: []row{},
		},
		{
			name: "job parameter passed a tainted value by a workflow",
			yaml: "jobs:\n  b:\n    parameters:\n      ref:\n        type: string\n" +
				"    steps:\n      - run: git checkout << parameters.ref >>\n" +
				"workflows:\n  w:\n    jobs:\n      - b:\n          ref: << pipeline.git.branch >>\n",
			want: []row{high(7, 27)},
		},
		{
			name: "job parameter passed a plain value by a workflow",
			yaml: "jobs:\n  b:\n    parameters:\n      ref:\n        type: string\n" +
				"    steps:\n      - run: git checkout << parameters.ref >>\n" +
				"workflows:\n  w:\n    jobs:\n      - b:\n          ref: main\n",
			want: []row{},
		},
		{
			name: "command parameter passed a tainted value by a step",
			yaml: "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n" +
				"    steps:\n      - run: echo << parameters.who >>\n" +
				"jobs:\n  b:\n    steps:\n      - greet:\n          who: << pipeline.git.commit.subject >>\n",
			want: []row{high(7, 19)},
		},
		{
			name: "command parameter passed a tainted value from pre-steps",
			yaml: "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n" +
				"    steps:\n      - run: echo << parameters.who >>\n" +
				"jobs:\n  b:\n    steps: []\n" +
				"workflows:\n  w:\n    jobs:\n      - b:\n          pre-steps:\n            - greet:\n                who: << pipeline.git.branch >>\n",
			want: []row{high(7, 19)},
		},
		{
			name: "taint of one command does not reach another with the same parameter name",
			yaml: "commands:\n  a:\n    parameters:\n      who:\n        type: string\n" +
				"        default: << pipeline.git.branch >>\n    steps:\n      - run: echo << parameters.who >>\n" +
				"  b:\n    parameters:\n      who:\n        type: string\n    steps:\n      - run: echo << parameters.who >>\n",
			want: []row{high(8, 19)},
		},
		{
			name: "parameter of another job is not tainted by this workflow entry",
			yaml: "jobs:\n  a:\n    parameters:\n      ref:\n        type: string\n" +
				"    steps:\n      - run: echo << parameters.ref >>\n" +
				"  b:\n    parameters:\n      ref:\n        type: string\n    steps:\n      - run: echo << parameters.ref >>\n" +
				"workflows:\n  w:\n    jobs:\n      - b:\n          ref: << pipeline.git.branch >>\n",
			want: []row{high(13, 19)},
		},
		{
			name: "alias is reported once at the anchor",
			yaml: "jobs:\n  a:\n    steps:\n      - &inject\n        run: echo << pipeline.git.branch >>\n" +
				"  b:\n    steps:\n      - *inject\n",
			want: []row{high(5, 19)},
		},
		{
			name: "no jobs",
			yaml: "version: 2.1\n",
			want: []row{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestPipelineValueInjectionMessages(t *testing.T) {
	const id = "pipeline-value-injection"
	t.Run("direct", func(t *testing.T) {
		got := auditMessages(t, "jobs:\n  b:\n    steps:\n      - run: echo << pipeline.git.branch >>\n", id)
		assert.Check(t, cmp.DeepEqual(got, []string{"pipeline.git.branch is expanded into a shell command"}))
	})
	t.Run("default", func(t *testing.T) {
		got := auditMessages(t, "commands:\n  greet:\n    parameters:\n      who:\n        type: string\n"+
			"        default: << pipeline.git.branch >>\n    steps:\n      - run: echo << parameters.who >>\n", id)
		assert.Check(t, cmp.DeepEqual(got, []string{
			"parameter who defaults to pipeline.git.branch and is expanded into a shell command",
		}))
	})
	t.Run("passed", func(t *testing.T) {
		got := auditMessages(t, "jobs:\n  b:\n    steps:\n      - run: echo << parameters.who >>\n"+
			"workflows:\n  w:\n    jobs:\n      - b:\n          who: << pipeline.git.tag >>\n", id)
		assert.Check(t, cmp.DeepEqual(got, []string{
			"parameter who is passed pipeline.git.tag and is expanded into a shell command",
		}))
	})
}

func TestPipelineValueInjectionFix(t *testing.T) {
	doc, err := Parse([]byte("jobs:\n  b:\n    steps:\n      - run: echo << pipeline.git.branch >>\n"), "c.yml")
	assert.NilError(t, err)
	report := Audit(doc, Options{})
	assert.Assert(t, cmp.Len(report.Findings, 1))
	assert.Check(t, cmp.Equal(report.Findings[0].Fix,
		`pass it through environment: (BRANCH: << pipeline.git.branch >>) and use "$BRANCH"`))
}

func TestHardcodedSecret(t *testing.T) {
	const id = "hardcoded-secret"
	high := func(line, col int) row { return row{ID: id, Line: line, Col: col, Sev: SeverityHigh} }
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "job environment token with a known prefix",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: ghp_abcdefghijklmnopqrstuvwxyz0123456789\n",
			want: []row{high(4, 21)},
		},
		{
			name: "quoted value",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: \"ghp_abcdefghijklmnopqrstuvwxyz0123456789\"\n",
			want: []row{high(4, 22)},
		},
		{
			name: "key matches without a known prefix",
			yaml: "jobs:\n  b:\n    environment:\n      DB_PASSWORD: s3cr3tvalue\n",
			want: []row{high(4, 20)},
		},
		{
			name: "known prefix under a key that does not match",
			yaml: "jobs:\n  b:\n    environment:\n      DEPLOY: AKIAIOSFODNN7EXAMPLE\n",
			want: []row{high(4, 15)},
		},
		{
			name: "key ending in _KEY",
			yaml: "jobs:\n  b:\n    environment:\n      SIGNING_KEY: abcd\n",
			want: []row{high(4, 20)},
		},
		{
			name: "key containing _KEY but not ending in it",
			yaml: "jobs:\n  b:\n    environment:\n      KEY_NAME: abcd\n",
			want: []row{},
		},
		{
			name: "executor environment",
			yaml: "executors:\n  e:\n    environment:\n      API_KEY: abcd1234\n    docker:\n      - image: x@sha256:a\n",
			want: []row{high(4, 16)},
		},
		{
			name: "docker entry environment",
			yaml: "jobs:\n  b:\n    docker:\n      - image: x@sha256:a\n        environment:\n          SECRET_VALUE: abcd1234\n",
			want: []row{high(6, 25)},
		},
		{
			name: "step environment",
			yaml: "jobs:\n  b:\n    steps:\n      - run:\n          command: ./deploy\n          environment:\n" +
				"            DEPLOY_TOKEN: abcd1234\n",
			want: []row{high(7, 27)},
		},
		{
			name: "step environment in a command",
			yaml: "commands:\n  c:\n    steps:\n      - run:\n          command: ./deploy\n          environment:\n" +
				"            DEPLOY_TOKEN: abcd1234\n",
			want: []row{high(7, 27)},
		},
		{
			name: "docker auth password",
			yaml: "jobs:\n  b:\n    docker:\n      - image: x@sha256:a\n        auth:\n          username: u\n" +
				"          password: hunter2hunter2\n",
			want: []row{high(7, 21)},
		},
		{
			name: "docker auth password of an executor",
			yaml: "executors:\n  e:\n    docker:\n      - image: x@sha256:a\n        auth:\n          password: hunter2\n",
			want: []row{high(6, 21)},
		},
		{
			name: "short value",
			yaml: "jobs:\n  b:\n    environment:\n      PASSWORD: hunter2\n",
			want: []row{high(4, 17)},
		},
		{
			name: "shell variable",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: $OTHER_TOKEN\n",
			want: []row{},
		},
		{
			name: "braced shell variable",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: ${OTHER_TOKEN}\n",
			want: []row{},
		},
		{
			name: "parameter reference",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: << parameters.token >>\n",
			want: []row{},
		},
		{
			name: "empty value",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: \"\"\n",
			want: []row{},
		},
		{
			name: "null value",
			yaml: "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN:\n",
			want: []row{},
		},
		{
			name: "placeholders",
			yaml: "jobs:\n  b:\n    environment:\n      A_TOKEN: changeme\n      B_TOKEN: PLACEHOLDER\n      C_TOKEN: xxxx\n" +
				"      D_TOKEN: \"****\"\n      E_TOKEN: \"<your token>\"\n",
			want: []row{},
		},
		{
			name: "boolean value under a secret-looking key",
			yaml: "jobs:\n  b:\n    environment:\n      PASSWORD_REQUIRED: true\n      A_TOKEN: No\n      B_TOKEN: \"off\"\n      C_SECRET: YES\n",
			want: []row{},
		},
		{
			name: "numeric value under a secret-looking key",
			yaml: "jobs:\n  b:\n    environment:\n      SECRET_COUNT: 3\n      TOKEN_TTL: 3600\n      A_PASSWORD: 0\n      B_PASSWORD: \"30\"\n",
			want: []row{},
		},
		{
			name: "known prefix is flagged under any key",
			yaml: "jobs:\n  b:\n    environment:\n      TOKEN_TTL: ghp_abcdefghijklmnopqrstuvwxyz0123456789\n      FLAG: ghp_abcdefghijklmnopqrstuvwxyz0123456789\n",
			want: []row{high(4, 18), high(5, 13)},
		},
		{
			name: "key that does not look secret",
			yaml: "jobs:\n  b:\n    environment:\n      NODE_ENV: production\n",
			want: []row{},
		},
		{
			name: "environment shared through an anchor is reported once",
			yaml: "jobs:\n  a:\n    environment: &env\n      GITHUB_TOKEN: ghp_abcdefghijklmnopqrstuvwxyz0123456789\n" +
				"  b:\n    environment: *env\n",
			want: []row{high(4, 21)},
		},
		{
			name: "no jobs",
			yaml: "version: 2.1\n",
			want: []row{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestHardcodedSecretMessageAndConfidence(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		key            string
		wantMessage    string
		wantConfidence Confidence
	}{
		{
			name:           "known prefix",
			key:            "GITHUB_TOKEN",
			value:          "ghp_abcdefghijklmnopqrstuvwxyz0123456789",
			wantMessage:    "GITHUB_TOKEN has a literal value (ghp_…)",
			wantConfidence: ConfidenceHigh,
		},
		{
			name:           "key only",
			key:            "DB_PASSWORD",
			value:          "correcthorsebatterystaple",
			wantMessage:    "DB_PASSWORD has a literal value (corr…)",
			wantConfidence: ConfidenceMedium,
		},
		{
			name:           "short value is shown as an ellipsis",
			key:            "DB_PASSWORD",
			value:          "hunter2",
			wantMessage:    "DB_PASSWORD has a literal value (…)",
			wantConfidence: ConfidenceMedium,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "jobs:\n  b:\n    environment:\n      " + tc.key + ": " + tc.value + "\n"
			doc, err := Parse([]byte(src), "c.yml")
			assert.NilError(t, err)
			report := Audit(doc, Options{})
			assert.Assert(t, cmp.Len(report.Findings, 1))
			f := report.Findings[0]
			assert.Check(t, cmp.Equal(f.Message, tc.wantMessage))
			assert.Check(t, cmp.Equal(f.Confidence, tc.wantConfidence))
			assert.Check(t, !strings.Contains(f.Snippet, tc.value), "snippet %q shows the secret", f.Snippet)
			assert.Check(t, !strings.Contains(f.Message, tc.value), "message %q shows the secret", f.Message)
		})
	}

	t.Run("docker auth password message", func(t *testing.T) {
		got := auditMessages(t, "jobs:\n  b:\n    docker:\n      - image: x@sha256:a\n        auth:\n"+
			"          password: hunter2hunter2\n", "hardcoded-secret")
		assert.Check(t, cmp.DeepEqual(got, []string{"docker auth password has a literal value (hunt…)"}))
	})
}

func TestHardcodedSecretSnippet(t *testing.T) {
	t.Run("the value is replaced in place", func(t *testing.T) {
		src := "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: ghp_abcdefghijklmnopqrstuvwxyz0123456789\n"
		doc, err := Parse([]byte(src), "c.yml")
		assert.NilError(t, err)
		report := Audit(doc, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].Snippet, "      GITHUB_TOKEN: ghp_…"))
	})

	t.Run("a quoted value keeps its quotes", func(t *testing.T) {
		src := "jobs:\n  b:\n    environment:\n      GITHUB_TOKEN: \"ghp_abcdefghijklmnopqrstuvwxyz0123456789\"\n"
		doc, err := Parse([]byte(src), "c.yml")
		assert.NilError(t, err)
		report := Audit(doc, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].Snippet, `      GITHUB_TOKEN: "ghp_…"`))
	})

	t.Run("another finding on the same line does not show the value", func(t *testing.T) {
		src := "jobs:\n  b:\n    docker: [{image: \"cimg/base:latest\", environment: {GITHUB_TOKEN: ghp_abcdefghijklmnopqrstuvwxyz0123456789}}]\n"
		doc, err := Parse([]byte(src), "c.yml")
		assert.NilError(t, err)
		report := Audit(doc, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 2))
		for _, f := range report.Findings {
			assert.Check(t, !strings.Contains(f.Snippet, "ghp_abcdefghijkl"), "%s snippet %q shows the secret", f.RuleID, f.Snippet)
		}
	})
}

func TestSSHKeysUnrestricted(t *testing.T) {
	const id = "ssh-keys-unrestricted"
	med := func(line, col int) row { return row{ID: id, Line: line, Col: col, Sev: SeverityMedium} }
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "bare step",
			yaml: "jobs:\n  b:\n    steps:\n      - checkout\n      - add_ssh_keys\n",
			want: []row{med(5, 9)},
		},
		{
			name: "mapping without fingerprints",
			yaml: "jobs:\n  b:\n    steps:\n      - add_ssh_keys: {}\n",
			want: []row{med(4, 9)},
		},
		{
			name: "empty fingerprints",
			yaml: "jobs:\n  b:\n    steps:\n      - add_ssh_keys:\n          fingerprints: []\n",
			want: []row{med(4, 9)},
		},
		{
			name: "blank fingerprint",
			yaml: "jobs:\n  b:\n    steps:\n      - add_ssh_keys:\n          fingerprints:\n            - \"\"\n",
			want: []row{med(4, 9)},
		},
		{
			name: "fingerprints present",
			yaml: "jobs:\n  b:\n    steps:\n      - add_ssh_keys:\n          fingerprints:\n            - \"SHA256:abc\"\n",
			want: []row{},
		},
		{
			name: "fingerprint passed as a parameter",
			yaml: "commands:\n  c:\n    steps:\n      - add_ssh_keys:\n          fingerprints:\n            - << parameters.fp >>\n",
			want: []row{},
		},
		{
			name: "in a command and a when step",
			yaml: "commands:\n  c:\n    steps:\n      - when:\n          condition: true\n          steps:\n            - add_ssh_keys\n",
			want: []row{med(7, 15)},
		},
		{
			name: "no steps",
			yaml: "jobs:\n  b:\n    docker: []\n",
			want: []row{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestCurlPipeShell(t *testing.T) {
	const id = "curl-pipe-shell"
	med := func(line, col int) row { return row{ID: id, Line: line, Col: col, Sev: SeverityMedium} }
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "curl piped to sh",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -fsSL https://example.com/i.sh | sh\n",
			want: []row{med(4, 14)},
		},
		{
			name: "curl piped to bash",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -fsSL https://example.com/i.sh | bash -s -- --yes\n",
			want: []row{med(4, 14)},
		},
		{
			name: "wget piped to sudo sh",
			yaml: "jobs:\n  b:\n    steps:\n      - run: wget -qO- https://example.com/i.sh | sudo sh\n",
			want: []row{med(4, 14)},
		},
		{
			name: "process substitution",
			yaml: "jobs:\n  b:\n    steps:\n      - run: bash <(curl -s https://example.com/i.sh)\n",
			want: []row{med(4, 14)},
		},
		{
			name: "sh -c with command substitution",
			yaml: "jobs:\n  b:\n    steps:\n      - run: sh -c \"$(curl -fsSL https://example.com/i.sh)\"\n",
			want: []row{med(4, 14)},
		},
		{
			name: "command key",
			yaml: "jobs:\n  b:\n    steps:\n      - run:\n          command: curl https://example.com/i.sh | zsh\n",
			want: []row{med(5, 20)},
		},
		{
			name: "continuation before the pipe",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          curl -fsSL https://example.com/i.sh \\\n            | sh\n",
			want: []row{med(5, 11)},
		},
		{
			name: "continuation inside the curl command",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          curl -fsSL \\\n            https://example.com/i.sh | sh\n",
			want: []row{med(5, 11)},
		},
		{
			name: "third line of a literal block",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          set -e\n          echo start\n" +
				"          curl -fsSL https://example.com/i.sh | sh\n",
			want: []row{med(7, 11)},
		},
		{
			name: "second occurrence in a block",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          curl https://a.example/i.sh | sh\n" +
				"          wget -O- https://b.example/i.sh | bash\n",
			want: []row{med(5, 11), med(6, 11)},
		},
		{
			name: "command substitution around a piped curl is reported once",
			yaml: "jobs:\n  b:\n    steps:\n      - run: bash -c \"$(curl https://example.com/i.sh | sh)\"\n",
			want: []row{med(4, 14)},
		},
		{
			name: "download to a file",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -o i.sh https://example.com/i.sh\n",
			want: []row{},
		},
		{
			name: "download then run",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          curl -o i.sh https://example.com/i.sh\n          sh i.sh\n",
			want: []row{},
		},
		{
			name: "piped to something else",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -s https://example.com/v.json | jq .version\n",
			want: []row{},
		},
		{
			name: "piped to a checksum tool",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -s https://example.com/i.sh | shasum -a 256\n",
			want: []row{},
		},
		{
			name: "or operator",
			yaml: "jobs:\n  b:\n    steps:\n      - run: curl -s https://example.com/i.sh || sh fallback.sh\n",
			want: []row{},
		},
		{
			name: "pipe on another command's line",
			yaml: "jobs:\n  b:\n    steps:\n      - run: |\n          curl -o i.sh https://example.com/i.sh\n          echo ok | sh\n",
			want: []row{},
		},
		{
			name: "step other than run",
			yaml: "jobs:\n  b:\n    steps:\n      - save_cache:\n          key: curl | sh\n          paths: [x]\n",
			want: []row{},
		},
		{
			name: "no jobs",
			yaml: "version: 2.1\n",
			want: []row{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestJoinContinuations(t *testing.T) {
	joined, orig := joinContinuations("a \\\nb\nc")
	assert.Check(t, cmp.Equal(joined, "a  b\nc"))
	assert.Check(t, cmp.DeepEqual(orig, []int{0, 1, 2, 4, 5, 6, 7}))
}
