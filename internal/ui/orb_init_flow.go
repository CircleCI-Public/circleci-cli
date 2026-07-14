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

package ui

import (
	"context"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// orbInitStage identifies which screen of the orb-init wizard is showing. The
// wizard walks these in order, skipping stages whose value was supplied up
// front (a --private / --template-only / --org / --skip-git flag), and pausing
// on the three loading stages while an injected callback runs.
type orbInitStage int

const (
	oiVisibility        orbInitStage = iota // public vs private (select)
	oiMode                                  // automated vs template-only (select)
	oiDownloading                           // spinner: Download callback
	oiOrg                                   // organization slug (text)
	oiNamespace                             // namespace (text)
	oiOrbName                               // orb name (text)
	oiCheckingOrb                           // spinner: GetOrb callback
	oiOrbExists                             // "already exists, continue?" (confirm)
	oiLoadingCategories                     // spinner: ListCategories callback
	oiCategories                            // add-a-category loop (select)
	oiPublishingContext                     // set up publishing context? (confirm)
	oiGitSetup                              // set up git project? (confirm)
	oiBranch                                // primary git branch (text)
	oiRemote                                // remote repository URL (text)
	oiDone                                  // terminal; renders an empty frame
)

const (
	orbInitVisibilityPrompt = "Would you like to create a public or private orb?"
	orbInitModePrompt       = "Would you like to perform an automated setup of this orb?"
	orbInitCategoryPrompt   = "Add a category for this orb (choose (done) to finish)"
	orbInitCategoryDone     = "(done)"
)

var (
	orbInitVisibilityOptions = []string{"Public", "Private"}
	orbInitModeOptions       = []string{"Yes, walk me through the process", "No, just download the template"}
)

// OrbInitCategory is one selectable orb category. The flow keeps this decoupled
// from the API client's own category type (like RunGetItem for the run flow).
type OrbInitCategory struct {
	ID   string
	Name string
}

// OrbInitResult is the outcome of a completed OrbInitFlowModel run, read via
// Result() after tea.Program.Run() returns. When Cancelled or Err is set the
// remaining fields are not meaningful; otherwise they carry every decision the
// caller needs to apply the setup (create the namespace/orb, assign categories,
// set up git, and so on).
type OrbInitResult struct {
	Cancelled bool
	// Err is set when a download / orb-lookup / category-list callback failed.
	Err error

	Private      bool
	TemplateOnly bool
	OrgSlug      string
	Namespace    string
	OrbName      string
	Categories   []OrbInitCategory
	SetupContext bool
	GitSetup     bool
	Branch       string
	Remote       string
}

// OrbInitFlowOptions configures an OrbInitFlowModel. The callbacks keep the
// program decoupled from the API client and filesystem: the caller supplies
// closures for the three operations the wizard has to run between prompts.
type OrbInitFlowOptions struct {
	// Path is the target directory. It seeds the default orb name (its final
	// segment) and the default remote URL.
	Path string
	// Private / TemplateOnly / OrgSlug / SkipGit mirror the command flags. A
	// non-zero value skips the matching prompt: --private skips the visibility
	// picker, --template-only skips the mode picker, --org skips the org prompt,
	// and --skip-git skips the git-setup confirm (forcing GitSetup off).
	Private      bool
	TemplateOnly bool
	OrgSlug      string
	SkipGit      bool
	// Branch is the default primary branch (the --branch flag, default "main").
	Branch string
	// Remote is the --remote flag; unused by the interactive flow (which always
	// prompts for it) but carried for symmetry.
	Remote string

	// Download fetches and extracts the orb template into Path, removing the
	// template LICENSE when private is true. Shown behind a spinner.
	Download func(ctx context.Context, private bool) error
	// GetOrb reports whether an orb already exists under the given full name
	// ("namespace/orb"). Shown behind a spinner.
	GetOrb func(ctx context.Context, fullName string) (exists bool, err error)
	// ListCategories lists the assignable orb categories. Shown behind a spinner;
	// an empty list skips the category picker entirely.
	ListCategories func(ctx context.Context) ([]OrbInitCategory, error)

	Color bool
	// Animate reports whether the loading spinner should animate. Pass false when
	// CIRCLE_SPINNER_DISABLED is set so the loading line stays static.
	Animate bool
}

// async message types
type (
	oiDownloadedMsg struct{ err error }
	oiOrbCheckedMsg struct {
		exists bool
		err    error
	}
	oiCategoriesLoadedMsg struct {
		cats []OrbInitCategory
		err  error
	}
)

// OrbInitFlowModel is a multi-stage bubbletea model that walks the user through
// scaffolding a new orb: pick public/private, choose automated setup or a bare
// template download, then (for automated setup) gather the org, namespace, orb
// name, categories, publishing-context and git choices. It performs the three
// gating operations (template download, orb-existence check, category list) via
// injected callbacks and reports every decision through Result().
type OrbInitFlowModel struct {
	ctx  context.Context
	opts OrbInitFlowOptions

	stage orbInitStage
	width int

	sel     components.SelectModel // visibility / mode / category picker
	input   textinput.Model        // active text stage
	header  string                 // active text stage title
	defVal  string                 // active text stage default (empty field → this)
	confirm string                 // active confirm prompt
	spin    spinner.Model
	loading string // active spinner label

	// owner is the org name parsed from OrgSlug, used to default the namespace
	// and remote URL.
	owner string

	// category picker state: the not-yet-chosen categories, rebuilt into sel each
	// round of the add-a-category loop.
	catRemaining []OrbInitCategory

	result OrbInitResult
}

// NewOrbInitFlow returns an OrbInitFlowModel ready to pass to tea.NewProgram.
// The initial stage honors any flags supplied in opts.
func NewOrbInitFlow(ctx context.Context, opts OrbInitFlowOptions) OrbInitFlowModel {
	m := OrbInitFlowModel{
		ctx:  ctx,
		opts: opts,
		spin: components.NewSpinner(opts.Color),
	}
	m.result.Private = opts.Private
	m.result.Branch = opts.Branch

	switch {
	case opts.Private && opts.TemplateOnly:
		// Both decisions made: go straight to the download (kicked off by Init).
		m.result.TemplateOnly = true
		m.stage = oiDownloading
		m.loading = "Downloading orb project template..."
	case opts.Private:
		_ = m.enterSelect(oiMode, orbInitModePrompt, orbInitModeOptions)
	default:
		_ = m.enterSelect(oiVisibility, orbInitVisibilityPrompt, orbInitVisibilityOptions)
	}
	return m
}

// Result returns the final outcome. Only valid after tea.Program.Run() returns.
func (m OrbInitFlowModel) Result() OrbInitResult { return m.result }

func (m OrbInitFlowModel) Init() tea.Cmd {
	if m.stage == oiDownloading {
		return m.loadingCmd(m.cmdDownload())
	}
	return nil
}

func (m OrbInitFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		// Fall through so a select stage re-windows on resize.
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Only animate while a callback is in flight; otherwise let the spinner
		// stop ticking so input stages don't repaint needlessly.
		if m.isLoading() {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	case oiDownloadedMsg:
		return m.onDownloaded(msg)
	case oiOrbCheckedMsg:
		return m.onOrbChecked(msg)
	case oiCategoriesLoadedMsg:
		return m.onCategoriesLoaded(msg)
	}

	switch m.stage {
	case oiVisibility, oiMode, oiCategories:
		return m.updateSelect(msg)
	case oiOrg, oiNamespace, oiOrbName, oiBranch, oiRemote:
		return m.updateText(msg)
	case oiOrbExists, oiPublishingContext, oiGitSetup:
		return m.updateConfirm(msg)
	case oiDownloading, oiCheckingOrb, oiLoadingCategories:
		// Allow ctrl+c to cancel while a callback runs.
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeyCtrlC) {
			m.result.Cancelled = true
			return m, tea.Quit
		}
	case oiDone:
	}
	return m, nil
}

