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

package run

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestFailedJobLogSuggestions verifies the suggestion attached to a failed run
// names a job the user can actually fetch: the UUID is interpolated from the
// state already in memory, so no lookup step is left to the reader.
func TestFailedJobLogSuggestions(t *testing.T) {
	jobID := uuid.MustParse("d0000000-0000-4000-8000-00000000f001")
	otherID := uuid.MustParse("d0000000-0000-4000-8000-00000000f003")

	t.Run("interpolates the failed job's id", func(t *testing.T) {
		state := runGetOutput{Workflows: []workflowOutput{{
			Name: "build",
			Jobs: []jobOutput{
				{ID: uuid.MustParse("d0000000-0000-4000-8000-00000000f002"), Name: "lint", Outcome: "succeeded"},
				{ID: jobID, Name: "test-service", Outcome: "failed"},
			},
		}}}

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get ` + jobID.String(),
		}))
	})

	t.Run("one suggestion per failed job across workflows", func(t *testing.T) {
		state := runGetOutput{Workflows: []workflowOutput{
			{Name: "build", Jobs: []jobOutput{{ID: jobID, Name: "test-service", Outcome: "failed"}}},
			{Name: "deploy", Jobs: []jobOutput{{ID: otherID, Name: "publish", Outcome: "failed"}}},
		}}

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get ` + jobID.String(),
			`View logs for failed job "publish": circleci job get ` + otherID.String(),
		}))
	})

	t.Run("falls back to the placeholder for a job with no id", func(t *testing.T) {
		state := runGetOutput{Workflows: []workflowOutput{{
			Name: "build",
			Jobs: []jobOutput{{Name: "test-service", Outcome: "failed"}},
		}}}

		assert.Check(t, is.DeepEqual(failedJobLogSuggestions(state), []string{
			`View logs for failed job "test-service": circleci job get <job-id>`,
		}))
	})

	t.Run("no suggestions when nothing failed", func(t *testing.T) {
		state := runGetOutput{Workflows: []workflowOutput{{
			Name: "build",
			Jobs: []jobOutput{{ID: jobID, Name: "test-service", Outcome: "succeeded"}},
		}}}

		assert.Check(t, is.Len(failedJobLogSuggestions(state), 0))
	})
}

// TestApplyWatchColor_NoColor verifies that all symbols are returned unchanged
// when colour output is disabled.
func TestApplyWatchColor_NoColor(t *testing.T) {
	for _, sym := range []string{"✓", "✗", "●", "○", "⊘", "!", "•", "unknown"} {
		t.Run(sym, func(t *testing.T) {
			assert.Check(t, is.Equal(applyWatchColor(false, sym), sym))
		})
	}
}

// TestApplyWatchColor_SuccessIsGreen verifies ✓ gets the green (colour 42) style.
func TestApplyWatchColor_SuccessIsGreen(t *testing.T) {
	got := applyWatchColor(true, "✓")
	assert.Check(t, is.Contains(got, "✓"), "rendered string must contain the glyph")
	assert.Check(t, strings.Contains(got, "42"),
		"success symbol should use green (256-colour 42), got: %q", got)
	assert.Check(t, got != "✓", "coloured output should differ from the raw symbol")
}

// TestApplyWatchColor_FailureIsRed verifies ✗ gets the red (colour 196) style.
func TestApplyWatchColor_FailureIsRed(t *testing.T) {
	got := applyWatchColor(true, "✗")
	assert.Check(t, is.Contains(got, "✗"), "rendered string must contain the glyph")
	assert.Check(t, strings.Contains(got, "196"),
		"failure symbol should use red (256-colour 196), got: %q", got)
}

// TestApplyWatchColor_YellowSymbols verifies that in-progress / pending /
// canceled symbols all get the yellow (colour 220) style.
func TestApplyWatchColor_YellowSymbols(t *testing.T) {
	for _, sym := range []string{"●", "○", "⊘", "!"} {
		t.Run(sym, func(t *testing.T) {
			got := applyWatchColor(true, sym)
			assert.Check(t, is.Contains(got, sym), "rendered string must contain the glyph")
			assert.Check(t, strings.Contains(got, "220"),
				"symbol %q should use yellow (256-colour 220), got: %q", sym, got)
		})
	}
}

// TestApplyWatchColor_PassthroughUnknown verifies that symbols without a
// specific colour mapping are returned unchanged even when colour is enabled.
func TestApplyWatchColor_PassthroughUnknown(t *testing.T) {
	assert.Check(t, is.Equal(applyWatchColor(true, "•"), "•"))
	assert.Check(t, is.Equal(applyWatchColor(true, "?"), "?"))
}
