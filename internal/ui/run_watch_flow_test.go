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
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// watchRunID is the run every watch test watches. It is fixed so assertions can
// name it.
var watchRunID = uuid.MustParse("f0000000-0000-4000-8000-000000000001")

// watchHarness drives a RunWatchFlowModel as a teatest program, answering
// snapshots and quit requests from inside the loop like flowHarness does for the
// run-get flow. Unlike that flow this one ends itself when the run ends, so the
// harness also exposes the inner model's result for the final-state assertions.
type watchHarness struct {
	m ui.RunWatchFlowModel
}

func (h watchHarness) Init() tea.Cmd { return h.m.Init() }

func (h watchHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case quitMsg:
		return h, tea.Quit
	case snapshotMsg:
		msg.frame <- h.m.View().Content
		return h, nil
	}
	u, cmd := h.m.Update(msg)
	h.m = u.(ui.RunWatchFlowModel)
	return h, cmd
}

func (h watchHarness) View() tea.View { return h.m.View() }

func (h watchHarness) result() ui.RunWatchResult { return h.m.Result() }

// startWatch runs a watch flow at a known terminal size. Unset options get test
// defaults: the fixed run ID, and a poll interval short enough that a test never
// waits on the real five-second cadence.
func startWatch(t *testing.T, opts ui.RunWatchFlowOptions) *teatest.TestModel {
	t.Helper()
	if opts.RunID == uuid.Nil {
		opts.RunID = watchRunID
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 10 * time.Millisecond
	}
	tm := teatest.NewTestModel(t, watchHarness{m: ui.NewRunWatchFlow(context.Background(), opts)},
		teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() {
		tm.Send(quitMsg{})
		tm.WaitFinished(t, teatest.WithFinalTimeout(teaTimeout))
	})
	return tm
}

// watchResult waits for the flow to end itself and returns its outcome.
func watchResult(t *testing.T, tm *teatest.TestModel) ui.RunWatchResult {
	t.Helper()
	tm.WaitFinished(t, teatest.WithFinalTimeout(teaTimeout))
	return tm.FinalModel(t).(watchHarness).result()
}

// runningState is a run mid-flight: one workflow with a finished job and a
// running one.
func runningState() ui.RunWatchState {
	return ui.RunWatchState{Workflows: []ui.RunWatchWorkflow{{
		Name:   "build-and-test",
		Symbol: "●",
		Status: "running",
		Jobs: []ui.RunWatchJob{
			{ID: uuid.New(), Name: "lint", Symbol: "✓", Status: "succeeded", Type: "build"},
			{ID: uuid.New(), Name: "unit-test", Symbol: "●", Status: "running", Type: "build"},
		},
	}}}
}

// doneState is the same run, finished and green.
func doneState() ui.RunWatchState {
	return ui.RunWatchState{
		Done:    true,
		Outcome: "succeeded",
		Workflows: []ui.RunWatchWorkflow{{
			Name:     "build-and-test",
			Symbol:   "✓",
			Status:   "succeeded",
			Duration: "1m2s",
			Jobs: []ui.RunWatchJob{
				{ID: uuid.New(), Name: "lint", Symbol: "✓", Status: "succeeded", Type: "build"},
				{ID: uuid.New(), Name: "test", Symbol: "✓", Status: "succeeded", Type: "build"},
			},
		}},
	}
}

// failedJobState is a run still going with one job already failed — what
// --failfast trips on.
func failedJobState() ui.RunWatchState {
	failedID := uuid.MustParse("d0000000-0000-4000-8000-00000000f001")
	return ui.RunWatchState{Workflows: []ui.RunWatchWorkflow{{
		Name:   "build-and-test",
		Symbol: "●",
		Status: "running",
		Jobs: []ui.RunWatchJob{
			{ID: failedID, Name: "test", Symbol: "✗", Status: "failed", Type: "build", Failed: true},
			{ID: uuid.New(), Name: "deploy", Symbol: "○", Status: "queued", Type: "build"},
		},
	}}}
}

// fetchStatic returns a Fetch that always answers with the same state, and a
// counter of how many times it was called. The counter is atomic: Fetch runs on
// the program's command goroutine, not the test's.
func fetchStatic(state ui.RunWatchState) (func(context.Context) (ui.RunWatchState, error), *atomic.Int32) {
	var calls atomic.Int32
	return func(context.Context) (ui.RunWatchState, error) {
		calls.Add(1)
		return state, nil
	}, &calls
}

// TestRunWatchFlow_Header confirms the run being watched is named in the header,
// with its branch as a bracketed scope — the picker titles' convention.
func TestRunWatchFlow_Header(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})

	waitForOutput(t, tm, "Watching run")
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "Watching run "+watchRunID.String()+" [main]"))
}