// --- select stages (visibility / mode / categories) ---

func (m OrbInitFlowModel) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(k, components.KeyCtrlC, components.KeyEsc) {
			m.result.Cancelled = true
			return m, tea.Quit
		}
	}

	updated, cmd := m.sel.Update(msg)
	m.sel = updated.(components.SelectModel)
	if !m.sel.Done() {
		return m, cmd
	}

	idx := m.sel.Selected()
	switch m.stage { //nolint:exhaustive // only the select stages reach here
	case oiVisibility:
		m.result.Private = idx == 1
		return m, m.afterVisibility()
	case oiMode:
		m.result.TemplateOnly = idx == 1
		return m, m.startDownload()
	case oiCategories:
		return m, m.afterCategoryPick(idx)
	}
	return m, nil
}

func (m *OrbInitFlowModel) afterVisibility() tea.Cmd {
	if m.opts.TemplateOnly {
		m.result.TemplateOnly = true
		return m.startDownload()
	}
	return m.enterSelect(oiMode, orbInitModePrompt, orbInitModeOptions)
}

func (m *OrbInitFlowModel) afterCategoryPick(idx int) tea.Cmd {
	if idx <= 0 { // "(done)"
		return m.enterConfirm(oiPublishingContext, "Automatically set up a publishing context with your API token?")
	}
	m.result.Categories = append(m.result.Categories, m.catRemaining[idx-1])
	m.catRemaining = append(m.catRemaining[:idx-1], m.catRemaining[idx:]...)
	if len(m.catRemaining) == 0 {
		return m.enterConfirm(oiPublishingContext, "Automatically set up a publishing context with your API token?")
	}
	return m.enterCategorySelect()
}

