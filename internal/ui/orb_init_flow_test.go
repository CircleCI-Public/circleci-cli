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

package ui_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// --- harness (mirrors run_get_flow_test.go) ---

// oiQuitMsg tells oiHarness to end the program without disturbing the inner
// model, so its live (mid-flow) View can be snapshotted. The flow ignores
// unknown message types, so sending it does not perturb state.
type oiQuitMsg struct{}

// oiHarness drives an OrbInitFlowModel as a standalone teatest program. It quits
// on oiQuitMsg for snapshotting, and otherwise lets the inner model's own
// tea.Quit (self-completion / cancel) propagate.
type oiHarness struct {
	m ui.OrbInitFlowModel
}

func (h oiHarness) Init() tea.Cmd { return h.m.Init() }

func (h oiHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(oiQuitMsg); ok {
		return h, tea.Quit
	}
	u, cmd := h.m.Update(msg)
	h.m = u.(ui.OrbInitFlowModel)
	return h, cmd
}

func (h oiHarness) View() tea.View { return h.m.View() }

var (
	oiKeyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	oiKeyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	oiKeyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	oiKeyY     = tea.KeyPressMsg{Code: 'y', Text: "y"}
	oiKeyN     = tea.KeyPressMsg{Code: 'n', Text: "n"}
)

func startOrbFlow(t *testing.T, m ui.OrbInitFlowModel) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, oiHarness{m: m}, teatest.WithInitialTermSize(80, 24))
}

func oiWaitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}, teatest.WithDuration(4*time.Second))
}

// oiSnapshot quits via oiQuitMsg (leaving the inner model on its live stage) and
// returns its ANSI-stripped frame.
func oiSnapshot(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(oiQuitMsg{})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(oiHarness)
	return ansi.Strip(fm.m.View().Content)
}

// oiResult waits for the flow to end on its own (self-quit) and returns its
// gathered result.
func oiResult(t *testing.T, tm *teatest.TestModel) ui.OrbInitResult {
	t.Helper()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(oiHarness)
	return fm.m.Result()
}

// oiType sends each rune of s as an individual key press so the textinput
// receives it as typed text.
func oiType(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// oiFakes records whether each injected callback fired and with what argument.
type oiFakes struct {
	downloadCalled  bool
	downloadPrivate bool
	orbExists       bool
	categories      []ui.OrbInitCategory
}

// newOrbFlow builds a flow with recording callbacks. opts is passed through
// (with the fakes' callbacks filled in). Animation is off so the loading stages
// resolve deterministically under teatest.
func newOrbFlow(f *oiFakes, opts ui.OrbInitFlowOptions) ui.OrbInitFlowModel {
	if opts.Path == "" {
		opts.Path = "my-orb"
	}
	opts.Animate = false
	opts.Download = func(_ context.Context, private bool) error {
		f.downloadCalled = true
		f.downloadPrivate = private
		return nil
	}
	opts.GetOrb = func(context.Context, string) (bool, error) { return f.orbExists, nil }
	opts.ListCategories = func(context.Context) ([]ui.OrbInitCategory, error) { return f.categories, nil }
	return ui.NewOrbInitFlow(context.Background(), opts)
}

// TestOrbInitFlow_VisibilityToMode confirms the flow opens on the public/private
// picker and advances to the automated-setup picker on selection.
func TestOrbInitFlow_VisibilityToMode(t *testing.T) {
	tm := startOrbFlow(t, newOrbFlow(&oiFakes{}, ui.OrbInitFlowOptions{}))
	oiWaitFor(t, tm, "public or private orb")

	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	assert.Check(t, cmp.Contains(oiSnapshot(t, tm), "automated setup"))
}

// TestOrbInitFlow_PrivateSkipsVisibility confirms --private opens the flow on the
// mode picker (the visibility question is already answered).
func TestOrbInitFlow_PrivateSkipsVisibility(t *testing.T) {
	tm := startOrbFlow(t, newOrbFlow(&oiFakes{}, ui.OrbInitFlowOptions{Private: true}))
	oiWaitFor(t, tm, "automated setup")

	v := oiSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "automated setup"))
	assert.Check(t, !bytes.Contains([]byte(v), []byte("public or private")))
}

// TestOrbInitFlow_TemplateOnly drives public → "just download the template" and
// confirms the flow downloads (non-private) and completes template-only.
func TestOrbInitFlow_TemplateOnly(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{}))
	oiWaitFor(t, tm, "public or private orb")

	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyDown)  // "No, just download the template"
	tm.Send(oiKeyEnter) // download runs, then the flow ends

	res := oiResult(t, tm)
	assert.Check(t, res.TemplateOnly)
	assert.Check(t, !res.Private)
	assert.Check(t, !res.Cancelled)
	assert.Check(t, f.downloadCalled)
	assert.Check(t, !f.downloadPrivate)
}

