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

package components

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/theme"
)

// FileTreeModel browses a set of slash-separated paths as if they were a
// filesystem: one directory on screen at a time, with enter/→ descending into a
// directory and ←/esc coming back out. It is content-agnostic — the paths need
// not exist on disk — so it suits a remote listing (a job's artifacts) as well
// as a local one.
//
// It composes SelectModel for the rows, which is what supplies the scrolling
// window, the "(3–12 of 40)" position indicator, the footer key hints and the
// height clamping. This model owns the tree, the current directory, the
// per-level cursor memory and the "/" filter.
//
// "/" opens a filter prompt. Filtering is live as you type and recursive from
// the current directory down, so matches are listed as flat paths relative to
// where you are and descending narrows the haystack. The pattern is a regular
// expression with smart case, falling back to a literal substring when it is not
// valid regex — the same rules as the pager's search. Enter commits the filter
// and leaves it applied; esc at the prompt cancels it, restoring whatever filter
// was committed before.
//
// Keys the model consumes: ↑/↓ (and k/j), pgup/pgdn, g/G, enter, →/l, ←/h, "/"
// and — while the prompt is open — everything else as pattern text. Everything
// else is left for the host, which should gate its own key handling on
// Searching() so keys typed into the prompt are not also acted on. esc outside
// the prompt is deliberately left alone: only the host knows whether it should
// clear the filter, go up a level or leave the browser, so it decides with
// FilterActive, AtRoot, ClearFilter and Ascend.
//
// The zero value is not usable; build one with NewFileTree.
type FileTreeModel struct {
	title string
	root  *fileTreeNode

	// path is the current directory as a slice of names beneath the root, and
	// cursors[i] is the row to restore on returning to the directory at path[:i]
	// — so ascending lands back on the row that was descended from. len(cursors)
	// tracks len(path).
	path    []string
	cursors []int

	rows   []fileTreeRow
	sel    SelectModel
	filter fileTreeFilter

	keys      []key.Binding
	note      string
	emptyNote string
	height    int

	chosen   bool
	selected FileTreeEntry
}

// FileTreeEntry is one leaf of the tree: a path, and an opaque payload the host
// gets back when the row is chosen. Paths are slash-separated regardless of
// platform (they describe a remote listing, not a local one); empty, "." and
// ".." segments are dropped when the tree is built, so a hostile path cannot
// climb out of the root.
type FileTreeEntry struct {
	Path string
	Ref  any
}

// fileTreeNode is one node of the built tree: a directory with children, or a
// leaf carrying its entry. leaves is the number of leaf descendants, shown as a
// directory row's file count.
type fileTreeNode struct {
	name     string
	dir      bool
	entry    FileTreeEntry
	children []*fileTreeNode
	leaves   int
}

// fileTreeRow is one on-screen row: the node it stands for and the label it
// renders as (a bare name in a directory listing, a relative path when
// filtered).
type fileTreeRow struct {
	node  *fileTreeNode
	label string
}

// fileTreeFilter is the "/" filter state: the in-progress prompt input and the
// committed pattern. Filtering applies live while typing, so the pattern in
// force is the input until it is committed.
type fileTreeFilter struct {
	typing bool
	input  string
	query  string
}

// pattern is the filter in force: the live input while the prompt is open,
// otherwise the committed query.
func (f fileTreeFilter) pattern() string {
	if f.typing {
		return f.input
	}
	return f.query
}

// Glyphs for the two row kinds. A directory gets a muted marker and a trailing
// slash; a file gets no glyph, so the name column stays flush with the directory
// names above it.
const (
	fileTreeDirGlyph = "▸"
)

// NewFileTree returns a tree of entries, opened at its root. title heads the
// view, followed by the current directory.
func NewFileTree(title string, entries []FileTreeEntry) FileTreeModel {
	m := FileTreeModel{
		title: title,
		root:  newFileTreeRoot(entries),
		keys:  []key.Binding{BindMove, BindOpen, BindUpLevel, BindSearch, BindQuitEsc},
	}
	return m.rebuild(0)
}

// WithKeys returns a copy with custom footer key bindings, replacing the default
// (move / open / up / search / quit) hint line. Use it to advertise the host's
// own actions alongside the tree's.
func (m FileTreeModel) WithKeys(keys ...key.Binding) FileTreeModel {
	m.keys = keys
	return m.rebuild(m.Cursor())
}

// WithHeight sets the number of terminal rows available, passed through to the
// underlying picker: when the listing is taller it scrolls to keep the cursor
// visible. Zero renders every row.
func (m FileTreeModel) WithHeight(rows int) FileTreeModel {
	m.height = rows
	return m.rebuild(m.Cursor())
}

