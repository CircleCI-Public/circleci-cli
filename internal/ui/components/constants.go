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

package components

// Key constants map bubbletea key-press string representations to named values.
const (
	KeyCtrlC     = "ctrl+c"
	KeyEnter     = "enter"
	KeyEsc       = "esc"
	KeyTab       = "tab"
	KeyShiftTab  = "shift+tab"
	KeyBackspace = "backspace"
	KeyHome      = "home"
	KeyEnd       = "end"
	KeyPgUp      = "pgup"
	KeyPgDown    = "pgdown"
	KeyUp        = "up"
	KeyDown      = "down"
	KeySlash     = "/"
	KeyQuestion  = "?"

	// Letter keys. The value is the literal bubbletea reports; uppercase
	// variants are the shifted key (e.g. "N" for shift+n).
	KeyQ      = "q"
	KeyR      = "r"
	KeyS      = "s"
	KeyShiftS = "S"
	KeyN      = "n"
	KeyShiftN = "N"
	KeyG      = "g"
	KeyShiftG = "G"
)
