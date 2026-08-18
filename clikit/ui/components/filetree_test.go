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

package components_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
)

// artifactPaths is a representative artifact listing: two directory levels, a
// shared prefix, a file at the root, and names that only sort correctly when
// directories come first.
var artifactPaths = []string{
	"coverage.out",
	"reports/junit/results.xml",
	"reports/junit/shards/0.xml",
	"reports/junit/shards/1.xml",
	"reports/summary.txt",
	"screenshots/login.png",
}

// treeHarness wraps FileTreeModel so it can be driven as a standalone program in
// teatest. The model never returns tea.Quit (a host flow decides when browsing
// ends), so the harness quits on request; treeProbeMsg hands the current model
// back to the test from inside the program loop, which both keeps the program
// running and guarantees every key sent beforehand has been processed.
type treeHarness struct {
	m components.FileTreeModel
}

type treeQuitMsg struct{}

type treeProbeMsg struct{ model chan components.FileTreeModel }

func (h treeHarness) Init() tea.Cmd { return h.m.Init() }

func (h treeHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case treeQuitMsg:
		return h, tea.Quit
	case treeProbeMsg:
		msg.model <- h.m
		return h, nil
	}
	m, cmd := h.m.Update(msg)
	h.m = m
	return h, cmd
}

func (h treeHarness) View() tea.View { return h.m.View() }

// treeTimeout bounds every wait on a teatest program here. It is a liveness
// ceiling, not a latency budget: a probe returns as soon as the program answers.
const treeTimeout = 10 * time.Second

// startTree runs a tree in an 80×24 terminal and returns the test program, ended
// on cleanup. The initial size means the model is exercised with the same
// windowing a real terminal gives it.
func startTree(t *testing.T, paths ...string) *teatest.TestModel {
	t.Helper()
	entries := make([]components.FileTreeEntry, len(paths))
	for i, p := range paths {
		entries[i] = components.FileTreeEntry{Path: p, Ref: p}
	}
	return startTreeModel(t, components.NewFileTree("Artifacts", entries))
}

func startTreeModel(t *testing.T, m components.FileTreeModel) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, treeHarness{m: m}, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() {
		tm.Send(treeQuitMsg{})
		tm.WaitFinished(t, teatest.WithFinalTimeout(treeTimeout))
	})
	return tm
}

// probe returns the tree's current state, answered from inside the program loop
// so every key sent before it has already been applied.
func probe(t *testing.T, tm *teatest.TestModel) components.FileTreeModel {
	t.Helper()
	// Buffered so the program loop is never left blocked on a send, however this
	// test goroutine ends.
	model := make(chan components.FileTreeModel, 1)
	tm.Send(treeProbeMsg{model: model})
	select {
	case m := <-model:
		return m
	case <-time.After(treeTimeout):
		t.Fatalf("no state from the tree after %s", treeTimeout)
		return components.FileTreeModel{}
	}
}

func send(tm *teatest.TestModel, keys ...tea.KeyPressMsg) {
	for _, k := range keys {
		tm.Send(k)
	}
}

func typeText(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// rowLabels are the visible row labels of the tree's current listing, in order,
// read back from the rendered frame with styling stripped. The frame is a title
// line, an optional note block, the rows, then the key hints; rows are the lines
// carrying the "›"/two-space row prefix.
func rowLabels(t *testing.T, m components.FileTreeModel) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		switch {
		case strings.HasPrefix(line, "› "), strings.HasPrefix(line, "  "):
			label := strings.TrimSpace(strings.TrimPrefix(line, "› "))
			// Directory rows carry the muted glyph in the icon column.
			out = append(out, strings.TrimPrefix(label, "▸ "))
		}
	}
	return out
}

func frame(t *testing.T, m components.FileTreeModel) string {
	t.Helper()
	return ansi.Strip(m.View().Content)
}

var (
	treeDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	treeUp    = tea.KeyPressMsg{Code: tea.KeyUp}
	treeRight = tea.KeyPressMsg{Code: tea.KeyRight}
	treeLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	treeEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	treeEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	treeSlash = tea.KeyPressMsg{Code: '/', Text: "/"}
	treeBksp  = tea.KeyPressMsg{Code: tea.KeyBackspace}
)