// --- text stages (org / namespace / orb name / branch / remote) ---

func (m OrbInitFlowModel) updateText(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, components.KeyCtrlC, components.KeyEsc):
			m.result.Cancelled = true
			return m, tea.Quit
		case key.Matches(k, components.KeyEnter):
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				val = m.defVal
			}
			return m.commitText(val)
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m OrbInitFlowModel) commitText(val string) (tea.Model, tea.Cmd) {
	switch m.stage { //nolint:exhaustive // only the text stages reach here
	case oiOrg:
		owner, ok := orbInitOwner(val)
		if !ok {
			// Not a <vcs>/<org> slug: keep the field open for correction.
			return m, nil
		}
		m.result.OrgSlug = val
		m.owner = owner
		return m, m.enterText(oiNamespace, "Enter the namespace to use for this orb", m.owner, m.owner)
	case oiNamespace:
		m.result.Namespace = val
		base := filepath.Base(m.opts.Path)
		return m, m.enterText(oiOrbName, "Orb name", base, base)
	case oiOrbName:
		m.result.OrbName = val
		return m, m.startCheckOrb()
	case oiBranch:
		m.result.Branch = val
		def := "https://github.com/" + m.owner + "/" + m.result.OrbName
		return m, m.enterText(oiRemote, "Enter the remote git repository URL", def, def)
	case oiRemote:
		m.result.Remote = val
		m.stage = oiDone
		return m, tea.Quit
	}
	return m, nil
}

// --- confirm stages (orb exists / publishing context / git setup) ---

func (m OrbInitFlowModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, components.KeyCtrlC, components.KeyEsc):
		m.result.Cancelled = true
		return m, tea.Quit
	case key.Matches(k, components.KeyYes):
		return m.confirmAnswered(true)
	case key.Matches(k, components.KeyNo, components.KeyEnter):
		return m.confirmAnswered(false)
	}
	return m, nil
}

func (m OrbInitFlowModel) confirmAnswered(yes bool) (tea.Model, tea.Cmd) {
	switch m.stage { //nolint:exhaustive // only the confirm stages reach here
	case oiOrbExists:
		if !yes {
			// Declining to continue with an existing orb aborts the whole init.
			m.result.Cancelled = true
			return m, tea.Quit
		}
		return m, m.startListCategories()
	case oiPublishingContext:
		m.result.SetupContext = yes
		return m, m.afterPublishingContext()
	case oiGitSetup:
		m.result.GitSetup = yes
		return m, m.afterGitSetup()
	}
	return m, nil
}

