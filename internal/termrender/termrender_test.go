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
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func render(t *testing.T, in string) string {
	t.Helper()
	var buf bytes.Buffer
	assert.NilError(t, Render(&buf, strings.NewReader(in)))
	return buf.String()
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "plain lines lf",
			in:   "alpha\nbeta\ngamma\n",
			want: "alpha\nbeta\ngamma\n",
		},
		{
			name: "crlf line endings",
			in:   "alpha\r\nbeta\r\ngamma\r\n",
			want: "alpha\nbeta\ngamma\n",
		},
		{
			name: "no trailing newline still emitted",
			in:   "alpha\nbeta",
			want: "alpha\nbeta\n",
		},
		{
			name: "sgr color is stripped",
			in:   "\x1b[38;2;1;2;3mred\x1b[0m text\n",
			want: "red text\n",
		},
		{
			name: "trailing blank lines dropped",
			in:   "alpha\n\n\n\n",
			want: "alpha\n",
		},
		{
			name: "interior blank lines kept",
			in:   "alpha\n\n\nbeta\n",
			want: "alpha\n\n\nbeta\n",
		},
		{
			name: "trailing whitespace on a line trimmed",
			in:   "alpha    \nbeta\n",
			want: "alpha\nbeta\n",
		},
		{
			name: "long line is never wrapped",
			in:   strings.Repeat("x", 5000) + "\n",
			want: strings.Repeat("x", 5000) + "\n",
		},
		{
			name: "carriage return progress collapses to final",
			in:   "Progress 10%\rProgress 55%\rProgress 100%\n",
			want: "Progress 100%\n",
		},
		{
			name: "carriage return leaves stale tail without erase",
			// "Done" overwrites the first four cells; the rest of the longer
			// previous content stays, exactly as a real terminal would show it.
			in:   "Processing...\rDone\n",
			want: "Doneessing...\n",
		},
		{
			name: "carriage return then erase-to-end clears tail",
			in:   "Processing...\rDone\x1b[K\n",
			want: "Done\n",
		},
		{
			name: "backspace moves cursor back",
			in:   "abcX\b\bYZ\n",
			want: "abYZ\n",
		},
		{
			name: "tab advances to 8-column stops",
			in:   "a\tb\tc\n",
			want: "a       b       c\n",
		},
		{
			name: "cursor up rewrites earlier line (docker style)",
			// Print three layer lines, jump up three, rewrite the first two.
			in:   "layer1: pull\nlayer2: pull\nlayer3: pull\n\x1b[3A\rlayer1: done\x1b[K\n\rlayer2: done\x1b[K\n",
			want: "layer1: done\nlayer2: done\nlayer3: pull\n",
		},
		{
			name: "cursor position addressing",
			in:   "....\n....\n\x1b[1;1HAB\x1b[2;3HCD\n",
			want: "AB..\n..CD\n",
		},
		{
			name: "erase whole screen collapses repaint to final frame",
			in:   "frame one\nstuff\x1b[2J\x1b[Hframe two\n",
			want: "frame two\n",
		},
		{
			name: "erase line start to cursor is inclusive of the cursor cell",
			// Write HELLOWORLD, return to column 0, move to column 5, then erase
			// the start of the line through the cursor (columns 0..5).
			in:   "HELLOWORLD\r\x1b[5C\x1b[1K\n",
			want: "      ORLD\n",
		},
		{
			name: "repeat previous character",
			in:   "-\x1b[9b\n",
			want: "----------\n",
		},
		{
			name: "device status query is ignored not printed",
			in:   "before\x1b[6nafter\n",
			want: "beforeafter\n",
		},
		{
			name: "osc title is ignored",
			in:   "x\x1b]0;my title\x07y\n",
			want: "xy\n",
		},
		{
			// Reply-eliciting queries a real terminal would answer (goreleaser
			// emits an OSC 11 background-color query, a Device Attributes request,
			// and a DECRQM mode query). We have no input channel, so they are
			// simply consumed — they must never leak into the rendered text.
			name: "reply-eliciting queries are consumed not printed",
			in:   "building\x1b]11;?\x07 packages\x1b[c done\x1b[?2026$p\n",
			want: "building packages done\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(render(t, tc.in), tc.want))
		})
	}
}