// WithNote returns a copy with an informational line rendered between the title
// and the rows — the outcome of the host's last action, say. It is emitted
// verbatim, so style it in the caller. An empty note renders nothing.
func (m FileTreeModel) WithNote(note string) FileTreeModel {
	m.note = note
	return m.rebuild(m.Cursor())
}

// WithEmptyNote sets the line shown in place of the rows when the tree has no
// entries at all (e.g. "this job produced no artifacts"), so an empty browser
// explains itself. It is emitted verbatim.
func (m FileTreeModel) WithEmptyNote(note string) FileTreeModel {
	m.emptyNote = note
	return m.rebuild(m.Cursor())
}

// Empty reports whether the tree holds no entries.
func (m FileTreeModel) Empty() bool { return m.root.leaves == 0 }

// Cursor is the index of the highlighted row within the current listing.
func (m FileTreeModel) Cursor() int { return m.sel.Selected() }

// Dir is the current directory as a display path, "/" at the root.
func (m FileTreeModel) Dir() string { return "/" + strings.Join(m.path, "/") }

// AtRoot reports whether the current directory is the root, so a host can treat
// esc there as "leave the browser" rather than "go up".
func (m FileTreeModel) AtRoot() bool { return len(m.path) == 0 }

// Searching reports whether the "/" prompt is capturing input. A host must gate
// its own key handling on this being false, or keys typed into the pattern are
// acted on twice.
func (m FileTreeModel) Searching() bool { return m.filter.typing }

// FilterActive reports whether a committed filter is narrowing the listing.
func (m FileTreeModel) FilterActive() bool { return m.filter.query != "" }

// ClearFilter returns a copy with any committed filter dropped and the cursor
// back at the top of the full listing.
func (m FileTreeModel) ClearFilter() FileTreeModel {
	m.filter = fileTreeFilter{}
	return m.rebuild(0)
}

// Done reports whether the user opened a file (enter or → on a file row). The
// chosen entry is read with Selected; clear the flag with ClearSelection before
// handing control back to the tree.
func (m FileTreeModel) Done() bool { return m.chosen }

// Selected is the file the user opened. Only valid when Done.
func (m FileTreeModel) Selected() FileTreeEntry { return m.selected }

// ClearSelection returns a copy with the opened-file flag cleared, ready to
// browse again.
func (m FileTreeModel) ClearSelection() FileTreeModel {
	m.chosen = false
	m.selected = FileTreeEntry{}
	return m
}

// HighlightedIsDir reports whether the cursor is on a directory row. It is false
// when the listing is empty.
func (m FileTreeModel) HighlightedIsDir() bool {
	row, ok := m.highlighted()
	return ok && row.node.dir
}

// HighlightedLabel is the cursor row's label — a name in a directory listing, a
// relative path when filtered — for naming the row in a host's prompt. Empty
// when the listing is empty.
func (m FileTreeModel) HighlightedLabel() string {
	row, ok := m.highlighted()
	if !ok {
		return ""
	}
	return row.label
}

// HighlightedEntries are the entries the cursor row stands for: the one file, or
// every file beneath the directory (depth-first, in listing order). Empty when
// the listing is empty, so a host acting on it needs no separate guard.
func (m FileTreeModel) HighlightedEntries() []FileTreeEntry {
	row, ok := m.highlighted()
	if !ok {
		return nil
	}
	return fileTreeLeaves(row.node, nil)
}

// Entries are every entry in the tree, in listing order. A host uses it for a
// "take everything" action.
func (m FileTreeModel) Entries() []FileTreeEntry {
	return fileTreeLeaves(m.root, nil)
}

func (m FileTreeModel) Init() tea.Cmd { return nil }

func (m FileTreeModel) Update(msg tea.Msg) (FileTreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m.rebuild(m.Cursor()), nil
	case tea.KeyPressMsg:
		// While the prompt is open every keystroke is pattern text, bar the few
		// keys that edit or end it.
		if m.filter.typing {
			return m.filterKey(msg), nil
		}
		switch {
		case key.Matches(msg, BindSearch):
			m.filter.typing = true
			m.filter.input = ""
			return m.rebuild(m.Cursor()), nil
		case key.Matches(msg, KeyEnter, KeyRight):
			return m.open(), nil
		case key.Matches(msg, KeyLeft):
			return m.Ascend(), nil
		}
	}
	// Everything else — movement, paging, resize — is the picker's.
	updated, cmd := m.sel.Update(msg)
	m.sel = updated.(SelectModel)
	return m, cmd
}

func (m FileTreeModel) View() tea.View {
	return m.sel.View()
}