// TestOrbInitFlow_TemplateOnlyPrivatePreset confirms that --private
// --template-only downloads immediately (both questions pre-answered) and passes
// private=true to the download callback.
func TestOrbInitFlow_TemplateOnlyPrivatePreset(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{Private: true, TemplateOnly: true}))

	res := oiResult(t, tm)
	assert.Check(t, res.TemplateOnly)
	assert.Check(t, res.Private)
	assert.Check(t, f.downloadCalled)
	assert.Check(t, f.downloadPrivate)
}

// TestOrbInitFlow_GathersOrgNamespaceOrbName drives the org prompt through the
// namespace and orb-name prompts, confirming the org owner defaults the
// namespace and the path segment defaults the orb name.
func TestOrbInitFlow_GathersOrgNamespaceOrbName(t *testing.T) {
	f := &oiFakes{}
	// No --org: the flow prompts for it after the (automated) download.
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{Path: "my-orb"}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter) // "Yes, walk me through" → download → org prompt
	oiWaitFor(t, tm, "Enter your organization")

	oiType(tm, "gh/acme")
	tm.Send(oiKeyEnter)
	// The namespace defaults to the org owner ("acme"). The org prompt header
	// also begins "Enter ", which is diff-rewritten in place, so sync on the
	// distinctive tail rather than the shared prefix.
	oiWaitFor(t, tm, "namespace to use for this orb")
	v := oiSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "default: acme"))
}

// TestOrbInitFlow_InvalidOrgStays confirms a non-<vcs>/<org> entry keeps the org
// prompt open rather than advancing.
func TestOrbInitFlow_InvalidOrgStays(t *testing.T) {
	tm := startOrbFlow(t, newOrbFlow(&oiFakes{}, ui.OrbInitFlowOptions{}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter) // automated → download → org prompt
	oiWaitFor(t, tm, "Enter your organization")

	oiType(tm, "notaslug")
	tm.Send(oiKeyEnter) // rejected: no "/" → stays on the org prompt

	v := oiSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "Enter your organization"))
}

// TestOrbInitFlow_NoGit gathers a full automated setup with git declined, and
// confirms the result carries the resolved decisions.
func TestOrbInitFlow_NoGit(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{
		Path:    "my-orb",
		OrgSlug: "gh/acme", // skips the org prompt
	}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter) // automated → download → namespace prompt
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter) // accept default namespace "acme"
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter) // accept default orb name "my-orb" → check orb → (no categories)
	oiWaitFor(t, tm, "publishing context")
	tm.Send(oiKeyN) // no publishing context
	oiWaitFor(t, tm, "set up your git project")
	tm.Send(oiKeyN) // no git → flow ends

	res := oiResult(t, tm)
	assert.Check(t, cmp.Equal(res.OrgSlug, "gh/acme"))
	assert.Check(t, cmp.Equal(res.Namespace, "acme"))
	assert.Check(t, cmp.Equal(res.OrbName, "my-orb"))
	assert.Check(t, !res.SetupContext)
	assert.Check(t, !res.GitSetup)
	assert.Check(t, !res.Cancelled)
	assert.Check(t, cmp.Len(res.Categories, 0))
}

// TestOrbInitFlow_OrbExistsContinue confirms the "already exists, continue?"
// confirm appears when GetOrb reports the orb exists, and that "y" proceeds.
func TestOrbInitFlow_OrbExistsContinue(t *testing.T) {
	f := &oiFakes{orbExists: true}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{Path: "my-orb", OrgSlug: "gh/acme"}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter) // Public
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter) // automated → download → namespace
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter) // check orb → exists
	oiWaitFor(t, tm, "already exists, continue?")

	tm.Send(oiKeyY) // continue → publishing context confirm
	oiWaitFor(t, tm, "publishing context")
	assert.Check(t, cmp.Contains(oiSnapshot(t, tm), "publishing context"))
}

// TestOrbInitFlow_OrbExistsDeclineCancels confirms declining the "already
// exists, continue?" prompt cancels the whole init.
func TestOrbInitFlow_OrbExistsDeclineCancels(t *testing.T) {
	f := &oiFakes{orbExists: true}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{Path: "my-orb", OrgSlug: "gh/acme"}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "already exists, continue?")

	tm.Send(oiKeyN) // decline → cancel
	res := oiResult(t, tm)
	assert.Check(t, res.Cancelled)
}

