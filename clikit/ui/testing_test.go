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
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"
)

// teaTimeout bounds every wait on a teatest program in this package. It is a
// liveness ceiling, not a latency budget: each wait returns the moment its
// condition is met, so raising it cannot slow a passing test — but a wait that
// is too tight fails a healthy program whenever the machine stalls. CI runs the
// whole tree with -race while the acceptance suite compiles and execs binaries
// on the same cores, and a 2s ceiling was tight enough to lose that race — on a
// step that takes ~10ms even with the CPUs saturated.
const teaTimeout = 10 * time.Second

// quitMsg tells a test harness to end the program. The models under test ignore
// unknown message types, so sending it does not perturb their state.
type quitMsg struct{}

// waitForOutput blocks until the program's cumulative output contains s. It
// returns as soon as the substring appears, so fast assertions stay fast.
func waitForOutput(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(s))
	}, teatest.WithDuration(teaTimeout))
}