func (m *OrbInitFlowModel) afterPublishingContext() tea.Cmd {
	if m.opts.SkipGit {
		m.result.GitSetup = false
		m.stage = oiDone
		return tea.Quit
	}
	return m.enterConfirm(oiGitSetup, "Would you like to set up your git project?")
}

func (m *OrbInitFlowModel) afterGitSetup() tea.Cmd {
	if !m.result.GitSetup {
		m.stage = oiDone
		return tea.Quit
	}
	branch := m.opts.Branch
	if branch == "" {
		branch = "main"
	}
	return m.enterText(oiBranch, "Enter your primary git branch", branch, branch)
}

// --- async callbacks + their result handlers ---

func (m *OrbInitFlowModel) startDownload() tea.Cmd {
	m.stage = oiDownloading
	m.loading = "Downloading orb project template..."
	return m.loadingCmd(m.cmdDownload())
}

func (m OrbInitFlowModel) onDownloaded(msg oiDownloadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = oiDone
		return m, tea.Quit
	}
	if m.result.TemplateOnly {
		m.stage = oiDone
		return m, tea.Quit
	}
	if m.opts.OrgSlug != "" {
		m.result.OrgSlug = m.opts.OrgSlug
		if owner, ok := orbInitOwner(m.opts.OrgSlug); ok {
			m.owner = owner
		}
		return m, m.enterText(oiNamespace, "Enter the namespace to use for this orb", m.owner, m.owner)
	}
	return m, m.enterText(oiOrg, "Enter your organization as <vcs>/<org> (e.g. gh/acme)", "gh/acme", "")
}

func (m *OrbInitFlowModel) startCheckOrb() tea.Cmd {
	m.stage = oiCheckingOrb
	m.loading = "Checking whether the orb already exists..."
	return m.loadingCmd(m.cmdCheckOrb())
}

func (m OrbInitFlowModel) onOrbChecked(msg oiOrbCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = oiDone
		return m, tea.Quit
	}
	if msg.exists {
		return m, m.enterConfirm(oiOrbExists,
			"Orb "+m.result.Namespace+"/"+m.result.OrbName+" already exists, continue?")
	}
	return m, m.startListCategories()
}

func (m *OrbInitFlowModel) startListCategories() tea.Cmd {
	m.stage = oiLoadingCategories
	m.loading = "Loading orb categories..."
	return m.loadingCmd(m.cmdListCategories())
}

func (m OrbInitFlowModel) onCategoriesLoaded(msg oiCategoriesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.result.Err = msg.err
		m.stage = oiDone
		return m, tea.Quit
	}
	m.catRemaining = msg.cats
	if len(m.catRemaining) == 0 {
		return m, m.enterConfirm(oiPublishingContext, "Automatically set up a publishing context with your API token?")
	}
	return m, m.enterCategorySelect()
}

func (m OrbInitFlowModel) cmdDownload() tea.Cmd {
	ctx, fn, private := m.ctx, m.opts.Download, m.result.Private
	return func() tea.Msg { return oiDownloadedMsg{err: fn(ctx, private)} }
}

func (m OrbInitFlowModel) cmdCheckOrb() tea.Cmd {
	ctx, fn := m.ctx, m.opts.GetOrb
	full := m.result.Namespace + "/" + m.result.OrbName
	return func() tea.Msg {
		exists, err := fn(ctx, full)
		return oiOrbCheckedMsg{exists: exists, err: err}
	}
}

func (m OrbInitFlowModel) cmdListCategories() tea.Cmd {
	ctx, fn := m.ctx, m.opts.ListCategories
	return func() tea.Msg {
		cats, err := fn(ctx)
		return oiCategoriesLoadedMsg{cats: cats, err: err}
	}
}

// loadingCmd pairs a callback with the spinner tick so the loading screen
// animates while it runs (unless animation is disabled).
func (m OrbInitFlowModel) loadingCmd(async tea.Cmd) tea.Cmd {
	if m.opts.Animate {
		return tea.Batch(m.spin.Tick, async)
	}
	return async
}