// TestRunWatchFlow_Table confirms every workflow and its jobs are drawn, each
// with a status glyph and word, and that a finished workflow shows its duration.
func TestRunWatchFlow_Table(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})

	waitForOutput(t, tm, "build-and-test")
	view := flowSnapshot(t, tm)

	t.Run("workflow row", func(t *testing.T) {
		assert.Check(t, cmp.Contains(view, "build-and-test"))
		assert.Check(t, cmp.Contains(view, "● running"))
	})

	t.Run("job rows", func(t *testing.T) {
		assert.Check(t, cmp.Contains(view, "lint"))
		assert.Check(t, cmp.Contains(view, "✓ succeeded"))
		assert.Check(t, cmp.Contains(view, "unit-test"))
	})

	t.Run("status glyphs share one column at every level", func(t *testing.T) {
		workflow := watchRow(t, view, "build-and-test")
		lint := watchRow(t, view, "lint")
		test := watchRow(t, view, "unit-test")

		assert.Check(t, cmp.Equal(glyphColumn(t, lint, "✓"), glyphColumn(t, test, "●")),
			"job status glyphs must share a column:\n%q\n%q", lint, test)
		assert.Check(t, cmp.Equal(glyphColumn(t, workflow, "●"), glyphColumn(t, lint, "✓")),
			"a workflow's status must sit in the same column as its jobs':\n%q\n%q", workflow, lint)
	})
}

// TestRunWatchFlow_PendingNote confirms the table says why it is still being
// watched once every workflow has ended — the state a dynamic-config run sits in
// between its setup workflow ending and the continued workflow appearing, where
// nothing on screen moves again until the run itself ends.
func TestRunWatchFlow_PendingNote(t *testing.T) {
	t.Run("shown while the run outlives its workflows", func(t *testing.T) {
		state := doneState()
		state.Done = false
		state.AllWorkflowsEnded = true
		fetch, _ := fetchStatic(state)
		tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})

		waitForOutput(t, tm, "All workflows have ended")
		assert.Check(t, cmp.Contains(flowSnapshot(t, tm),
			"All workflows have ended; waiting for the run itself to finish."))
	})

	t.Run("absent while a workflow is still going", func(t *testing.T) {
		fetch, _ := fetchStatic(runningState())
		tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})

		waitForOutput(t, tm, "build-and-test")
		assert.Check(t, !strings.Contains(flowSnapshot(t, tm), "All workflows have ended"))
	})
}

// TestRunWatchFlow_Footer confirms the footer reports elapsed time, when the next
// poll lands, and the two live keys. The countdown is what makes the table read
// as live between polls.
func TestRunWatchFlow_Footer(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{
		Branch: "main", Fetch: fetch,
		// Long enough that the countdown is still on screen when we look.
		PollInterval: 30 * time.Second,
	})

	waitForOutput(t, tm, "Elapsed")
	view := flowSnapshot(t, tm)

	assert.Check(t, cmp.Contains(view, "Elapsed "))
	assert.Check(t, cmp.Contains(view, "r refresh"))
	assert.Check(t, cmp.Contains(view, "q quit"))

	t.Run("the wait for the next poll is drawn as a meter and a figure", func(t *testing.T) {
		// The whole interval is still ahead, so every cell is empty.
		assert.Check(t, cmp.Contains(view, "next update ▱▱▱▱▱▱ "))
		assert.Check(t, cmp.Contains(view, "▱ 30s"))
	})
}

// TestRunWatchFlow_PollMeterFills confirms the meter fills as the wait elapses,
// which is the only thing that distinguishes it from a static decoration.
func TestRunWatchFlow_PollMeterFills(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	// Six cells over a three-second wait: the first clock tick is already worth
	// two of them, so the wait for a filled cell is a tick, not a fixed pause.
	tm := startWatch(t, ui.RunWatchFlowOptions{
		Branch: "main", Fetch: fetch, PollInterval: 3 * time.Second,
	})

	waitForOutput(t, tm, "next update")
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "next update ▱▱▱▱▱▱"),
		"the meter starts empty")

	// The footer is redrawn in place, so a filled cell reaches the output stream
	// as a cursor move and a few cells — "next update ▰" only ever lands there
	// contiguously if the renderer happens to repaint the whole line on the same
	// frame. Wait on the model's frames instead.
	waitForFrame(t, tm, "next update ▰")
}

// TestRunWatchFlow_FirstWaitIsTheBaseInterval confirms the backoff is applied
// when a wait is scheduled rather than while it runs. It used to be applied
// first, so the very first countdown started at twice the base interval — the
// display and the schedule agreed with each other, and both were wrong.
//
// The countdown's rounding rule and the meter's fill are pinned as pure
// functions in run_watch_timing_test.go; both are defined below the resolution
// this program loop can be observed at.
func TestRunWatchFlow_FirstWaitIsTheBaseInterval(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{
		Branch: "main", Fetch: fetch, PollInterval: 20 * time.Second,
	})

	waitForOutput(t, tm, "next update")
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "next update ▱▱▱▱▱▱ 20s"))
}

