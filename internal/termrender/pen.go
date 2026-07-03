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
	"bufio"

	"github.com/charmbracelet/x/ansi"
)

// penAttr is a bitmask of boolean SGR attributes (bold, italic, …).
type penAttr uint16

const (
	attrBold penAttr = 1 << iota
	attrFaint
	attrItalic
	attrUnderline
	attrBlink
	attrReverse
	attrConceal
	attrStrike
)

// attrCodes maps each boolean attribute to the SGR parameter that turns it on,
// in the order they are serialized.
var attrCodes = []struct {
	bit  penAttr
	code string
}{
	{attrBold, "1"},
	{attrFaint, "2"},
	{attrItalic, "3"},
	{attrUnderline, "4"},
	{attrBlink, "5"},
	{attrReverse, "7"},
	{attrConceal, "8"},
	{attrStrike, "9"},
}

// attrOn and attrOff map SGR parameters to the attributes they set and clear.
// Absent parameters return the zero mask (a no-op). Kept as tables so sgr stays
// a flat dispatch rather than a long switch.
var (
	attrOn = map[int]penAttr{
		1: attrBold, 2: attrFaint, 3: attrItalic, 4: attrUnderline,
		5: attrBlink, 6: attrBlink, 7: attrReverse, 8: attrConceal, 9: attrStrike,
	}
	attrOff = map[int]penAttr{
		22: attrBold | attrFaint, 23: attrItalic, 24: attrUnderline,
		25: attrBlink, 27: attrReverse, 28: attrConceal, 29: attrStrike,
	}
)

// color encodes an SGR color as a fixed-size integer so a pen is cheap to
// compare and never carries a heap-allocated string. The top nibble is the kind
// and the low 28 bits the payload:
//
//	default    the zero value (no color)
//	basic      an ANSI 16-color index 0..15 (0..7 normal, 8..15 bright)
//	palette    a 256-color palette index 0..255
//	rgb        a truecolor value packed as r<<16 | g<<8 | b
//
// The fg/bg role (which decides 30- vs 40-range, 38 vs 48) is not stored — it is
// known from the field the color lives in, and applied by writeSGR.
type color uint32

const (
	colorKindShift       = 28
	colorPayload   color = (1 << colorKindShift) - 1
	kindMask       color = 3 << colorKindShift

	colorDefault color = 0
	kindBasic    color = 1 << colorKindShift
	kindPalette  color = 2 << colorKindShift
	kindRGB      color = 3 << colorKindShift
)

// The payloads are masked (idx to 4 bits, palette to 8, each RGB channel to 8)
// so a malformed SGR parameter cannot bleed into the kind bits, which also keeps
// the int→uint32 conversion provably in range.
func basicColor(idx int) color { return kindBasic | color(idx&0xf) }
func paletteColor(n int) color { return kindPalette | color(n&0xff) }
func rgbColor(r, g, b int) color {
	//#nosec:G115 -- each channel is masked to 8 bits, so the value fits in 24 bits
	return kindRGB | color((r&0xff)<<16|(g&0xff)<<8|b&0xff)
}

// writeSGR writes this color's SGR parameters (no leading ';') to w. fg selects
// the foreground (30/90/38) vs background (40/100/48) forms. The default color
// (and any other kind) writes nothing — callers skip it. The color constants are
// bit masks rather than a closed enum, so the switch is not exhaustive over them.
//
//nolint:exhaustive // color constants are bit masks, not a closed enum
func (c color) writeSGR(w *bufio.Writer, fg bool) {
	switch c & kindMask {
	case kindBasic:
		idx := int(c & colorPayload)
		switch {
		case idx < 8 && fg:
			writeInt(w, 30+idx)
		case idx < 8:
			writeInt(w, 40+idx)
		case fg:
			writeInt(w, 90+idx-8)
		default:
			writeInt(w, 100+idx-8)
		}
	case kindPalette:
		writeColorPrefix(w, fg, "5;")
		writeInt(w, int(c&colorPayload))
	case kindRGB:
		p := c & colorPayload
		writeColorPrefix(w, fg, "2;")
		writeInt(w, int(p>>16&0xff))
		_ = w.WriteByte(';')
		writeInt(w, int(p>>8&0xff))
		_ = w.WriteByte(';')
		writeInt(w, int(p&0xff))
	}
}

