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

// Package termrender replays captured terminal output and writes the resulting
// text, line by line, to an output writer.
//
// It exists to render the stdout/stderr a CI step produced into the text a
// human would have seen. A naive ANSI strip is not enough: tools like Docker
// redraw progress with carriage returns and cursor movement, so stripping the
// escapes alone leaves a pile of stale, half-drawn lines. Replaying the stream
// through a terminal model collapses those redraws to their final state.
//
// Two rendering modes are offered:
//
//   - Render writes plain text — SGR styling is discarded. Use it when piping to
//     a file or another program (grep/awk/less), where escape codes are noise.
//   - RenderStyled preserves SGR styling (colors, bold, underline, …): each
//     rendered cell keeps the graphic rendition active when it was drawn, with
//     redraws still collapsed. Use it for a human-facing viewer such as the
//     interactive output pager.
//
// Unlike a general terminal emulator it is built for the one job of rendering a
// (potentially enormous) append-only log:
//
//   - Lines are never wrapped. Each row holds a full logical line of unbounded
//     width, so long log lines survive intact for grep/awk and friends. A
//     fixed-width grid would either wrap (inserting fake breaks) or truncate
//     them.
//   - Lines are streamed out as they scroll off the top of a fixed-height
//     window, so memory stays flat (O(window)) and nothing is ever silently
//     dropped, however large the log.
//   - There is no input/reply channel: device-status and color queries in the
//     stream are simply ignored. The renderer never blocks waiting to answer
//     them.
//
// Cursor addressing is tracked in columns of one cell per rune; wide-rune (CJK)
// alignment inside an in-place redraw is therefore approximate, which is
// immaterial for the ASCII and box-drawing output that progress redraws use in
// practice.
package termrender

import (
	"bufio"
	"errors"
	"io"

	"github.com/charmbracelet/x/ansi"
)

// DefaultHeight is the height, in rows, of the window the renderer keeps live
// for in-place redraws. Cursor movement (e.g. a Docker pull repainting its
// per-layer progress lines) is collapsed within this window; once a line
// scrolls above it the line is final and is written out. It only needs to be
// as tall as the largest block a tool repaints at once.
const DefaultHeight = 100

// maxColumns bounds how far a cursor-positioning sequence may push the column.
// Printable content extends a line naturally and is never truncated, but a
// movement/position/repeat command with an absurd parameter — e.g.
// "\x1b[999999999C", or binary data misparsed as a CSI parameter — must not be
// able to force a multi-gigabyte blank-padded row (an OOM that looks like a
// hang). No real terminal or progress redraw addresses columns beyond this.
const maxColumns = 1 << 16

// initialRowCap is the capacity a line is first allocated with, sized for a
// typical log line so a filling row does not regrow through several append
// doublings. Longer lines still grow on demand; the live window is bounded, so
// over-allocating a short line costs little.
const initialRowCap = 80

// resetSeq is the SGR sequence that clears all styling, emitted (in styled mode)
// at the end of a styled line and before applying a new pen.
const resetSeq = "\x1b[0m"

// Render replays the captured terminal output read from src and writes the
// rendered plain text (styling discarded) to dst. Input is consumed one byte at
// a time and output is streamed as lines scroll off, so memory stays flat
// regardless of how large the source is. See the package documentation.
//
// Whatever was rendered before a read error is still flushed to dst; the read
// error is then returned.
func Render(dst io.Writer, src io.Reader) error {
	return replay(dst, src, false)
}

// RenderStyled is like Render but preserves SGR styling: each rendered cell
// keeps the graphic rendition (colors, bold, …) active when it was drawn, with
// redraws still collapsed to their final state. Use it for a human-facing
// viewer where colors matter; use Render for plain-text pipe/file output.
func RenderStyled(dst io.Writer, src io.Reader) error {
	return replay(dst, src, true)
}