// TestRunWatchFlow_ColorsStatusGlyphs confirms each status glyph carries the
// shared status palette — the same mapping the run-get pickers use, so a green
// tick means the same thing wherever it appears.
func TestRunWatchFlow_ColorsStatusGlyphs(t *testing.T) {
	state := ui.RunWatchState{Workflows: []ui.RunWatchWorkflow{{
		Name: "build", Symbol: "●", Status: "running",
		Jobs: []ui.RunWatchJob{
			{Name: "lint", Symbol: "✓", Status: "succeeded"},
			{Name: "test", Symbol: "✗", Status: "failed", Failed: true},
			{Name: "warn", Symbol: "!", Status: "errored"},
		},
	}}}
	fetch, _ := fetchStatic(state)

	t.Run("with color", func(t *testing.T) {
		tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Color: true, Fetch: fetch})
		waitForOutput(t, tm, "build")
		raw := flowFrame(t, tm)

		for _, tc := range []struct{ glyph, code, name string }{
			{"✓", "42", "green"},
			{"✗", "196", "red"},
			{"!", "220", "yellow"},
			{"●", "39", "blue"},
		} {
			assert.Check(t, cmp.Contains(raw, "38;5;"+tc.code+"m"+tc.glyph),
				"%s should render %s (256-color %s)", tc.glyph, tc.name, tc.code)
		}
	})

	t.Run("without color", func(t *testing.T) {
		tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})
		waitForOutput(t, tm, "build")
		raw := flowFrame(t, tm)

		assert.Check(t, cmp.Contains(raw, "✓"))
		assert.Check(t, !strings.Contains(raw, "38;5;42"), "no color means no SGR sequences: %q", raw)
	})
}

// TestRunWatchFlow_PollsUntilTheRunEnds confirms the flow keeps polling on its
// own and ends the moment a poll reports every workflow finished, carrying the
// run's outcome out in the result.
func TestRunWatchFlow_PollsUntilTheRunEnds(t *testing.T) {
	var calls atomic.Int32
	fetch := func(context.Context) (ui.RunWatchState, error) {
		if calls.Add(1) < 3 {
			return runningState(), nil
		}
		return doneState(), nil
	}

	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})
	res := watchResult(t, tm)

	assert.Check(t, cmp.Equal(res.State.Outcome, "succeeded"))
	assert.Check(t, res.State.Done)
	assert.Check(t, !res.Cancelled)
	assert.Check(t, !res.TimedOut)
	assert.Check(t, res.Err == nil)
	assert.Check(t, calls.Load() >= 3, "the flow polled %d times without being asked to", calls.Load())
}

// TestRunWatchFlow_RefreshPollsAgain confirms "r" fetches immediately rather than
// waiting out the interval.
func TestRunWatchFlow_RefreshPollsAgain(t *testing.T) {
	fetch, calls := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{
		Branch: "main", Fetch: fetch,
		// Long enough that a second poll can only be the one "r" asked for.
		PollInterval: time.Hour,
	})

	waitForOutput(t, tm, "next update")
	assert.Assert(t, cmp.Equal(calls.Load(), int32(1)))

	tm.Send(keyR)
	waitForFetches(t, calls, 2)
}

// TestRunWatchFlow_FailFastEndsOnAFailedJob confirms --failfast stops the watch
// while the run is still going, and hands back the failed job so the caller can
// name it.
func TestRunWatchFlow_FailFastEndsOnAFailedJob(t *testing.T) {
	fetch, _ := fetchStatic(failedJobState())
	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", FailFast: true, Fetch: fetch})

	res := watchResult(t, tm)

	assert.Check(t, res.FailFast)
	assert.Check(t, !res.State.Done, "the run had not finished when the watch gave up on it")
	assert.Assert(t, cmp.Len(res.State.FailedJobs(), 1))
	assert.Check(t, cmp.Equal(res.State.FailedJobs()[0].Name, "test"))
}

// TestRunWatchFlow_FailFastOffKeepsWatching confirms a failed job alone does not
// end the watch: without --failfast the run is followed to its own conclusion.
func TestRunWatchFlow_FailFastOffKeepsWatching(t *testing.T) {
	fetch, calls := fetchStatic(failedJobState())
	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})

	waitForFetches(t, calls, 2)
	assert.Check(t, cmp.Contains(flowSnapshot(t, tm), "✗ failed"))
}