// open acts on the cursor row: descend into a directory, or report a file as
// chosen for the host to open. Descending drops any filter, since the listing it
// narrowed is no longer the one on screen.
func (m FileTreeModel) open() FileTreeModel {
	row, ok := m.highlighted()
	if !ok {
		return m
	}
	if !row.node.dir {
		m.chosen = true
		m.selected = row.node.entry
		return m
	}
	// A filtered row's label is a path relative to the current directory, so a
	// descent can cross more than one level. Each level crossed gets a remembered
	// cursor: the row actually left for the current one, the top for the rest.
	segs := fileTreeSegments(row.label)
	cursor := m.Cursor()
	for range segs {
		m.cursors = append(m.cursors, cursor)
		cursor = 0
	}
	m.path = append(m.path, segs...)
	m.filter = fileTreeFilter{}
	return m.rebuild(0)
}

// Ascend returns a copy showing the parent directory, with the cursor back on
// the row that was descended from and any filter dropped. At the root it is a
// no-op — a host should check AtRoot first if esc there means something else.
func (m FileTreeModel) Ascend() FileTreeModel {
	if m.AtRoot() {
		return m
	}
	cursor := m.cursors[len(m.cursors)-1]
	m.path = m.path[:len(m.path)-1]
	m.cursors = m.cursors[:len(m.cursors)-1]
	m.filter = fileTreeFilter{}
	return m.rebuild(cursor)
}

// filterKey handles a keystroke while the "/" prompt is open: editing the
// pattern, committing it with enter, or cancelling with esc (which restores the
// previously committed filter, if any). The listing re-filters on every
// keystroke, with the cursor pulled back to the top whenever the pattern
// changes.
func (m FileTreeModel) filterKey(msg tea.KeyPressMsg) FileTreeModel {
	switch {
	case key.Matches(msg, KeyEnter):
		m.filter.typing = false
		m.filter.query = m.filter.input
		m.filter.input = ""
		return m.rebuild(m.Cursor())
	case key.Matches(msg, KeyEsc, KeyCtrlC):
		m.filter.typing = false
		m.filter.input = ""
		return m.rebuild(0)
	case key.Matches(msg, KeyBackspace):
		if r := []rune(m.filter.input); len(r) > 0 {
			m.filter.input = string(r[:len(r)-1])
		}
		return m.rebuild(0)
	}
	// Append printable characters; msg.Text is empty for non-printable keys.
	m.filter.input += msg.Text
	return m.rebuild(0)
}

// highlighted returns the cursor row, false when the listing is empty.
func (m FileTreeModel) highlighted() (fileTreeRow, bool) {
	i := m.Cursor()
	if i < 0 || i >= len(m.rows) {
		return fileTreeRow{}, false
	}
	return m.rows[i], true
}

// rebuild recomputes the current listing and the picker that renders it, placing
// the cursor at index cursor. Every state change that can alter the rows — a
// descent, a filter keystroke, a resize, a builder — goes through here, so the
// rows and the picker never disagree.
func (m FileTreeModel) rebuild(cursor int) FileTreeModel {
	m.rows = m.listing()
	labels := make([]string, len(m.rows))
	icons := make([]string, len(m.rows))
	for i, row := range m.rows {
		labels[i] = row.label
		if row.node.dir {
			icons[i] = theme.HelperStyle.Render(fileTreeDirGlyph)
			labels[i] = row.label + "/  (" + fileCount(row.node.leaves) + ")"
		}
	}
	m.sel = NewSelectModel(m.title+" "+m.Dir(), labels).
		WithIcons(icons).
		WithNote(m.noteBlock()).
		WithKeys(m.keys...).
		WithHeight(m.height).
		WithCursor(cursor)
	return m
}

// noteBlock is the block of lines between the title and the rows: the host's
// note, then the filter's prompt or status, then an explanation when there is
// nothing to show.
func (m FileTreeModel) noteBlock() string {
	var lines []string
	if m.note != "" {
		lines = append(lines, m.note)
	}
	switch pattern := m.filter.pattern(); {
	case m.filter.typing:
		// The live prompt, styled like the pager's so "/" reads the same
		// everywhere in the CLI, with the running hit count beside it so the
		// pattern can be judged as it is typed.
		prompt := theme.HelperStyle.Render("/"+m.filter.input) + theme.AccentStyle.Render("▌")
		if m.filter.input != "" {
			prompt += theme.HelperStyle.Render(" " + matchCount(len(m.rows)))
		}
		lines = append(lines, prompt)
	case pattern != "":
		lines = append(lines, theme.AccentStyle.Render("/"+pattern)+
			theme.HelperStyle.Render(" "+matchCount(len(m.rows))))
	}
	switch {
	case len(m.rows) > 0:
	case m.Empty() && m.emptyNote != "":
		lines = append(lines, m.emptyNote)
	case m.filter.pattern() != "":
		lines = append(lines, theme.WarningStyle.Render("no files match "+m.filter.pattern()))
	}
	return strings.Join(lines, "\n")
}

