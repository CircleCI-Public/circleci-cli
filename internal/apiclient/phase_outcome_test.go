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

package apiclient_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestPhaseOutcome_NotRun verifies the "not_run" terminal outcome (a run that
// never executed, e.g. its config could not be fetched/compiled) renders a
// no-entry glyph and readable wording rather than a bare bullet and the raw
// "not_run". It is a no-entry, not a warning, since nothing actually ran.
func TestPhaseOutcome_NotRun(t *testing.T) {
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeSymbol("ended", "", "not_run"), "⊘"))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeText("ended", "", "not_run"), "not run"))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeStatus("ended", "", "not_run"), ":no_entry_sign: not run"))
}

// TestPhaseOutcome_StatusIsTextWithEmoji checks the emoji and plain-text status
// helpers stay in lockstep: PhaseOutcomeStatus is PhaseOutcomeText with a status
// emoji prefixed (or exactly the text when there is no emoji, e.g. an unknown
// outcome that passes through undecorated).
func TestPhaseOutcome_StatusIsTextWithEmoji(t *testing.T) {
	cases := []struct{ phase, outcome, current string }{
		{"created", "", ""},
		{"queued", "", ""},
		{"started", "", ""},
		{"started", "", "failed"},
		{"ended", "succeeded", ""},
		{"ended", "", "failed"},
		{"ended", "", "not_run"},
		{"ended", "", "some_new_outcome"}, // unknown → undecorated
		{"weird_phase", "", ""},           // unknown phase → undecorated
	}
	for _, c := range cases {
		status := apiclient.PhaseOutcomeStatus(c.phase, c.outcome, c.current)
		text := apiclient.PhaseOutcomeText(c.phase, c.outcome, c.current)
		assert.Check(t, status == text || strings.HasSuffix(status, " "+text),
			"status %q should be text %q, optionally emoji-prefixed", status, text)
	}
}