func TestFileTree_ListsDirectoriesFirst(t *testing.T) {
	m := probe(t, startTree(t, artifactPaths...))

	t.Run("directories sort first, then files, with a file count each", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(rowLabels(t, m), []string{
			"reports/  (4 files)",
			"screenshots/  (1 file)",
			"coverage.out",
		}))
	})

	t.Run("it opens at the root, named in the title", func(t *testing.T) {
		assert.Check(t, cmp.Equal(m.Dir(), "/"))
		assert.Check(t, m.AtRoot())
		assert.Check(t, !m.Empty())
		assert.Check(t, cmp.Contains(frame(t, m), "Artifacts /"))
	})
}

func TestFileTree_DescendAndAscendRestoresCursor(t *testing.T) {
	tm := startTree(t, artifactPaths...)

	assert.Assert(t, t.Run("→ descends into the highlighted directory", func(t *testing.T) {
		send(tm, treeDown, treeRight)
		m := probe(t, tm)
		assert.Check(t, cmp.Equal(m.Dir(), "/screenshots"))
		assert.Check(t, !m.AtRoot())
		assert.Check(t, cmp.DeepEqual(rowLabels(t, m), []string{"login.png"}))
	}))

	assert.Assert(t, t.Run("← comes back out onto the row that was left", func(t *testing.T) {
		send(tm, treeLeft)
		m := probe(t, tm)
		assert.Check(t, cmp.Equal(m.Dir(), "/"))
		assert.Check(t, cmp.Equal(m.Cursor(), 1))
		assert.Check(t, cmp.Equal(m.HighlightedLabel(), "screenshots"))
	}))

	assert.Assert(t, t.Run("nesting works, and ← at the root is inert", func(t *testing.T) {
		send(tm, treeUp, treeEnter, treeEnter)
		assert.Check(t, cmp.Equal(probe(t, tm).Dir(), "/reports/junit"))
		send(tm, treeLeft, treeLeft, treeLeft)
		assert.Check(t, cmp.Equal(probe(t, tm).Dir(), "/"))
	}))
}

func TestFileTree_OpenFileReportsSelection(t *testing.T) {
	tm := startTree(t, artifactPaths...)
	// "coverage.out" is the third row: the two directories sort first.
	send(tm, treeDown, treeDown, treeEnter)
	m := probe(t, tm)

	t.Run("enter on a file reports it, with the host's payload", func(t *testing.T) {
		assert.Check(t, m.Done())
		assert.Check(t, cmp.Equal(m.Selected().Path, "coverage.out"))
		assert.Check(t, cmp.Equal(m.Selected().Ref, "coverage.out"))
		assert.Check(t, !m.HighlightedIsDir())
	})

	t.Run("the host clears the selection to browse on", func(t *testing.T) {
		cleared := m.ClearSelection()
		assert.Check(t, !cleared.Done())
		assert.Check(t, cmp.Equal(cleared.Selected().Path, ""))
	})
}

func TestFileTree_HighlightedEntries(t *testing.T) {
	tm := startTree(t, artifactPaths...)

	assert.Assert(t, t.Run("a directory stands for every file beneath it", func(t *testing.T) {
		m := probe(t, tm)
		assert.Check(t, m.HighlightedIsDir())
		assert.Check(t, cmp.DeepEqual(entryPaths(m.HighlightedEntries()), []string{
			"reports/junit/shards/0.xml",
			"reports/junit/shards/1.xml",
			"reports/junit/results.xml",
			"reports/summary.txt",
		}))
	}))

	assert.Assert(t, t.Run("a file stands for itself", func(t *testing.T) {
		send(tm, treeDown, treeDown)
		m := probe(t, tm)
		assert.Check(t, cmp.DeepEqual(entryPaths(m.HighlightedEntries()), []string{"coverage.out"}))
		assert.Check(t, cmp.Equal(len(m.Entries()), len(artifactPaths)))
	}))
}