// writeColorPrefix writes the "38;" or "48;" selector plus the mode ("5;" or
// "2;") for an extended (palette/truecolor) color.
func writeColorPrefix(w *bufio.Writer, fg bool, mode string) {
	if fg {
		_, _ = w.WriteString("38;")
	} else {
		_, _ = w.WriteString("48;")
	}
	_, _ = w.WriteString(mode)
}

// writeInt writes n (assumed non-negative — all SGR parameters are) in base 10
// to w one digit at a time. Recursing to emit the most-significant digit first
// avoids any intermediate buffer, so it never allocates; the depth is the digit
// count (at most a handful for the 0..255 range colors use).
func writeInt(w *bufio.Writer, n int) {
	if n >= 10 {
		writeInt(w, n/10)
	}
	_ = w.WriteByte(byte('0' + n%10))
}

// pen is the graphic-rendition (SGR) state active at a cell: foreground and
// background color plus the boolean attributes. The zero pen is fully default
// (no styling). All fields are fixed-size integers, so pen is comparable with a
// cheap == (runs of identically-styled cells coalesce when emitting) and carries
// no heap-allocated strings.
type pen struct {
	fg, bg color
	attrs  penAttr
}

// writeTo writes the SGR sequence that establishes this pen from an unknown
// prior state directly to w. It always begins with a reset (parameter 0) so it
// is a clean, self-contained transition — attributes from a previous pen never
// linger. The zero pen yields a bare reset. Writing straight to the buffered
// writer avoids the per-transition allocation a string return would cost on the
// styled emit hot path.
func (p pen) writeTo(w *bufio.Writer) {
	if p == (pen{}) {
		_, _ = w.WriteString(resetSeq)
		return
	}
	_, _ = w.WriteString("\x1b[0") // reset, then append the set attributes
	for _, a := range attrCodes {
		if p.attrs&a.bit != 0 {
			_ = w.WriteByte(';')
			_, _ = w.WriteString(a.code)
		}
	}
	if p.fg != colorDefault {
		_ = w.WriteByte(';')
		p.fg.writeSGR(w, true)
	}
	if p.bg != colorDefault {
		_ = w.WriteByte(';')
		p.bg.writeSGR(w, false)
	}
	_ = w.WriteByte('m')
}

// sgr applies an SGR (select graphic rendition) sequence to the current pen.
// Parameters are processed left to right so later values override earlier ones
// (as a terminal does); parameter 0, or an empty sequence, resets to default.
func (r *renderer) sgr(params ansi.Params) {
	if len(params) == 0 {
		r.cur = pen{}
		return
	}
	for i := 0; i < len(params); i++ {
		n, _, _ := params.Param(i, 0)
		switch {
		case n == 0:
			r.cur = pen{}
		case attrOn[n] != 0:
			r.cur.attrs |= attrOn[n]
		case attrOff[n] != 0:
			r.cur.attrs &^= attrOff[n]
		case n >= 30 && n <= 37:
			r.cur.fg = basicColor(n - 30)
		case n >= 90 && n <= 97:
			r.cur.fg = basicColor(8 + n - 90)
		case n == 39:
			r.cur.fg = colorDefault
		case n >= 40 && n <= 47:
			r.cur.bg = basicColor(n - 40)
		case n >= 100 && n <= 107:
			r.cur.bg = basicColor(8 + n - 100)
		case n == 49:
			r.cur.bg = colorDefault
		case n == 38:
			r.cur.fg, i = extColor(params, i)
		case n == 48:
			r.cur.bg, i = extColor(params, i)
		}
	}
}

// extColor reads an extended-color selector beginning at index i (a 38 or 48
// parameter): "…;5;n" (256-color) or "…;2;r;g;b" (truecolor). It returns the
// encoded color and the index of the last parameter it consumed, so the caller's
// loop resumes after it. A malformed/truncated selector yields the default color
// and consumes what it could.
func extColor(params ansi.Params, i int) (color, int) {
	mode, _, ok := params.Param(i+1, 0)
	if !ok {
		return colorDefault, i
	}
	switch mode {
	case 5:
		n, _, ok := params.Param(i+2, 0)
		if !ok {
			return colorDefault, i + 1
		}
		return paletteColor(n), i + 2
	case 2:
		red, _, _ := params.Param(i+2, 0)
		green, _, _ := params.Param(i+3, 0)
		blue, _, ok := params.Param(i+4, 0)
		if !ok {
			return colorDefault, i + 3
		}
		return rgbColor(red, green, blue), i + 4
	}
	return colorDefault, i + 1
}
