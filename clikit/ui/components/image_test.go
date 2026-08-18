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

package components_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
)

// testPNG encodes a w×h image with a diagonal gradient, so the rendering has
// something to vary over rather than one flat color.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / max(w-1, 1)), G: uint8(y * 255 / max(h-1, 1)), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	assert.NilError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// renderedSize is the size of a mosaic rendering in terminal cells: the number of
// lines, and the width of the widest one with ANSI stripped. It deliberately does
// not trim: a trailing newline is a real extra row to a viewport, so it has to
// show up here.
func renderedSize(t *testing.T, out string) (cols, rows int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > cols {
			cols = w
		}
	}
	return cols, len(lines)
}

func TestImageInfo(t *testing.T) {
	t.Run("a png header reports its format and dimensions", func(t *testing.T) {
		format, w, h, ok := components.ImageInfo(testPNG(t, 40, 20))
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(format, "png"))
		assert.Check(t, cmp.Equal(w, 40))
		assert.Check(t, cmp.Equal(h, 20))
	})

	t.Run("jpeg is recognised too", func(t *testing.T) {
		var jpg bytes.Buffer
		assert.NilError(t, jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil))
		format, _, _, ok := components.ImageInfo(jpg.Bytes())
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(format, "jpeg"))
	})

	t.Run("text, a truncated header and nothing at all are not images", func(t *testing.T) {
		_, _, _, ok := components.ImageInfo([]byte("mode: set\ntotal: 91.2%\n"))
		assert.Check(t, !ok, "text should not sniff as an image")

		_, _, _, ok = components.ImageInfo(testPNG(t, 4, 4)[:12])
		assert.Check(t, !ok, "a truncated png header should not sniff as an image")

		_, _, _, ok = components.ImageInfo(nil)
		assert.Check(t, !ok, "no bytes should not sniff as an image")
	})
}

func TestRenderImage_FitsWidthPreservingAspect(t *testing.T) {
	// A 2:1 image at 40 columns: one cell covers a 2×2 pixel block but reads twice
	// as tall as wide, so an undistorted rendering is 40 × 10.
	out, err := components.RenderImage(testPNG(t, 100, 50), 40, 0)
	assert.NilError(t, err)

	t.Run("the rendering fills the width at the image's aspect ratio", func(t *testing.T) {
		cols, rows := renderedSize(t, out)
		assert.Check(t, cmp.Equal(cols, 40))
		assert.Check(t, cmp.Equal(rows, 10))
	})

	t.Run("it is colored block glyphs, with no trailing row", func(t *testing.T) {
		assert.Check(t, strings.Contains(out, "\x1b["), "expected ANSI color in the rendering")
		// A trailing newline is a real extra row to a viewport: an image that exactly
		// fills its window must not make the pager scroll.
		assert.Check(t, !strings.HasSuffix(out, "\n"), "rendering should not end with a newline")
	})
}

func TestRenderImage_FitsWithinRows(t *testing.T) {
	t.Run("a tall image narrows to keep its shape within the rows", func(t *testing.T) {
		// At 50 columns this would want 100 rows; capped at 20 it narrows rather
		// than squashing.
		out, err := components.RenderImage(testPNG(t, 50, 200), 50, 20)
		assert.NilError(t, err)
		cols, rows := renderedSize(t, out)
		assert.Check(t, cmp.Equal(rows, 20))
		assert.Check(t, cmp.Equal(cols, 10))
	})

	t.Run("with room to spare the cap does not bite", func(t *testing.T) {
		out, err := components.RenderImage(testPNG(t, 200, 50), 40, 20)
		assert.NilError(t, err)
		cols, rows := renderedSize(t, out)
		assert.Check(t, cmp.Equal(cols, 40))
		assert.Check(t, cmp.Equal(rows, 5))
	})
}

func TestRenderImage_TinyTargets(t *testing.T) {
	t.Run("a wide image squeezed into one column still renders a row", func(t *testing.T) {
		out, err := components.RenderImage(testPNG(t, 400, 4), 1, 0)
		assert.NilError(t, err)
		_, rows := renderedSize(t, out)
		assert.Check(t, cmp.Equal(rows, 1))
	})

	t.Run("no columns at all is an error, not an empty rendering", func(t *testing.T) {
		_, err := components.RenderImage(testPNG(t, 40, 20), 0, 10)
		assert.Check(t, cmp.ErrorContains(err, "no room"))
	})
}

func TestRenderImage_Errors(t *testing.T) {
	t.Run("bytes that are not an image", func(t *testing.T) {
		_, err := components.RenderImage([]byte("not an image"), 40, 0)
		assert.Check(t, cmp.ErrorContains(err, "not a supported image"))
	})

	t.Run("a header claiming enormous dimensions is refused before decoding", func(t *testing.T) {
		// Four bytes per pixel would be allocated, from a file the CLI did not
		// produce.
		_, err := components.RenderImage(hugePNGHeader(t, 40000, 40000), 40, 0)
		assert.Check(t, cmp.ErrorContains(err, "too large to render"))
	})
}

// hugePNGHeader builds just enough of a PNG — signature plus an IHDR chunk — for
// image.DecodeConfig to report the given dimensions. Encoding a real image that
// size would need gigabytes; the point is that the guard fires on the header.
func hugePNGHeader(t *testing.T, w, h uint32) []byte {
	t.Helper()
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	assert.NilError(t, binary.Write(&ihdr, binary.BigEndian, w))
	assert.NilError(t, binary.Write(&ihdr, binary.BigEndian, h))
	ihdr.Write([]byte{8, 6, 0, 0, 0}) // 8-bit RGBA, no interlace

	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	assert.NilError(t, binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-len("IHDR"))))
	out.Write(ihdr.Bytes())
	assert.NilError(t, binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes())))
	return out.Bytes()
}