func TestFileTree_Filter(t *testing.T) {
	tm := startTree(t, artifactPaths...)

	assert.Assert(t, t.Run("/ filters live and recursively, matches listed by path", func(t *testing.T) {
		send(tm, treeSlash)
		assert.Check(t, probe(t, tm).Searching())
		typeText(tm, "xml")
		m := probe(t, tm)
		// results.xml sorts ahead of the shards directory it shares a prefix with.
		assert.Check(t, cmp.DeepEqual(rowLabels(t, m), []string{
			"reports/junit/results.xml",
			"reports/junit/shards/0.xml",
			"reports/junit/shards/1.xml",
		}))
		assert.Check(t, cmp.Contains(frame(t, m), "3 matches"))
	}))

	assert.Assert(t, t.Run("backspace widens the pattern again", func(t *testing.T) {
		send(tm, treeBksp)
		assert.Check(t, cmp.Equal(len(rowLabels(t, probe(t, tm))), 3))
	}))

	assert.Assert(t, t.Run("enter commits: the prompt closes, the filter stays", func(t *testing.T) {
		send(tm, treeEnter)
		m := probe(t, tm)
		assert.Check(t, !m.Searching())
		assert.Check(t, m.FilterActive())
		assert.Check(t, cmp.Equal(len(rowLabels(t, m)), 3))
	}))

	assert.Assert(t, t.Run("a filtered file opens through its flat path", func(t *testing.T) {
		send(tm, treeEnter)
		m := probe(t, tm)
		assert.Check(t, m.Done())
		assert.Check(t, cmp.Equal(m.Selected().Path, "reports/junit/results.xml"))
	}))

	t.Run("clearing restores the full listing from the top", func(t *testing.T) {
		m := probe(t, tm).ClearSelection().ClearFilter()
		assert.Check(t, !m.FilterActive())
		assert.Check(t, cmp.Equal(len(rowLabels(t, m)), 3)) // two dirs + one root file
		assert.Check(t, cmp.Equal(m.Cursor(), 0))
	})
}

func TestFileTree_FilterCancelKeepsCommittedPattern(t *testing.T) {
	tm := startTree(t, artifactPaths...)

	assert.Assert(t, t.Run("a second pattern narrows further", func(t *testing.T) {
		send(tm, treeSlash)
		typeText(tm, "xml")
		send(tm, treeEnter) // commit
		send(tm, treeSlash)
		typeText(tm, "png")
		assert.Check(t, cmp.Equal(len(rowLabels(t, probe(t, tm))), 1))
	}))

	t.Run("esc at the prompt restores the committed pattern", func(t *testing.T) {
		send(tm, treeEsc)
		m := probe(t, tm)
		assert.Check(t, !m.Searching())
		assert.Check(t, m.FilterActive())
		assert.Check(t, cmp.Equal(len(rowLabels(t, m)), 3))
	})
}

func TestFileTree_FilterScopedToCurrentDirectory(t *testing.T) {
	tm := startTree(t, artifactPaths...)

	assert.Assert(t, t.Run("a filter cannot reach outside the current directory", func(t *testing.T) {
		send(tm, treeRight, treeSlash)
		typeText(tm, "n")
		// Directories match on their own relative path too, so the nested shards
		// directory is listed alongside the files under it — and screenshots/, which
		// also contains an "n", is out of scope.
		assert.Check(t, cmp.DeepEqual(rowLabels(t, probe(t, tm)), []string{
			"junit/  (3 files)",
			"junit/shards/  (2 files)",
			"junit/results.xml",
			"junit/shards/0.xml",
			"junit/shards/1.xml",
		}))
	}))

	assert.Assert(t, t.Run("descending a filtered directory crosses every level of its path", func(t *testing.T) {
		// The first enter commits the pattern; the second descends.
		send(tm, treeEnter, treeEnter)
		m := probe(t, tm)
		assert.Check(t, cmp.Equal(m.Dir(), "/reports/junit"))
		assert.Check(t, !m.FilterActive())
		assert.Check(t, cmp.DeepEqual(rowLabels(t, m), []string{"shards/  (2 files)", "results.xml"}))
	}))

	t.Run("← still comes back one level at a time", func(t *testing.T) {
		send(tm, treeLeft)
		assert.Check(t, cmp.Equal(probe(t, tm).Dir(), "/reports"))
	})
}

func TestFileTree_FilterSmartCaseAndNoMatch(t *testing.T) {
	t.Run("a lowercase pattern matches case-insensitively", func(t *testing.T) {
		tm := startTree(t, artifactPaths...)
		send(tm, treeSlash)
		typeText(tm, "junit")
		// The junit and shards directories, plus the three files under them.
		assert.Check(t, cmp.Equal(len(rowLabels(t, probe(t, tm))), 5))
	})

	t.Run("an uppercase letter makes it case-sensitive, and says so when nothing matches", func(t *testing.T) {
		tm := startTree(t, artifactPaths...)
		send(tm, treeSlash)
		typeText(tm, "JUNIT")
		m := probe(t, tm)
		assert.Check(t, cmp.Equal(len(rowLabels(t, m)), 0))
		assert.Check(t, cmp.Contains(frame(t, m), "no files match JUNIT"))
	})

	t.Run("an invalid regex falls back to a literal search", func(t *testing.T) {
		literal := startTree(t, artifactPaths...)
		send(literal, treeSlash)
		typeText(literal, "0.xml")
		assert.Check(t, cmp.Equal(len(rowLabels(t, probe(t, literal))), 1))

		broken := startTree(t, artifactPaths...)
		send(broken, treeSlash)
		typeText(broken, "junit(")
		assert.Check(t, cmp.Equal(len(rowLabels(t, probe(t, broken))), 0))
	})
}

