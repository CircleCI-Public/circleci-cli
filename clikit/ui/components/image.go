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

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	// Decoders for the formats a CI job is likely to publish. Registering them is
	// what lets image.Decode recognise the bytes; nothing calls into them directly.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/charmbracelet/x/mosaic"

	// The rest of the formats, from the module mosaic already pulls in — so webp,
	// bmp and tiff support costs no extra dependency. Blank imports for the same
	// reason as the stdlib decoders above.
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// maxImagePixels caps how large an image may be to render. Decoding allocates
// four bytes per pixel and mosaic then scales that buffer, so a header claiming
// enormous dimensions is a cheap way to exhaust memory — and the dimensions are
// read from a file the CLI did not produce. 25 megapixels is comfortably above
// any screenshot (a 4K frame is 8.3) while keeping the decode under ~100 MB.
const maxImagePixels = 25_000_000

// ImageInfo reports whether data is an image this package can render, along with
// its format name ("png", "jpeg", "gif", "webp", "bmp", "tiff") and pixel
// dimensions. It reads only the header, so it is cheap enough to use as a
// content sniff before deciding how to display a file.
func ImageInfo(data []byte) (format string, width, height int, ok bool) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, false
	}
	return format, cfg.Width, cfg.Height, true
}

// RenderImage draws data as a block-mosaic image — Unicode block glyphs colored
// per half-cell — fitted into cols columns and, when rows is positive, rows
// rows, preserving the image's aspect ratio. Pass rows as zero to fit the width
// only and let the caller scroll.
//
// The returned string is one line per cell row, carrying ANSI color, so it can be
// handed to a pager or written straight to a terminal. It has no trailing
// newline: mosaic ends its last row with one, which a viewport counts as a
// further (empty) line — enough to make an image that exactly fits its window
// scroll by a row.
func RenderImage(data []byte, cols, rows int) (string, error) {
	format, srcWidth, srcHeight, ok := ImageInfo(data)
	if !ok {
		return "", fmt.Errorf("not a supported image format")
	}
	if srcWidth <= 0 || srcHeight <= 0 {
		return "", fmt.Errorf("%s image has no pixels", format)
	}
	if px := srcWidth * srcHeight; px > maxImagePixels {
		return "", fmt.Errorf("%s image is %d megapixels, too large to render", format, px/1_000_000)
	}
	if cols < 1 {
		return "", fmt.Errorf("no room to render a %s image", format)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decoding %s image: %w", format, err)
	}

	w, h := fitImageCells(srcWidth, srcHeight, cols, rows)
	// mosaic sizes in pixels and reads a 2×2 pixel block per cell, so the pixel
	// dimensions are twice the cell dimensions. It stretches to whatever it is
	// given rather than preserving the aspect ratio, which is why fitImageCells
	// works that out first.
	m := mosaic.New().Width(w * 2).Height(h * 2)
	return strings.TrimRight(m.Render(img), "\n"), nil
}

// fitImageCells returns the cell dimensions an image of srcWidth×srcHeight pixels
// should render at to fill cols columns without distortion, capped to rows rows
// when rows is positive (fitting the height instead, and narrowing to match).
//
// A cell holds a 2×2 pixel block but is itself about twice as tall as it is wide,
// so an aspect-correct image is half as many rows as the pixel ratio implies.
func fitImageCells(srcWidth, srcHeight, cols, rows int) (w, h int) {
	const cellAspect = 2

	w = cols
	h = w * srcHeight / (srcWidth * cellAspect)
	if h < 1 {
		h = 1
	}
	if rows > 0 && h > rows {
		h = rows
		w = h * srcWidth * cellAspect / srcHeight
		if w < 1 {
			w = 1
		}
		if w > cols {
			w = cols
		}
	}
	return w, h
}
