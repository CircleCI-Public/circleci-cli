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
	"strconv"
	"strings"

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

// pen is the graphic-rendition (SGR) state active at a cell: foreground and
// background color fragments plus the boolean attributes. fg/bg hold the SGR
// parameters for the color ("31", "90", "38;5;9", "38;2;1;2;3"); "" is the
// terminal default. The zero pen is fully default (no styling). pen is
// comparable so runs of identically-styled cells coalesce when emitting.
type pen struct {
	fg, bg string
	attrs  penAttr
}

// sequence returns the SGR sequence that establishes this pen from an unknown
// prior state. It always begins with a reset (parameter 0) so it is a clean,
// self-contained transition — attributes from a previous pen never linger. The
// zero pen yields a bare reset.
func (p pen) sequence() string {
	if p == (pen{}) {
		return resetSeq
	}
	params := make([]string, 0, len(attrCodes)+3)
	params = append(params, "0")
	for _, a := range attrCodes {
		if p.attrs&a.bit != 0 {
			params = append(params, a.code)
		}
	}
	if p.fg != "" {
		params = append(params, p.fg)
	}
	if p.bg != "" {
		params = append(params, p.bg)
	}
	return "\x1b[" + strings.Join(params, ";") + "m"
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
		case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
			r.cur.fg = strconv.Itoa(n)
		case n == 39:
			r.cur.fg = ""
		case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
			r.cur.bg = strconv.Itoa(n)
		case n == 49:
			r.cur.bg = ""
		case n == 38:
			r.cur.fg, i = extColor(params, i)
		case n == 48:
			r.cur.bg, i = extColor(params, i)
		}
	}
}

// extColor reads an extended-color selector beginning at index i (a 38 or 48
// parameter): "…;5;n" (256-color) or "…;2;r;g;b" (truecolor). It returns the
// full SGR color fragment (e.g. "38;5;9") and the index of the last parameter it
// consumed, so the caller's loop resumes after it. A malformed/truncated
// selector yields an empty fragment and consumes what it could.
func extColor(params ansi.Params, i int) (string, int) {
	base, _, _ := params.Param(i, 0)
	mode, _, ok := params.Param(i+1, 0)
	if !ok {
		return "", i
	}
	switch mode {
	case 5:
		n, _, ok := params.Param(i+2, 0)
		if !ok {
			return "", i + 1
		}
		return strconv.Itoa(base) + ";5;" + strconv.Itoa(n), i + 2
	case 2:
		red, _, _ := params.Param(i+2, 0)
		green, _, _ := params.Param(i+3, 0)
		blue, _, ok := params.Param(i+4, 0)
		if !ok {
			return "", i + 3
		}
		return strconv.Itoa(base) + ";2;" + strconv.Itoa(red) + ";" +
			strconv.Itoa(green) + ";" + strconv.Itoa(blue), i + 4
	}
	return "", i + 1
}