func (m OrbInitFlowModel) isLoading() bool {
	switch m.stage { //nolint:exhaustive // the non-loading stages default to false
	case oiDownloading, oiCheckingOrb, oiLoadingCategories:
		return true
	}
	return false
}

// --- stage entry helpers ---

func (m *OrbInitFlowModel) enterSelect(stage orbInitStage, prompt string, options []string) tea.Cmd {
	m.sel = components.NewSelectModel(prompt, options)
	m.stage = stage
	return nil
}

func (m *OrbInitFlowModel) enterCategorySelect() tea.Cmd {
	options := make([]string, 0, len(m.catRemaining)+1)
	options = append(options, orbInitCategoryDone)
	for _, c := range m.catRemaining {
		options = append(options, c.Name)
	}
	m.sel = components.NewSelectModel(orbInitCategoryPrompt, options)
	m.stage = oiCategories
	return nil
}

func (m *OrbInitFlowModel) enterText(stage orbInitStage, header, placeholder, def string) tea.Cmd {
	ti := textinput.New()
	ti.SetVirtualCursor(false)
	if placeholder != "" {
		ti.Placeholder = placeholder
	}
	// Size the field to a comfortable fixed width rather than the placeholder
	// length — a short placeholder (e.g. "gh/acme") would otherwise give a
	// cramped field that scrolls awkwardly as a longer value is typed.
	ti.SetWidth(m.textInputWidth())
	ti.Focus()

	m.input = ti
	m.header = header
	m.defVal = def
	m.stage = stage
	return textinput.Blink
}

// textInputWidth is the visible width for a text field: a comfortable default,
// capped to the terminal so the field never runs past the right edge.
func (m OrbInitFlowModel) textInputWidth() int {
	const preferred = 50
	w := preferred
	if m.width > 0 && m.width-1 < w {
		w = m.width - 1
	}
	if w < 1 {
		w = 1
	}
	return w
}

func (m *OrbInitFlowModel) enterConfirm(stage orbInitStage, prompt string) tea.Cmd {
	m.confirm = prompt
	m.stage = stage
	return nil
}

// --- views ---

func (m OrbInitFlowModel) View() tea.View {
	switch m.stage {
	case oiVisibility, oiMode, oiCategories:
		return m.sel.View()
	case oiOrg, oiNamespace, oiOrbName, oiBranch, oiRemote:
		return m.textView()
	case oiOrbExists, oiPublishingContext, oiGitSetup:
		return m.confirmView()
	case oiDownloading, oiCheckingOrb, oiLoadingCategories:
		label := theme.HelperStyle.Render(m.loading)
		if m.opts.Animate {
			label = m.spin.View() + " " + m.loading
		}
		return tea.NewView(label)
	case oiDone:
		// Empty final frame so the last screen is cleared before the program
		// exits and the caller prints its own output in its place.
		return tea.NewView("")
	}
	return tea.NewView("")
}

func (m OrbInitFlowModel) textView() tea.View {
	header := theme.TitleStyle.Render(m.header)

	var c *tea.Cursor
	if !m.input.VirtualCursor() {
		c = m.input.Cursor()
		c.Y += lipgloss.Height(header)
	}

	// Build the whole footer as one muted run so an SGR reset does not split the
	// line on terminals that don't coalesce adjacent same-color runs.
	keys := ansi.Strip(components.Hints(
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	))
	line := keys
	if m.defVal != "" {
		line = "default: " + m.defVal + " · " + keys
	}
	footer := theme.HelperStyle.Render(line)

	str := lipgloss.JoinVertical(lipgloss.Top, header, m.input.View(), footer)
	v := tea.NewView(str)
	v.Cursor = c
	return v
}

func (m OrbInitFlowModel) confirmView() tea.View {
	title := theme.TitleStyle.Render(m.confirm)
	return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, title, " [y/N] "))
}

// orbInitOwner extracts the org name from a "<vcs>/<org>" slug, reporting
// ok=false when the slug is not in that form.
func orbInitOwner(slug string) (owner string, ok bool) {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