func TestFileTree_Empty(t *testing.T) {
	tm := startTreeModel(t, components.NewFileTree("Artifacts", nil).
		WithEmptyNote("This job produced no artifacts."))

	assert.Assert(t, t.Run("an empty tree explains itself", func(t *testing.T) {
		m := probe(t, tm)
		assert.Check(t, m.Empty())
		assert.Check(t, cmp.Equal(len(rowLabels(t, m)), 0))
		assert.Check(t, cmp.Contains(frame(t, m), "This job produced no artifacts."))
	}))

	t.Run("row keys are inert with no rows to act on", func(t *testing.T) {
		send(tm, treeEnter, treeRight, treeDown, treeLeft)
		m := probe(t, tm)
		assert.Check(t, !m.Done())
		assert.Check(t, cmp.Equal(m.HighlightedLabel(), ""))
		assert.Check(t, cmp.Equal(len(m.HighlightedEntries()), 0))
	})
}

func TestFileTree_PathsAreSanitized(t *testing.T) {
	// Artifact paths come from an API response: absolute, traversing and empty
	// segments must not escape the root or produce phantom rows.
	m := probe(t, startTree(t, "/leading/slash.txt", "../../escape.txt", "./dot/file.txt", "double//slash.txt", ""))

	t.Run("leading, empty and traversing segments are dropped", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(rowLabels(t, m), []string{
			"dot/  (1 file)",
			"double/  (1 file)",
			"leading/  (1 file)",
			"escape.txt",
		}))
	})

	t.Run("the unusable path contributes no entry", func(t *testing.T) {
		assert.Check(t, cmp.Equal(len(m.Entries()), 4))
	})
}

func TestFileTree_DuplicatePathKeepsFirst(t *testing.T) {
	tm := startTreeModel(t, components.NewFileTree("Artifacts", []components.FileTreeEntry{
		{Path: "a/b.txt", Ref: "first"},
		{Path: "a/b.txt", Ref: "second"},
	}))

	send(tm, treeEnter, treeEnter)
	m := probe(t, tm)
	assert.Check(t, m.Done())
	assert.Check(t, cmp.Equal(m.Selected().Ref, "first"))
}

func TestFileTree_NoteAndScrolling(t *testing.T) {
	// The 80×24 terminal cannot show 40 rows, so the picker windows them.
	m := probe(t, startTreeModel(t, components.NewFileTree("Artifacts", numberedEntries(40)).
		WithNote("✓ Downloaded 40 files")))
	view := frame(t, m)

	t.Run("the host's note renders above the rows", func(t *testing.T) {
		assert.Check(t, cmp.Contains(view, "✓ Downloaded 40 files"))
	})

	t.Run("an over-long listing is windowed, with its position shown", func(t *testing.T) {
		assert.Check(t, cmp.Contains(view, "of 40)"))
		assert.Check(t, len(rowLabels(t, m)) < 40, "expected a windowed listing, got every row")
	})
}

func TestFileTree_ResizeReflowsTheWindow(t *testing.T) {
	tm := startTreeModel(t, components.NewFileTree("Artifacts", numberedEntries(40)))
	tall := len(rowLabels(t, probe(t, tm)))

	t.Run("a shorter terminal shows fewer rows", func(t *testing.T) {
		tm.Send(tea.WindowSizeMsg{Width: 80, Height: 10})
		m := probe(t, tm)
		short := len(rowLabels(t, m))
		assert.Check(t, short < tall, "expected fewer rows at height 10: %d vs %d", short, tall)
		assert.Check(t, cmp.Contains(frame(t, m), "of 40)"))
	})
}

// numberedEntries builds n flat entries, for the windowing tests.
func numberedEntries(n int) []components.FileTreeEntry {
	entries := make([]components.FileTreeEntry, n)
	for i := range entries {
		path := "log-" + itoa(i) + ".txt"
		entries[i] = components.FileTreeEntry{Path: path, Ref: path}
	}
	return entries
}

func entryPaths(entries []components.FileTreeEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}
	return out
}

func itoa(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