// TestRunWatchFlow_TimeoutEndsTheWatch confirms the timeout is enforced by the
// flow itself, and reported as a timeout rather than a failure.
func TestRunWatchFlow_TimeoutEndsTheWatch(t *testing.T) {
	fetch, _ := fetchStatic(runningState())
	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Timeout: time.Nanosecond, Fetch: fetch})

	res := watchResult(t, tm)

	assert.Check(t, res.TimedOut)
	assert.Check(t, !res.Cancelled)
	assert.Check(t, res.Err == nil)
}

// TestRunWatchFlow_QuitCancels confirms each of the stop keys ends the watch as a
// cancellation — the run is still going, so the caller must not report success.
func TestRunWatchFlow_QuitCancels(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"q", keyQ},
		{"esc", keyEsc},
		{"ctrl+c", tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetch, _ := fetchStatic(runningState())
			tm := startWatch(t, ui.RunWatchFlowOptions{
				Branch: "main", Fetch: fetch, PollInterval: time.Hour,
			})
			waitForOutput(t, tm, "next update")

			tm.Send(tc.key)
			res := watchResult(t, tm)

			assert.Check(t, res.Cancelled)
			assert.Check(t, !res.TimedOut)
			assert.Check(t, res.Err == nil)
		})
	}
}

// TestRunWatchFlow_FetchErrorEndsTheWatch confirms a failing poll ends the flow
// and carries the error out, rather than spinning against an API that has
// stopped answering.
func TestRunWatchFlow_FetchErrorEndsTheWatch(t *testing.T) {
	boom := errors.New("token expired")
	fetch := func(context.Context) (ui.RunWatchState, error) {
		return ui.RunWatchState{}, boom
	}

	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})
	res := watchResult(t, tm)

	assert.Check(t, cmp.ErrorIs(res.Err, boom))
	assert.Check(t, !res.Cancelled)
}

// TestRunWatchFlow_CancelledContextIsNotAnError confirms a cancelled context —
// what a ctrl+c the program never saw as a keystroke looks like from inside a
// fetch — is reported as a cancellation, not an API failure.
func TestRunWatchFlow_CancelledContextIsNotAnError(t *testing.T) {
	fetch := func(context.Context) (ui.RunWatchState, error) {
		return ui.RunWatchState{}, context.Canceled
	}

	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})
	res := watchResult(t, tm)

	assert.Check(t, res.Cancelled)
	assert.Check(t, res.Err == nil)
}

// TestRunWatchFlow_TruncatesLongNames confirms a job name longer than its column
// is cut rather than left to push the status column out of alignment.
func TestRunWatchFlow_TruncatesLongNames(t *testing.T) {
	long := "integration-test-with-a-very-long-name-indeed"
	state := ui.RunWatchState{Workflows: []ui.RunWatchWorkflow{{
		Name: "build", Symbol: "●", Status: "running",
		Jobs: []ui.RunWatchJob{
			{Name: long, Symbol: "●", Status: "running", Type: "build"},
			{Name: "lint", Symbol: "✓", Status: "succeeded", Type: "build"},
		},
	}}}
	fetch, _ := fetchStatic(state)

	tm := startWatch(t, ui.RunWatchFlowOptions{Branch: "main", Fetch: fetch})
	waitForOutput(t, tm, "integration-test")
	view := flowSnapshot(t, tm)

	assert.Check(t, !strings.Contains(view, long), "an over-long name should be truncated")
	assert.Check(t, cmp.Contains(view, "…"))
	assert.Check(t, cmp.Equal(
		glyphColumn(t, watchRow(t, view, "integration-test"), "●"),
		glyphColumn(t, watchRow(t, view, "lint"), "✓"),
	), "a truncated name must not move the status column")
}

// watchRow returns the (ANSI-stripped) line of a frame containing substr.
func watchRow(t *testing.T, view, substr string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no row containing %q in:\n%s", substr, view)
	return ""
}

// glyphColumn is the display column (counted in cells, not bytes) where glyph
// starts in row. Byte offsets cannot answer an alignment question: a truncated
// name ends in a three-byte ellipsis, which shifts every index after it.
func glyphColumn(t *testing.T, row, glyph string) int {
	t.Helper()
	i := strings.Index(row, glyph)
	assert.Assert(t, i >= 0, "row %q has no %q", row, glyph)
	return ansi.StringWidth(row[:i])
}

// waitForFetches blocks until the flow has polled at least n times. The poll
// count is the only observable for a fetch that changes nothing on screen, so
// this is the one wait teatest.WaitFor cannot serve. It polls for the condition
// the way WaitFor does rather than pausing for a fixed spell: it returns the
// moment the count is reached, and teaTimeout is a liveness ceiling.
func waitForFetches(t *testing.T, calls *atomic.Int32, n int32) {
	t.Helper()
	deadline := time.Now().Add(teaTimeout)
	for calls.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d polls happened within %s", calls.Load(), n, teaTimeout)
		}
		time.Sleep(time.Millisecond)
	}
}