func replay(dst io.Writer, src io.Reader, styled bool) error {
	r := newRenderer(dst, DefaultHeight)
	r.styled = styled

	// Avoid double-buffering when the caller already supplies a byte reader
	// (e.g. a *bytes.Reader wrapping an in-memory log).
	br, ok := src.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(src)
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return r.flush()
			}
			_ = r.flush()
			return err
		}
		r.parser.Advance(b)
	}
}

// cell is one rendered character: the rune and the graphic-rendition pen active
// when it was drawn. In plain mode the pen is always the zero (default) pen.
type cell struct {
	r   rune
	pen pen
}

// blank is the default-styled space used to pad gaps and erase cells.
var blank = cell{r: ' '}

// renderer holds the live terminal window and emits lines as they scroll off.
type renderer struct {
	w      *bufio.Writer
	parser *ansi.Parser
	height int
	styled bool

	// rows is the live window, always exactly height rows. rows[0] is the top
	// of the screen; each row is the cells of one (unwrapped) line. A nil row
	// is blank.
	rows [][]cell
	cx   int // cursor column
	cy   int // cursor row, within [0, height)

	cur      pen  // current graphic-rendition pen (tracked only in styled mode)
	lastRune rune // most recently printed rune, for REP

	// pendingBlanks counts blank lines emitted but withheld: they are only
	// written once real content follows, which drops the trailing blank rows of
	// the window (and any blank tail of the log) while preserving blank lines
	// between content.
	pendingBlanks int
}

func newRenderer(dst io.Writer, height int) *renderer {
	if height < 1 {
		height = 1
	}
	r := &renderer{
		w:      bufio.NewWriter(dst),
		height: height,
		rows:   make([][]cell, height),
	}
	p := ansi.NewParser()
	p.SetParamsSize(32)
	p.SetHandler(ansi.Handler{
		Print:     r.print,
		Execute:   r.execute,
		HandleCsi: r.csi,
		HandleEsc: r.esc,
	})
	r.parser = p
	return r
}

// print writes a printable rune at the cursor and advances one column. The hot
// path — appending at the end of the current line — is a single append with no
// padding or double-write; overwriting an existing cell (an in-place redraw) and
// filling a gap left by a forward cursor jump are the slower branches. A fresh
// line is allocated with room for a typical log line so it does not regrow
// through several doublings as it fills.
func (r *renderer) print(c rune) {
	row := r.rows[r.cy]
	nc := cell{r: c, pen: r.cur}
	switch {
	case r.cx < len(row):
		row[r.cx] = nc // overwrite a cell already on the line (redraw)
	case r.cx == len(row):
		if row == nil {
			row = make([]cell, 0, initialRowCap)
		}
		row = append(row, nc)
	default: // cursor moved past the end: pad the gap with blanks, then write.
		row = append(padTo(row, r.cx), nc)
	}
	r.rows[r.cy] = row
	r.cx++
	r.lastRune = c
}

// execute handles a C0 control byte found in the stream.
func (r *renderer) execute(b byte) {
	switch b {
	case '\r': // CR
		r.cx = 0
	case '\n': // LF — newline mode: carriage return + line feed.
		r.cx = 0
		r.index()
	case '\v', '\f': // VT, FF — line feed without carriage return.
		r.index()
	case '\b': // BS
		if r.cx > 0 {
			r.cx--
		}
	case '\t': // HT — advance to the next 8-column tab stop.
		r.cx += 8 - (r.cx % 8)
	}
	// All other control bytes (BEL, etc.) are ignored.
}

// esc handles an ESC (non-CSI) sequence.
func (r *renderer) esc(cmd ansi.Cmd) {
	switch cmd.Final() {
	case 'D': // IND — index (down, scrolling at the bottom).
		r.index()
	case 'E': // NEL — next line (CR + index).
		r.cx = 0
		r.index()
	case 'M': // RI — reverse index (up, scrolling down at the top).
		r.reverseIndex()
	case 'c': // RIS — full reset. Treat as a screen clear.
		r.clearScreen()
		r.cx, r.cy = 0, 0
		r.cur = pen{}
	}
}