// listing is the rows for the current directory: its children, or — with a
// filter in force — every descendant whose path relative to it matches, listed
// flat.
func (m FileTreeModel) listing() []fileTreeRow {
	dir := m.node()
	if dir == nil {
		return nil
	}
	pattern := m.filter.pattern()
	if pattern == "" {
		rows := make([]fileTreeRow, 0, len(dir.children))
		for _, child := range dir.children {
			rows = append(rows, fileTreeRow{node: child, label: child.name})
		}
		return rows
	}
	re := compileSearch(pattern)
	if re == nil {
		return nil
	}
	var rows []fileTreeRow
	var walk func(n *fileTreeNode, prefix string)
	walk = func(n *fileTreeNode, prefix string) {
		for _, child := range n.children {
			rel := prefix + child.name
			if re.MatchString(rel) {
				rows = append(rows, fileTreeRow{node: child, label: rel})
			}
			if child.dir {
				walk(child, rel+"/")
			}
		}
	}
	walk(dir, "")
	// Keep the directories-first order of a plain listing, so a matching
	// directory is easy to spot among the files beneath it.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].node.dir != rows[j].node.dir {
			return rows[i].node.dir
		}
		return fileTreeLess(rows[i].label, rows[j].label)
	})
	return rows
}

// node is the node for the current directory, nil if the path no longer resolves.
func (m FileTreeModel) node() *fileTreeNode {
	cur := m.root
	for _, name := range m.path {
		next := cur.dirChild(name)
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// dirChild returns the directory child called name, or nil.
func (n *fileTreeNode) dirChild(name string) *fileTreeNode {
	for _, c := range n.children {
		if c.dir && c.name == name {
			return c
		}
	}
	return nil
}

// hasChild reports whether a child called name already exists, of either kind.
func (n *fileTreeNode) hasChild(name string) bool {
	for _, c := range n.children {
		if c.name == name {
			return true
		}
	}
	return false
}

// newFileTreeRoot builds the tree from a flat list of paths. Entries whose path
// contributes no usable segment are skipped, and the first entry wins when two
// share a path.
func newFileTreeRoot(entries []FileTreeEntry) *fileTreeNode {
	root := &fileTreeNode{dir: true}
	for _, e := range entries {
		segs := fileTreeSegments(e.Path)
		if len(segs) == 0 {
			continue
		}
		cur := root
		for _, seg := range segs[:len(segs)-1] {
			next := cur.dirChild(seg)
			if next == nil {
				next = &fileTreeNode{name: seg, dir: true}
				cur.children = append(cur.children, next)
			}
			cur = next
		}
		if name := segs[len(segs)-1]; !cur.hasChild(name) {
			cur.children = append(cur.children, &fileTreeNode{name: name, entry: e})
		}
	}
	sortFileTree(root)
	countLeaves(root)
	return root
}

// fileTreeSegments splits a slash-separated path into usable names: empty, "."
// and ".." segments are dropped, so no path can address anything above the root.
func fileTreeSegments(path string) []string {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, seg := range parts {
		switch seg {
		case "", ".", "..":
			continue
		}
		out = append(out, seg)
	}
	return out
}

// sortFileTree orders every directory's children: directories first, then files,
// each group by name.
func sortFileTree(n *fileTreeNode) {
	sort.SliceStable(n.children, func(i, j int) bool {
		if n.children[i].dir != n.children[j].dir {
			return n.children[i].dir
		}
		return fileTreeLess(n.children[i].name, n.children[j].name)
	})
	for _, c := range n.children {
		if c.dir {
			sortFileTree(c)
		}
	}
}

// fileTreeLess orders two names case-insensitively, falling back to a plain
// comparison so the order is total (and therefore stable across runs).
func fileTreeLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}

// countLeaves fills in each directory's leaf-descendant count, returning the
// node's own.
func countLeaves(n *fileTreeNode) int {
	if !n.dir {
		return 1
	}
	total := 0
	for _, c := range n.children {
		total += countLeaves(c)
	}
	n.leaves = total
	return total
}

// fileTreeLeaves appends every leaf under n (n itself when it is one) in listing
// order.
func fileTreeLeaves(n *fileTreeNode, out []FileTreeEntry) []FileTreeEntry {
	if !n.dir {
		return append(out, n.entry)
	}
	for _, c := range n.children {
		out = fileTreeLeaves(c, out)
	}
	return out
}

// fileCount renders a directory's leaf count for its row, singular where it
// matters.
func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// matchCount renders the filter's hit count for the status line.
func matchCount(n int) string {
	if n == 1 {
		return "1 match"
	}
	return fmt.Sprintf("%d matches", n)
}
