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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// auditRowsFile is auditRows for a config labelled file.
func auditRowsFile(t *testing.T, src, file, id string) []row {
	t.Helper()
	doc, err := Parse([]byte(src), file)
	assert.NilError(t, err)
	got := []row{}
	for _, f := range Audit(doc, Options{}).Findings {
		if f.RuleID == id {
			got = append(got, row{ID: f.RuleID, Line: f.Line, Col: f.Column, Sev: f.Severity})
		}
	}
	return got
}

func TestContextOnAnyBranch(t *testing.T) {
	const id = "context-on-any-branch"
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "no filter",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n",
			want: []row{{ID: id, Line: 5, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "list form gives one finding per name",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: [a, b]\n",
			want: []row{
				{ID: id, Line: 5, Col: 21, Sev: SeverityMedium},
				{ID: id, Line: 5, Col: 24, Sev: SeverityMedium},
			},
		},
		{
			name: "list form in block style",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context:\n            - a\n            - b\n",
			want: []row{
				{ID: id, Line: 6, Col: 15, Sev: SeverityMedium},
				{ID: id, Line: 7, Col: 15, Sev: SeverityMedium},
			},
		},
		{
			name: "branches.only",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n          filters:\n            branches:\n              only: main\n",
			want: []row{},
		},
		{
			name: "branches.only list",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n          filters:\n            branches:\n              only: [main, release]\n",
			want: []row{},
		},
		{
			name: "branches.ignore is not a restriction",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n          filters:\n            branches:\n              ignore: wip\n",
			want: []row{{ID: id, Line: 5, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "tag-only filter",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n          filters:\n            branches:\n              ignore: /.*/\n            tags:\n              only: /^v.*/\n",
			want: []row{},
		},
		{
			name: "upstream approval job",
			yaml: "workflows:\n  main:\n    jobs:\n      - hold:\n          type: approval\n      - build:\n          requires: [hold]\n          context: deploy\n",
			want: []row{},
		},
		{
			name: "approval job two requires away",
			yaml: "workflows:\n  main:\n    jobs:\n      - hold:\n          type: approval\n      - test:\n          requires: [hold]\n      - build:\n          requires: [test]\n          context: deploy\n",
			want: []row{},
		},
		{
			name: "approval job known by its name key",
			yaml: "workflows:\n  main:\n    jobs:\n      - gate:\n          name: hold\n          type: approval\n      - build:\n          requires:\n            - hold\n          context: deploy\n",
			want: []row{},
		},
		{
			name: "requires a job that is not an approval",
			yaml: "workflows:\n  main:\n    jobs:\n      - test\n      - build:\n          requires: [test]\n          context: deploy\n",
			want: []row{{ID: id, Line: 7, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "approval in another workflow does not count",
			yaml: "workflows:\n  a:\n    jobs:\n      - hold:\n          type: approval\n  b:\n    jobs:\n      - build:\n          requires: [hold]\n          context: deploy\n",
			want: []row{{ID: id, Line: 10, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "requires cycle ends",
			yaml: "workflows:\n  main:\n    jobs:\n      - a:\n          requires: [b]\n          context: deploy\n      - b:\n          requires: [a]\n",
			want: []row{{ID: id, Line: 6, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "workflow when",
			yaml: "workflows:\n  main:\n    when: << pipeline.parameters.deploy >>\n    jobs:\n      - build:\n          context: deploy\n",
			want: []row{},
		},
		{
			name: "workflow unless",
			yaml: "workflows:\n  main:\n    unless: << pipeline.parameters.skip >>\n    jobs:\n      - build:\n          context: deploy\n",
			want: []row{},
		},
		{
			name: "no context",
			yaml: "workflows:\n  main:\n    jobs:\n      - build\n      - test:\n          requires: [build]\n",
			want: []row{},
		},
		{
			name: "parameterised context is still flagged",
			yaml: "workflows:\n  main:\n    jobs:\n      - build:\n          context: << pipeline.parameters.ctx >>\n",
			want: []row{{ID: id, Line: 5, Col: 20, Sev: SeverityMedium}},
		},
		{
			name: "workflows version key is not a workflow",
			yaml: "workflows:\n  version: 2\n  main:\n    jobs:\n      - build:\n          context: deploy\n",
			want: []row{{ID: id, Line: 6, Col: 20, Sev: SeverityMedium}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestContextOnAnyBranchMessage(t *testing.T) {
	got := auditMessages(t, "workflows:\n  main:\n    jobs:\n      - build:\n          context: deploy\n", "context-on-any-branch")
	assert.Check(t, cmp.DeepEqual(got, []string{"job build gets context deploy on every branch"}))
}

func TestCachePoisoning(t *testing.T) {
	const id = "cache-poisoning"
	// cacheYAML is a job that restores and saves the same key family, run by
	// the given workflow entry.
	cacheYAML := func(restoreKey, saveKey, entry string) string {
		return "jobs:\n  build:\n    steps:\n" +
			"      - restore_cache:\n          key: " + restoreKey + "\n" +
			"      - save_cache:\n          key: " + saveKey + "\n          paths: [x]\n" +
			"workflows:\n  main:\n    jobs:\n" + entry
	}
	const (
		sum          = `deps-{{ checksum "a" }}`
		withContext  = "      - build:\n          context: ctx\n          filters:\n            branches:\n              only: feature\n"
		bare         = "      - build\n"
		prOnly       = "      - build:\n          filters:\n            branches:\n              only: feature-x\n"
		onMain       = "      - build:\n          filters:\n            branches:\n              only: main\n"
		onRegexMain  = "      - build:\n          filters:\n            branches:\n              only: /ma.*/\n"
		onRegexOther = "      - build:\n          filters:\n            branches:\n              only: /feature-.*/\n"
	)
	tests := []struct {
		name string
		yaml string
		want []row
	}{
		{
			name: "restored by a job with a context",
			yaml: cacheYAML(sum, sum, withContext),
			want: []row{{ID: id, Line: 7, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "restored by a job with no filters",
			yaml: cacheYAML(sum, sum, bare),
			want: []row{{ID: id, Line: 7, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "restored by a job that runs on main",
			yaml: cacheYAML(sum, sum, onMain),
			want: []row{{ID: id, Line: 7, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "restored by a job whose only is a regex matching main",
			yaml: cacheYAML(sum, sum, onRegexMain),
			want: []row{{ID: id, Line: 7, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "key has .Branch",
			yaml: cacheYAML("deps-{{ .Branch }}-{{ checksum \"a\" }}", "deps-{{ .Branch }}-{{ checksum \"a\" }}", bare),
			want: []row{},
		},
		{
			name: "restored only by a pull-request job with no context",
			yaml: cacheYAML(sum, sum, prOnly),
			want: []row{},
		},
		{
			name: "restored only by a job whose only regex cannot match main",
			yaml: cacheYAML(sum, sum, onRegexOther),
			want: []row{},
		},
		{
			name: "restored family differs",
			yaml: cacheYAML("other-{{ checksum \"a\" }}", sum, bare),
			want: []row{},
		},
		{
			name: "job not in any workflow",
			yaml: cacheYAML(sum, sum, "      - other\n"),
			want: []row{},
		},
		{
			name: "restored by a fallback key of the same family",
			yaml: "jobs:\n  build:\n    steps:\n      - restore_cache:\n          keys:\n            - other-{{ checksum \"a\" }}\n            - deps-\n" +
				"      - save_cache:\n          key: " + sum + "\n          paths: [x]\n" +
				"workflows:\n  main:\n    jobs:\n" + bare,
			want: []row{{ID: id, Line: 9, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "restored by a command the job runs",
			yaml: "commands:\n  restore:\n    steps:\n      - restore_cache:\n          key: " + sum + "\n" +
				"jobs:\n  build:\n    steps:\n      - restore\n      - save_cache:\n          key: " + sum + "\n          paths: [x]\n" +
				"workflows:\n  main:\n    jobs:\n" + bare,
			want: []row{{ID: id, Line: 11, Col: 16, Sev: SeverityMedium}},
		},
		{
			name: "save_cache with no restore",
			yaml: "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: " + sum + "\n          paths: [x]\nworkflows:\n  main:\n    jobs:\n" + bare,
			want: []row{},
		},
		{
			name: "epoch key gets a note",
			yaml: "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: deps-{{ epoch }}\n          paths: [x]\n",
			want: []row{{ID: id, Line: 5, Col: 21, Sev: SeverityLow}},
		},
		{
			name: "arch-only key gets a note",
			yaml: "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: v1-{{ arch }}\n          paths: [x]\n",
			want: []row{{ID: id, Line: 5, Col: 19, Sev: SeverityLow}},
		},
		{
			name: "arch with a checksum is not a time window",
			yaml: "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: v1-{{ arch }}-{{ checksum \"a\" }}\n          paths: [x]\n",
			want: []row{},
		},
		{
			name: "epoch key with .Branch has no note",
			yaml: "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: deps-{{ .Branch }}-{{ epoch }}\n          paths: [x]\n",
			want: []row{},
		},
		{
			name: "epoch key restored by an exposed job gets both findings",
			yaml: cacheYAML("deps-{{ epoch }}", "deps-{{ epoch }}", bare),
			want: []row{
				{ID: id, Line: 7, Col: 16, Sev: SeverityMedium},
				{ID: id, Line: 7, Col: 21, Sev: SeverityLow},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := auditRows(t, tc.yaml, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

func TestCachePoisoningMessages(t *testing.T) {
	t.Run("restored by a job with a context", func(t *testing.T) {
		src := "jobs:\n  build:\n    steps:\n      - restore_cache:\n          key: deps-{{ checksum \"a\" }}\n" +
			"      - save_cache:\n          key: deps-{{ checksum \"a\" }}\n          paths: [x]\n" +
			"workflows:\n  main:\n    jobs:\n      - build:\n          context: ctx\n"
		got := auditMessages(t, src, "cache-poisoning")
		assert.Check(t, cmp.DeepEqual(got, []string{
			`cache key deps-{{ checksum "a" }} has no {{ .Branch }} and is restored by job build, which gets a context`,
		}))
	})
	t.Run("time window note", func(t *testing.T) {
		src := "jobs:\n  build:\n    steps:\n      - save_cache:\n          key: deps-{{ epoch }}\n          paths: [x]\n"
		got := auditMessages(t, src, "cache-poisoning")
		assert.Check(t, cmp.DeepEqual(got, []string{"cache key deps-{{ epoch }} is built on a predictable time window"}))
	})
}

func TestDynamicConfigUntrusted(t *testing.T) {
	const id = "dynamic-config-untrusted"
	// setupYAML is a setup config whose job runs script and then continues
	// with next.yml.
	setupYAML := func(script string) string {
		return "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - run: " + script + "\n" +
			"      - continuation/continue:\n          configuration_path: next.yml\n"
	}
	const stepOnly = "      - continuation/continue:\n          configuration_path: next.yml\n"
	tests := []struct {
		name string
		yaml string
		// file builds the file label, creating what the test needs on disk.
		file func(t *testing.T) string
		want []row
	}{
		{
			name: "written by tee",
			yaml: setupYAML("echo x | tee next.yml"),
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "written by a redirect",
			yaml: setupYAML("./gen.sh > next.yml"),
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "written by an append",
			yaml: setupYAML("./gen.sh >> ./next.yml"),
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "written by curl -o",
			yaml: setupYAML("curl -o next.yml https://example.com/c.yml"),
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "written through a quoted redirect",
			yaml: setupYAML(`./gen.sh > "next.yml"`),
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "written by a step even though the file exists",
			yaml: setupYAML("./gen.sh > next.yml"),
			file: func(t *testing.T) string { return writeConfigTree(t, "next.yml") },
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "another path with the same prefix is not the one written",
			yaml: setupYAML("./gen.sh > next.yml.bak"),
			file: func(t *testing.T) string { return writeConfigTree(t, "next.yml") },
			want: []row{},
		},
		{
			name: "file at the repository root",
			yaml: setupYAML("echo hi"),
			file: func(t *testing.T) string { return writeConfigTree(t, "next.yml") },
			want: []row{},
		},
		{
			name: "file next to the config",
			yaml: setupYAML("echo hi"),
			file: func(t *testing.T) string { return writeConfigTree(t, filepath.Join(".circleci", "next.yml")) },
			want: []row{},
		},
		{
			name: "file nowhere on disk",
			yaml: setupYAML("echo hi"),
			file: func(t *testing.T) string { return writeConfigTree(t, "") },
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "no setup: true",
			yaml: "version: 2.1\njobs:\n  setup:\n    steps:\n      - run: ./gen.sh > next.yml\n" + stepOnly,
			want: []row{},
		},
		{
			name: "setup: false",
			yaml: "version: 2.1\nsetup: false\njobs:\n  setup:\n    steps:\n      - run: ./gen.sh > next.yml\n" + stepOnly,
			want: []row{},
		},
		{
			name: "a run in another job does not count",
			yaml: "version: 2.1\nsetup: true\njobs:\n  gen:\n    steps:\n      - run: ./gen.sh > next.yml\n  setup:\n    steps:\n" + stepOnly,
			file: func(t *testing.T) string { return writeConfigTree(t, "next.yml") },
			want: []row{},
		},
		{
			name: "written by a command the job runs",
			yaml: "version: 2.1\nsetup: true\ncommands:\n  gen:\n    steps:\n      - run: ./gen.sh > next.yml\n" +
				"jobs:\n  setup:\n    steps:\n      - gen\n" + stepOnly,
			want: []row{{ID: id, Line: 12, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "parameterised path",
			yaml: "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - continuation/continue:\n          configuration_path: << parameters.path >>\n",
			want: []row{},
		},
		{
			name: "path-filtering config-path that is missing",
			yaml: "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - path-filtering/set-parameters:\n          config-path: .circleci/continue.yml\n",
			file: func(t *testing.T) string { return writeConfigTree(t, "") },
			want: []row{{ID: id, Line: 7, Col: 24, Sev: SeverityLow}},
		},
		{
			name: "path-filtering mapping table is not a path",
			yaml: "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - path-filtering/set-parameters:\n          mapping: src/.* run-build true\n",
			file: func(t *testing.T) string { return writeConfigTree(t, "") },
			want: []row{},
		},
		{
			name: "orb job in a workflow with a missing config",
			yaml: "version: 2.1\nsetup: true\nworkflows:\n  setup:\n    jobs:\n      - continuation/continue:\n          configuration_path: .circleci/next.yml\n",
			file: func(t *testing.T) string { return writeConfigTree(t, "") },
			want: []row{{ID: id, Line: 7, Col: 31, Sev: SeverityLow}},
		},
		{
			name: "stdin: a path that exists in the working directory",
			yaml: "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - continuation/continue:\n          configuration_path: doc.go\n",
			file: func(*testing.T) string { return stdinLabel },
			want: []row{},
		},
		{
			name: "stdin: a path that exists nowhere is not checked",
			yaml: setupYAML("echo hi"),
			file: func(*testing.T) string { return stdinLabel },
			want: []row{},
		},
		{
			name: "stdin: a path written by tee",
			yaml: setupYAML("echo x | tee next.yml"),
			file: func(*testing.T) string { return stdinLabel },
			want: []row{{ID: id, Line: 8, Col: 31, Sev: SeverityLow}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file := "c.yml"
			if tc.file != nil {
				file = tc.file(t)
			}
			got := auditRowsFile(t, tc.yaml, file, id)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}

// writeConfigTree makes a temporary repository with a .circleci directory and,
// when extra is not empty, an empty file at that path inside it. It returns
// the path of the config file the test should audit.
func writeConfigTree(t *testing.T, extra string) string {
	t.Helper()
	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, ".circleci"), 0o755)
	assert.NilError(t, err)
	if extra != "" {
		err = os.WriteFile(filepath.Join(root, extra), nil, 0o600)
		assert.NilError(t, err)
	}
	return filepath.Join(root, ".circleci", "config.yml")
}

func TestDynamicConfigUntrustedMessages(t *testing.T) {
	setup := "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - run: echo x | tee next.yml\n" +
		"      - continuation/continue:\n          configuration_path: next.yml\n"
	t.Run("written by a step", func(t *testing.T) {
		got := auditMessages(t, setup, "dynamic-config-untrusted")
		assert.Check(t, cmp.DeepEqual(got, []string{
			"configuration_path next.yml is written by a run step, so the config it continues with is generated",
		}))
	})
	t.Run("not checked in", func(t *testing.T) {
		src := "version: 2.1\nsetup: true\njobs:\n  setup:\n    steps:\n      - continuation/continue:\n          configuration_path: next.yml\n"
		doc, err := Parse([]byte(src), writeConfigTree(t, ""))
		assert.NilError(t, err)
		report := Audit(doc, Options{})
		assert.Assert(t, cmp.Len(report.Findings, 1))
		assert.Check(t, cmp.Equal(report.Findings[0].Message, "configuration_path next.yml is not a file checked in next to the audited config"))
	})
}

func TestDocContexts(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{
			name: "sorted and unique across workflows and list forms",
			yaml: "workflows:\n  a:\n    jobs:\n      - build:\n          context: [zeta, alpha]\n      - test:\n          context: alpha\n" +
				"  b:\n    jobs:\n      - deploy:\n          context:\n            - mid\n            - zeta\n",
			want: []string{"alpha", "mid", "zeta"},
		},
		{
			name: "parameterised names are left out",
			yaml: "workflows:\n  a:\n    jobs:\n      - build:\n          context: [<< pipeline.parameters.ctx >>, real]\n      - test:\n          context: \"<< parameters.c >>-x\"\n",
			want: []string{"real"},
		},
		{
			name: "jobs without a context",
			yaml: "workflows:\n  a:\n    jobs:\n      - build\n      - test:\n          requires: [build]\n",
			want: []string{},
		},
		{
			name: "no workflows",
			yaml: "jobs: {}\n",
			want: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse([]byte(tc.yaml), "c.yml")
			assert.NilError(t, err)
			got := doc.Contexts()
			if len(got) == 0 {
				got = []string{}
			}
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}
}