// csi handles a CSI sequence. Private sequences (with a prefix such as '?',
// e.g. mode sets) are ignored.
func (r *renderer) csi(cmd ansi.Cmd, params ansi.Params) {
	if cmd.Prefix() != 0 {
		return
	}
	switch cmd.Final() {
	case 'A': // CUU — cursor up
		r.cy = max(r.cy-param(params, 0, 1), 0)
	case 'B', 'e': // CUD / VPR — cursor down
		r.cy = min(r.cy+param(params, 0, 1), r.height-1)
	case 'C', 'a': // CUF / HPR — cursor forward
		r.cx = r.clampCol(r.cx + param(params, 0, 1))
	case 'D': // CUB — cursor back
		r.cx = max(r.cx-param(params, 0, 1), 0)
	case 'E': // CNL — cursor next line
		r.cy = min(r.cy+param(params, 0, 1), r.height-1)
		r.cx = 0
	case 'F': // CPL — cursor previous line
		r.cy = max(r.cy-param(params, 0, 1), 0)
		r.cx = 0
	case 'G', '`': // CHA / HPA — cursor horizontal absolute (1-based)
		r.cx = r.clampCol(param(params, 0, 1) - 1)
	case 'd': // VPA — line position absolute (1-based)
		r.cy = min(max(param(params, 0, 1)-1, 0), r.height-1)
	case 'H', 'f': // CUP / HVP — cursor position (row;col, 1-based)
		r.cy = min(max(param(params, 0, 1)-1, 0), r.height-1)
		r.cx = r.clampCol(param(params, 1, 1) - 1)
	case 'J': // ED — erase in display
		r.eraseDisplay(param(params, 0, 0))
	case 'K': // EL — erase in line
		r.eraseLine(param(params, 0, 0))
	case 'X': // ECH — erase n characters from the cursor
		r.eraseChars(param(params, 0, 1))
	case 'S': // SU — scroll up
		r.scrollUp(param(params, 0, 1))
	case 'T': // SD — scroll down
		r.scrollDown(param(params, 0, 1))
	case 'b': // REP — repeat the last printed rune
		if r.lastRune != 0 {
			for n := min(param(params, 0, 1), maxColumns); n > 0; n-- {
				r.print(r.lastRune)
			}
		}
	case 'm': // SGR — select graphic rendition
		if r.styled {
			r.sgr(params)
		}
	}
}

// clampCol bounds a target cursor column to [0, max(maxColumns, current line
// length)]. The current line's length is allowed so the cursor can always reach
// the end of genuinely long printed content, while an absurd absolute column
// from a malformed sequence is capped before it can balloon a row in padTo.
func (r *renderer) clampCol(x int) int {
	if x < 0 {
		return 0
	}
	hi := maxColumns
	if l := len(r.rows[r.cy]); l > hi {
		hi = l
	}
	return min(x, hi)
}

// index moves the cursor down one row, scrolling the window (and emitting the
// line that scrolls off) when already at the bottom.
func (r *renderer) index() {
	if r.cy >= r.height-1 {
		r.scrollUp(1)
		return
	}
	r.cy++
}

// reverseIndex moves the cursor up one row, scrolling the window down when
// already at the top. The line pushed off the bottom is discarded (it was
// blank or below the visible region a tool is repainting).
func (r *renderer) reverseIndex() {
	if r.cy <= 0 {
		r.scrollDown(1)
		return
	}
	r.cy--
}

// scrollUp removes n rows from the top of the window, emitting each, and adds n
// blank rows at the bottom. The cursor stays on the bottom row.
func (r *renderer) scrollUp(n int) {
	n = min(max(n, 0), r.height)
	for i := 0; i < n; i++ {
		r.emit(r.rows[i])
	}
	copy(r.rows, r.rows[n:])
	for i := r.height - n; i < r.height; i++ {
		r.rows[i] = nil
	}
	r.cy = r.height - 1
}

