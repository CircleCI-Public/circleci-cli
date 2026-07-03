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

package termrender

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

var benchFixtures = []struct {
	name string
	in   string
}{
	{"plain/1000lines", buildPlainLog(1000)},
	{"progress/200bars", buildProgressLog(200, 20)},
	{"color/1000lines", buildColorLog(1000)},
}

func BenchmarkRender(b *testing.B) {
	for _, f := range benchFixtures {
		b.Run(f.name, func(b *testing.B) {
			src := []byte(f.in)

			var buf bytes.Buffer
			buf.Grow(len(src))
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()

			for b.Loop() {
				buf.Reset()

				err := Render(&buf, bytes.NewReader(src))
				assert.NilError(b, err)
			}
		})
	}
}

func BenchmarkRenderStyled(b *testing.B) {
	for _, f := range benchFixtures {
		b.Run(f.name, func(b *testing.B) {
			src := []byte(f.in)

			var buf bytes.Buffer
			buf.Grow(len(src))
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()

			for b.Loop() {
				buf.Reset()

				err := RenderStyled(&buf, bytes.NewReader(src))
				assert.NilError(b, err)
			}
		})
	}
}

// Fixtures are built deterministically (fixed sizes, no randomness) so results
// are comparable across runs. Each benchmark runs a fixture through both Render
// (plain) and RenderStyled (color-preserving) so the styled overhead — the
// per-cell pen tracking and SGR re-serialization — is directly visible, and so
// the cost of collapsing carriage-return redraws (the pager's hot path) is
// isolated from plain line throughput.

// buildPlainLog returns n lines of plain ~80-column text with no escapes — the
// throughput baseline.
func buildPlainLog(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		_, _ = fmt.Fprintf(&sb, "line %6d: the quick brown fox jumps over the lazy dog and then some\n", i)
	}
	return sb.String()
}

// buildProgressLog returns bars progress bars, each redrawn ticks times in place
// with a carriage return before settling to a final line — the apt/Docker
// redraw pattern the renderer exists to collapse.
func buildProgressLog(bars, ticks int) string {
	var sb strings.Builder
	for b := 0; b < bars; b++ {
		sb.WriteString("0% [Working]")
		for t := 1; t <= ticks; t++ {
			_, _ = fmt.Fprintf(&sb, "\rGet:%d downloading %d%%", b, t*100/ticks)
		}
		_, _ = fmt.Fprintf(&sb, "\rHit:%d archive InRelease [%d kB]\n", b, b*7%900)
	}
	return sb.String()
}

// buildColorLog returns n lines each wrapped in an SGR color that cycles, so the
// styled path exercises real pen changes (and the plain path exercises skipping
// them).
func buildColorLog(n int) string {
	colors := []int{31, 32, 33, 34, 35, 36, 90, 92}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		_, _ = fmt.Fprintf(&sb, "\x1b[%dmGet:%d built target //pkg/component:lib in %dms\x1b[0m\n",
			colors[i%len(colors)], i, i%1000)
	}
	return sb.String()
}
