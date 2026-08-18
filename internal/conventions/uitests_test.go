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

package conventions_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// repoRoot is where this test's own package sits relative to the repository, so
// both Go modules (the CLI and clikit) can be read from one place.
const repoRoot = "../.."

// uiPackages are the packages these rules cover: the reusable interactive
// components and the flows that compose them. Everything here is a bubbletea
// program or a piece of one, which is what makes the teatest rule meaningful.
var uiPackages = []string{
	"internal/ui",
	"clikit/ui",
	"clikit/ui/components",
}

// teatestImport is the package agents/14-testing.md requires component-level
// tests to drive their model through.
const teatestImport = "github.com/charmbracelet/x/exp/teatest/v2"

// maxUngroupedStmts is how many top-level statements a test may have before
// agents/14-testing.md's "break tests into meaningful groupings with t.Run"
// stops being a preference and starts being the difference between a readable
// failure and a wall of assertions. It is a proxy for "this test has phases",
// deliberately loose: a short, single-purpose test needs no subtests.
const maxUngroupedStmts = 11

// teatestExempt lists test files that drive a bubbletea model without teatest.
// They predate the rule; each entry is a file to convert, not a style to copy —
// see the note at the top of the file itself.
var teatestExempt = map[string]string{
	"internal/ui/run_filter_dialog_test.go": "drives runFilterDialog by calling Update directly",
}

// groupingExempt lists long test functions that predate the t.Run rule, keyed by
// "<package>/<file>:<test>". As with teatestExempt, this is a to-do list: group
// one and delete its line. Nothing new belongs here.
var groupingExempt = map[string]string{
	"clikit/ui/components/pager_test.go:TestPagerModel_SearchHistoryRecall":                      "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_AdoptedRepoSkipsBranchAndRemote":          "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_AdoptedRepoWithoutCommitsStillAsksBranch": "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_Categories":                               "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_CategoryLimit":                            "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_GathersOrgNamespaceOrbName":               "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_GitEndToEnd":                              "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_GitGathersBranchAndRemote":                "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_NoGit":                                    "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_OrbExistsContinue":                        "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_OrbExistsDeclineCancels":                  "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_SkipGitPresetEndsAfterContext":            "predates the rule",
	"internal/ui/orb_init_flow_test.go:TestOrbInitFlow_TemplateOnly":                             "predates the rule",
	"internal/ui/run_filter_dialog_test.go:TestRunFilterDialog_TabSwitching":                     "predates the rule",
	"internal/ui/run_get_flow_test.go:TestRunGetFlow_FilterApplyCreated":                         "predates the rule",
	"internal/ui/run_get_flow_test.go:TestRunGetFlow_StepPagerCollapsesCarriageReturns":          "predates the rule",
}

// TestUIComponentTestsUseTeatest checks that every bubbletea model in the UI
// packages with a test file of its own is driven through teatest, as
// agents/14-testing.md requires. A model tested by calling Update directly
// bypasses the program loop — message ordering, commands, the renderer — which is
// exactly where the interesting bugs live.
func TestUIComponentTestsUseTeatest(t *testing.T) {
	for _, pkg := range uiPackages {
		t.Run(pkg, func(t *testing.T) {
			dir := filepath.Join(repoRoot, pkg)
			files, err := os.ReadDir(dir)
			assert.NilError(t, err)

			for _, entry := range files {
				name := entry.Name()
				if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				if !declaresBubbleteaUpdate(t, filepath.Join(dir, name)) {
					continue
				}
				testFile := strings.TrimSuffix(name, ".go") + "_test.go"
				testPath := filepath.Join(dir, testFile)
				if _, err := os.Stat(testPath); err != nil {
					// No test file of its own: covered elsewhere, or not covered at
					// all — either way, a different problem than this rule's.
					continue
				}
				key := pkg + "/" + testFile
				if reason, ok := teatestExempt[key]; ok {
					t.Logf("%s: exempt (%s) — convert it and drop the exemption", key, reason)
					continue
				}
				imports := fileImports(t, testPath)
				assert.Check(t, imports[teatestImport],
					"%s drives a bubbletea model, so it must import %s (agents/14-testing.md)", key, teatestImport)
			}
		})
	}
}

// TestUITestsGroupWithSubtests checks that long test functions in the UI packages
// break themselves up with t.Run, as agents/14-testing.md requires: the grouping
// is what names the failing phase in the test output instead of leaving a reader
// to map an assertion back onto a comment.
func TestUITestsGroupWithSubtests(t *testing.T) {
	for _, pkg := range uiPackages {
		t.Run(pkg, func(t *testing.T) {
			dir := filepath.Join(repoRoot, pkg)
			files, err := os.ReadDir(dir)
			assert.NilError(t, err)

			for _, entry := range files {
				name := entry.Name()
				if !strings.HasSuffix(name, "_test.go") {
					continue
				}
				for _, fn := range longUngroupedTests(t, filepath.Join(dir, name)) {
					key := pkg + "/" + name + ":" + fn
					if reason, ok := groupingExempt[key]; ok {
						t.Logf("%s: exempt (%s) — group it and drop the exemption", key, reason)
						continue
					}
					t.Errorf("%s has more than %d statements and no t.Run: group it into subtests (agents/14-testing.md)",
						key, maxUngroupedStmts)
				}
			}
		})
	}
}

// declaresBubbleteaUpdate reports whether path declares an Update method taking a
// bubbletea message — the marker of a model that has to be driven by a program
// rather than called.
func declaresBubbleteaUpdate(t *testing.T, path string) bool {
	t.Helper()
	file := parse(t, path)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "Update" || len(fn.Type.Params.List) == 0 {
			continue
		}
		// tea.Msg, however the bubbletea import is named.
		if sel, ok := fn.Type.Params.List[0].Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Msg" {
			return true
		}
	}
	return false
}

// fileImports is the set of import paths in path.
func fileImports(t *testing.T, path string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, spec := range parse(t, path).Imports {
		out[strings.Trim(spec.Path.Value, `"`)] = true
	}
	return out
}

// longUngroupedTests names the Test functions in path with more than
// maxUngroupedStmts statements and no t.Run call.
func longUngroupedTests(t *testing.T, path string) []string {
	t.Helper()
	decls := parse(t, path).Decls
	out := make([]string, 0, len(decls))
	for _, decl := range decls {
		fn, ok := decl.(*ast.FuncDecl)
		switch {
		case !ok, fn == nil:
			continue
		case fn.Recv != nil, fn.Body == nil:
			continue
		case !strings.HasPrefix(fn.Name.Name, "Test"):
			continue
		case len(fn.Body.List) <= maxUngroupedStmts:
			continue
		case callsSubtest(fn.Body):
			continue
		}
		out = append(out, fn.Name.Name)
	}
	return out
}

// callsSubtest reports whether body contains a t.Run call. It matches on the
// receiver being named t so a program's own Run method cannot pass for a subtest.
func callsSubtest(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		if recv, ok := sel.X.(*ast.Ident); ok && recv.Name == "t" {
			found = true
		}
		return true
	})
	return found
}

func parse(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	assert.NilError(t, err)
	return file
}