// scrollDown inserts n blank rows at the top of the window, dropping n rows off
// the bottom. Used by reverse index / SD; the dropped rows are discarded.
func (r *renderer) scrollDown(n int) {
	n = min(max(n, 0), r.height)
	copy(r.rows[n:], r.rows)
	for i := 0; i < n; i++ {
		r.rows[i] = nil
	}
	r.cy = 0
}

// eraseDisplay implements ED. Note that erasing the whole screen (mode 2)
// discards the visible content rather than emitting it: this is what collapses
// a full-screen TUI that repaints each frame to just its final frame.
func (r *renderer) eraseDisplay(mode int) {
	switch mode {
	case 0: // cursor to end of screen
		if row := r.rows[r.cy]; len(row) > r.cx {
			r.rows[r.cy] = row[:r.cx]
		}
		for y := r.cy + 1; y < r.height; y++ {
			r.rows[y] = nil
		}
	case 1: // start of screen to cursor
		for y := 0; y < r.cy; y++ {
			r.rows[y] = nil
		}
		r.eraseLine(1)
	case 2, 3: // whole screen (3 also clears scrollback, which we cannot un-emit)
		r.clearScreen()
	}
}

func (r *renderer) clearScreen() {
	for y := range r.rows {
		r.rows[y] = nil
	}
}

// eraseLine implements EL on the cursor row.
func (r *renderer) eraseLine(mode int) {
	row := r.rows[r.cy]
	switch mode {
	case 0: // cursor to end of line
		if len(row) > r.cx {
			r.rows[r.cy] = row[:r.cx]
		}
	case 1: // start of line to cursor (inclusive)
		for i := 0; i <= r.cx && i < len(row); i++ {
			row[i] = blank
		}
	case 2: // entire line
		r.rows[r.cy] = nil
	}
}

// eraseChars implements ECH: blank n cells from the cursor without moving it.
func (r *renderer) eraseChars(n int) {
	row := r.rows[r.cy]
	for i := r.cx; i < r.cx+n && i < len(row); i++ {
		row[i] = blank
	}
}

// emit writes a single finalized row, trimming trailing blanks and buffering
// wholly-blank lines so trailing blank rows are dropped. In styled mode the
// active pen is applied per run of same-styled cells and reset at line end so
// styling never bleeds past the row.
func (r *renderer) emit(row []cell) {
	// Trim trailing default-styled spaces (plain padding). A styled trailing
	// space — e.g. a colored progress bar — carries a non-default pen and is
	// kept, matching what the terminal would have shown.
	end := len(row)
	for end > 0 && row[end-1] == blank {
		end--
	}
	if end == 0 {
		r.pendingBlanks++
		return
	}
	for ; r.pendingBlanks > 0; r.pendingBlanks-- {
		_ = r.w.WriteByte('\n')
	}

	if r.styled {
		var active pen
		for i := 0; i < end; i++ {
			c := row[i]
			if c.pen != active {
				c.pen.writeTo(r.w)
				active = c.pen
			}
			_, _ = r.w.WriteRune(c.r)
		}
		if active != (pen{}) {
			_, _ = r.w.WriteString(resetSeq)
		}
	} else {
		for i := 0; i < end; i++ {
			_, _ = r.w.WriteRune(row[i].r)
		}
	}
	_ = r.w.WriteByte('\n')
}

// flush emits the rows still in the window and flushes the buffer.
func (r *renderer) flush() error {
	for _, row := range r.rows {
		r.emit(row)
	}
	return r.w.Flush()
}

// param returns CSI parameter i, substituting def when absent or zero. Cursor
// and scroll counts treat a zero parameter as the default of 1, matching the
// usual terminal behavior; callers pass def accordingly.
func param(p ansi.Params, i, def int) int {
	v, _, ok := p.Param(i, def)
	if !ok || v == 0 {
		return def
	}
	return v
}

func padTo(row []cell, n int) []cell {
	for len(row) < n {
		row = append(row, blank)
	}
	return row
}