// TestOrbInitFlow_Categories confirms the category picker offers the listed
// categories and records the chosen one, then "(done)" advances the flow.
func TestOrbInitFlow_Categories(t *testing.T) {
	f := &oiFakes{categories: []ui.OrbInitCategory{
		{ID: "c1", Name: "Testing"},
		{ID: "c2", Name: "Deployment"},
	}}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{Path: "my-orb", OrgSlug: "gh/acme"}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter) // → check orb (not exists) → load categories → picker
	oiWaitFor(t, tm, "Add a category")

	tm.Send(oiKeyDown)  // "(done)" → "Testing"
	tm.Send(oiKeyEnter) // choose "Testing"; one category remains → picker again
	oiWaitFor(t, tm, "Deployment")
	tm.Send(oiKeyEnter) // cursor on "(done)" → finish → publishing context
	oiWaitFor(t, tm, "publishing context")
	tm.Send(oiKeyN)
	oiWaitFor(t, tm, "set up your git project")
	tm.Send(oiKeyN)

	res := oiResult(t, tm)
	assert.Assert(t, cmp.Len(res.Categories, 1))
	assert.Check(t, cmp.Equal(res.Categories[0].Name, "Testing"))
}

// TestOrbInitFlow_GitGathersBranchAndRemote confirms accepting git setup gathers
// the branch and remote, defaulting them from the flag and the org/orb.
func TestOrbInitFlow_GitGathersBranchAndRemote(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{
		Path:    "my-orb",
		OrgSlug: "gh/acme",
		Branch:  "main",
	}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "publishing context")
	tm.Send(oiKeyN)
	oiWaitFor(t, tm, "set up your git project")
	tm.Send(oiKeyY) // yes → branch prompt
	oiWaitFor(t, tm, "Enter your primary git branch")
	tm.Send(oiKeyEnter) // default "main"
	oiWaitFor(t, tm, "remote git repository URL")

	// The remote defaults from the org owner and orb name.
	v := oiSnapshot(t, tm)
	assert.Check(t, cmp.Contains(v, "https://github.com/acme/my-orb"))
}

// TestOrbInitFlow_GitEndToEnd accepts git setup and accepts both defaults,
// confirming the gathered branch and remote reach the result.
func TestOrbInitFlow_GitEndToEnd(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{
		Path:    "my-orb",
		OrgSlug: "gh/acme",
		Branch:  "main",
	}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "publishing context")
	tm.Send(oiKeyY) // set up publishing context
	oiWaitFor(t, tm, "set up your git project")
	tm.Send(oiKeyY)
	oiWaitFor(t, tm, "Enter your primary git branch")
	tm.Send(oiKeyEnter) // "main"
	oiWaitFor(t, tm, "remote git repository URL")
	tm.Send(oiKeyEnter) // default remote → flow ends

	res := oiResult(t, tm)
	assert.Check(t, res.SetupContext)
	assert.Check(t, res.GitSetup)
	assert.Check(t, cmp.Equal(res.Branch, "main"))
	assert.Check(t, cmp.Equal(res.Remote, "https://github.com/acme/my-orb"))
}

// TestOrbInitFlow_SkipGitPresetEndsAfterContext confirms --skip-git ends the
// flow right after the publishing-context confirm (no git prompts) with GitSetup
// off.
func TestOrbInitFlow_SkipGitPresetEndsAfterContext(t *testing.T) {
	f := &oiFakes{}
	tm := startOrbFlow(t, newOrbFlow(f, ui.OrbInitFlowOptions{
		Path:    "my-orb",
		OrgSlug: "gh/acme",
		SkipGit: true,
	}))
	oiWaitFor(t, tm, "public or private orb")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "automated setup")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Enter the namespace")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "Orb name")
	tm.Send(oiKeyEnter)
	oiWaitFor(t, tm, "publishing context")
	tm.Send(oiKeyN) // no context → flow ends (git skipped)

	res := oiResult(t, tm)
	assert.Check(t, !res.GitSetup)
	assert.Check(t, !res.Cancelled)
}

// TestOrbInitFlow_CancelOnEsc confirms esc cancels the flow from a picker.
func TestOrbInitFlow_CancelOnEsc(t *testing.T) {
	tm := startOrbFlow(t, newOrbFlow(&oiFakes{}, ui.OrbInitFlowOptions{}))
	oiWaitFor(t, tm, "public or private orb")

	tm.Send(oiKeyEsc)
	res := oiResult(t, tm)
	assert.Check(t, res.Cancelled)
}