// TestRenderScrollsBeyondWindowWithoutLoss feeds far more lines than the live
// window holds and verifies every line survives, in order — the property the
// previous emulator-with-bounded-scrollback approach violated.
func TestRenderScrollsBeyondWindowWithoutLoss(t *testing.T) {
	const n = DefaultHeight*5 + 37
	var in strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&in, "line %d\n", i)
	}

	lines := strings.Split(strings.TrimSuffix(render(t, in.String()), "\n"), "\n")

	assert.Check(t, cmp.Len(lines, n))
	for i, l := range lines {
		assert.Check(t, cmp.Equal(l, fmt.Sprintf("line %d", i)))
	}
}

// TestRenderProgressBarThenScroll mixes a redraw with enough following output
// to scroll the collapsed line out of the window, confirming the collapse
// survives scrolling and no intermediate state leaks.
func TestRenderProgressBarThenScroll(t *testing.T) {
	var in strings.Builder
	in.WriteString("download 0%\rdownload 50%\rdownload 100%\n")
	for i := 0; i < DefaultHeight+10; i++ {
		fmt.Fprintf(&in, "after %d\n", i)
	}

	got := render(t, in.String())

	assert.Check(t, cmp.Equal(strings.HasPrefix(got, "download 100%\n"), true))
	assert.Check(t, !strings.Contains(got, "download 0%"))
	assert.Check(t, !strings.Contains(got, "download 50%"))
}

// TestRenderHugeCursorParamsDoNotBlowUp guards against an OOM/hang where a
// cursor-positioning or repeat sequence with an enormous parameter forced a
// multi-gigabyte blank-padded row. Each case must return promptly and produce a
// row no larger than the column cap.
func TestRenderHugeCursorParamsDoNotBlowUp(t *testing.T) {
	cases := map[string]string{
		"cursor forward":  "x\x1b[999999999Cy\n",
		"cursor position": "\x1b[1;999999999Hy\n",
		"horizontal abs":  "\x1b[999999999Gy\n",
		"repeat":          "x\x1b[999999999b\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			done := make(chan string, 1)
			go func() {
				var buf bytes.Buffer
				assert.Check(t, Render(&buf, strings.NewReader(in)) == nil)
				done <- buf.String()
			}()
			select {
			case got := <-done:
				// The cursor caps at maxColumns; printing one rune there yields at
				// most maxColumns+1 cells — bounded, not gigabytes.
				line := strings.TrimSuffix(got, "\n")
				assert.Check(t, len([]rune(line)) <= maxColumns+1,
					"rendered line length %d exceeds cap %d", len([]rune(line)), maxColumns)
			case <-time.After(10 * time.Second):
				t.Fatal("Render did not return: a huge cursor parameter ballooned a row")
			}
		})
	}
}

// readOnly hides any io.ByteReader implementation so Render must fall back to
// wrapping the source in a bufio.Reader.
type readOnly struct{ r io.Reader }

func (ro readOnly) Read(p []byte) (int, error) { return ro.r.Read(p) }

func TestRenderFromNonByteReader(t *testing.T) {
	var buf bytes.Buffer
	assert.NilError(t, Render(&buf, readOnly{strings.NewReader("alpha\r\nbeta\n")}))
	assert.Check(t, cmp.Equal(buf.String(), "alpha\nbeta\n"))
}

// errReader yields some bytes and then a non-EOF error.
type errReader struct {
	data []byte
	pos  int
	err  error
}

func (e *errReader) Read(p []byte) (int, error) {
	if e.pos >= len(e.data) {
		return 0, e.err
	}
	n := copy(p, e.data[e.pos:])
	e.pos += n
	return n, nil
}

func TestRenderFlushesBeforeReturningReadError(t *testing.T) {
	boom := errors.New("boom")
	var buf bytes.Buffer

	// readOnly so the byte loop goes through bufio and surfaces the read error.
	err := Render(&buf, readOnly{&errReader{data: []byte("done\n"), err: boom}})

	assert.Check(t, cmp.ErrorIs(err, boom))
	// Content read before the error is still flushed.
	assert.Check(t, cmp.Equal(buf.String(), "done\n"))
}
